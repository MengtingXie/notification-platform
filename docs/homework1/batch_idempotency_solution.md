# 批量接口幂等问题解决方案

## 问题分析

### 当前问题
当前批量接口在处理幂等冲突时采用了简单粗暴的方式：只要批次中有任何一个通知存在幂等冲突，整个批次就直接返回失败。这种处理方式存在以下问题：

1. **部分幂等冲突导致整批失败**：批次中可能只有少数通知存在幂等冲突，但整批都被拒绝
2. **业务方重试困难**：业务方无法区分哪些通知成功、哪些失败、哪些是幂等冲突
3. **资源浪费**：已经成功处理的通知在重试时会被重复处理
4. **用户体验差**：无法提供精确的处理结果反馈

### 核心挑战
1. **批次中部分幂等冲突**：需要区分处理成功、失败和幂等冲突的通知
2. **业务方重试场景**：同一批次的重试请求需要返回一致的结果
3. **部分成功场景**：前一次请求部分发送成功，部分发送失败时的重试处理

## 解决方案设计

### 1. 整体架构调整

#### 1.1 批量处理策略调整
将当前的"全成功或全失败"模式调整为"部分成功"模式：

```go
type BatchProcessResult struct {
    SuccessResults    []SendResponse     // 成功处理的通知
    IdempotentResults []SendResponse     // 幂等冲突的通知（返回已存在记录的状态）
    FailedResults     []SendResponse     // 处理失败的通知
}
```

#### 1.2 幂等检测增强
在批量处理前进行幂等预检测，将通知分类：

```go
type NotificationClassification struct {
    NewNotifications        []domain.Notification  // 新通知
    IdempotentNotifications []domain.Notification  // 幂等冲突通知
}
```

### 2. 详细实现方案

#### 2.1 批量幂等预检测服务

```go
type BatchIdempotencyService struct {
    idempotentSvc idempotent.IdempotencyService
    repo          repository.NotificationRepository
}

func (s *BatchIdempotencyService) ClassifyNotifications(
    ctx context.Context, 
    notifications []domain.Notification,
) (*NotificationClassification, error) {
    // 1. 构建幂等键
    keys := make([]string, len(notifications))
    for i, n := range notifications {
        keys[i] = fmt.Sprintf("%d-%s", n.BizID, n.Key)
    }
    
    // 2. 批量检测幂等性
    existsResults, err := s.idempotentSvc.MExists(ctx, keys...)
    if err != nil {
        return nil, err
    }
    
    // 3. 分类通知
    classification := &NotificationClassification{
        NewNotifications:        make([]domain.Notification, 0),
        IdempotentNotifications: make([]domain.Notification, 0),
    }
    
    for i, exists := range existsResults {
        if exists {
            classification.IdempotentNotifications = append(
                classification.IdempotentNotifications, 
                notifications[i],
            )
        } else {
            classification.NewNotifications = append(
                classification.NewNotifications, 
                notifications[i],
            )
        }
    }
    
    return classification, nil
}
```

#### 2.2 幂等冲突通知处理

```go
func (s *BatchIdempotencyService) HandleIdempotentNotifications(
    ctx context.Context, 
    notifications []domain.Notification,
) ([]domain.SendResponse, error) {
    responses := make([]domain.SendResponse, 0, len(notifications))
    
    for _, notification := range notifications {
        // 查询已存在的通知记录
        existing, err := s.repo.GetByKey(ctx, notification.BizID, notification.Key)
        if err != nil {
            // 如果查询失败，返回错误响应
            responses = append(responses, domain.SendResponse{
                NotificationID: notification.ID,
                Status:         domain.SendStatusFailed,
                Error:          err,
            })
            continue
        }
        
        // 返回已存在记录的状态
        responses = append(responses, domain.SendResponse{
            NotificationID: existing.ID,
            Status:         existing.Status,
            IsIdempotent:   true, // 标记为幂等响应
        })
    }
    
    return responses, nil
}
```

#### 2.3 批量发送服务重构

```go
func (s *sendService) BatchSendNotifications(
    ctx context.Context, 
    notifications ...domain.Notification,
) (domain.BatchSendResponse, error) {
    response := domain.BatchSendResponse{
        Results: make([]domain.SendResponse, 0, len(notifications)),
    }
    
    // 1. 参数校验和ID生成
    for i := range notifications {
        if err := notifications[i].Validate(); err != nil {
            return domain.BatchSendResponse{}, fmt.Errorf("参数非法 %w", err)
        }
        id := s.idGenerator.GenerateID(notifications[i].BizID, notifications[i].Key)
        notifications[i].ID = uint64(id)
    }
    
    // 2. 幂等性分类
    classification, err := s.idempotencyService.ClassifyNotifications(ctx, notifications)
    if err != nil {
        return response, fmt.Errorf("幂等性检测失败: %w", err)
    }
    
    // 3. 处理幂等冲突的通知
    if len(classification.IdempotentNotifications) > 0 {
        idempotentResults, err := s.idempotencyService.HandleIdempotentNotifications(
            ctx, 
            classification.IdempotentNotifications,
        )
        if err != nil {
            return response, fmt.Errorf("处理幂等通知失败: %w", err)
        }
        response.Results = append(response.Results, idempotentResults...)
    }
    
    // 4. 处理新通知
    if len(classification.NewNotifications) > 0 {
        newResults, err := s.sendStrategy.BatchSend(ctx, classification.NewNotifications)
        if err != nil {
            // 对于新通知的处理失败，需要回滚幂等标记
            s.rollbackIdempotencyMarks(ctx, classification.NewNotifications)
            return response, fmt.Errorf("发送新通知失败: %w", err)
        }
        response.Results = append(response.Results, newResults...)
    }
    
    // 5. 按原始顺序重新排列结果
    response.Results = s.reorderResults(notifications, response.Results)
    
    return response, nil
}
```

### 3. 边缘场景处理

#### 3.1 并发请求处理
**场景**：同一个通知的多个请求几乎同时到达

**解决方案**：
- 使用Redis的SETNX操作确保原子性
- 对于并发冲突，后到达的请求查询已存在记录并返回其状态
- 增加重试机制处理短暂的竞态条件

#### 3.2 部分成功重试
**场景**：批次中部分通知发送成功，部分失败，业务方重试整个批次

**解决方案**：
```go
type RetryBatchResult struct {
    PreviouslySucceeded []domain.SendResponse  // 之前已成功的
    NewlyProcessed      []domain.SendResponse  // 本次新处理的
    StillFailed         []domain.SendResponse  // 仍然失败的
}
```

#### 3.3 状态不一致处理
**场景**：幂等检测通过但数据库插入时发生唯一键冲突

**解决方案**：
- 在数据库层面捕获唯一键冲突
- 查询已存在记录并返回其状态
- 清理已设置的幂等标记

#### 3.4 长时间重试场景
**场景**：业务方在很长时间后重试相同的批次

**解决方案**：
- 设置合理的幂等键过期时间（如24小时）
- 过期后的重试按新请求处理
- 提供查询接口让业务方确认历史状态

### 4. 响应结构调整

#### 4.1 增强的响应结构
```go
type SendResponse struct {
    NotificationID uint64     `json:"notificationId"`
    Status         SendStatus `json:"status"`
    IsIdempotent   bool       `json:"isIdempotent"`   // 是否为幂等响应
    Error          error      `json:"error,omitempty"`
    ProcessedAt    time.Time  `json:"processedAt"`    // 处理时间
}

type BatchSendResponse struct {
    Results         []SendResponse `json:"results"`
    TotalCount      int           `json:"totalCount"`
    SuccessCount    int           `json:"successCount"`
    IdempotentCount int           `json:"idempotentCount"` // 幂等冲突数量
    FailedCount     int           `json:"failedCount"`
}
```

#### 4.2 gRPC响应调整
在proto文件中增加幂等标识：
```protobuf
message SendNotificationResponse {
  uint64 notification_id = 1;
  SendStatus status = 2;
  ErrorCode error_code = 3;
  string error_message = 4;
  bool is_idempotent = 5;  // 新增：是否为幂等响应
  int64 processed_at = 6;  // 新增：处理时间戳
}
```

### 5. 实现步骤

#### 阶段1：基础设施准备
1. 扩展SendResponse结构，增加幂等标识
2. 创建BatchIdempotencyService
3. 调整数据库查询方法支持批量幂等检测

#### 阶段2：核心逻辑实现
1. 实现通知分类逻辑
2. 重构批量发送服务
3. 增加幂等冲突处理逻辑

#### 阶段3：边缘场景处理
1. 实现并发冲突处理
2. 增加状态不一致恢复机制
3. 完善错误处理和回滚逻辑

#### 阶段4：测试和优化
1. 单元测试覆盖所有场景
2. 集成测试验证端到端流程
3. 性能测试确保批量处理效率

### 6. 监控和告警

#### 6.1 关键指标
- 批量请求中幂等冲突比例
- 部分成功批次比例
- 重试请求处理时间
- 并发冲突频率

#### 6.2 告警规则
- 幂等冲突率超过阈值（如30%）
- 批量处理失败率异常
- 数据库唯一键冲突频率异常

### 7. 向后兼容性

为确保平滑升级，新方案需要：
1. 保持现有API接口不变
2. 响应结构向后兼容
3. 提供配置开关控制新旧行为
4. 详细的迁移文档和测试指南

## 8. 具体代码实现示例

### 8.1 扩展的领域模型

```go
// internal/domain/send_notification.go
type SendResponse struct {
    NotificationID uint64     `json:"notificationId"`
    Status         SendStatus `json:"status"`
    IsIdempotent   bool       `json:"isIdempotent"`   // 新增：是否为幂等响应
    ProcessedAt    time.Time  `json:"processedAt"`    // 新增：处理时间
    Error          error      `json:"error,omitempty"`
}

type BatchSendResponse struct {
    Results         []SendResponse `json:"results"`
    TotalCount      int           `json:"totalCount"`
    SuccessCount    int           `json:"successCount"`
    IdempotentCount int           `json:"idempotentCount"` // 新增：幂等冲突数量
    FailedCount     int           `json:"failedCount"`
}
```

### 8.2 批量幂等服务实现

```go
// internal/service/idempotency/batch_idempotency.go
package idempotency

import (
    "context"
    "fmt"
    "time"

    "gitee.com/flycash/notification-platform/internal/domain"
    "gitee.com/flycash/notification-platform/internal/pkg/idempotent"
    "gitee.com/flycash/notification-platform/internal/repository"
)

type BatchIdempotencyService struct {
    idempotentSvc idempotent.IdempotencyService
    repo          repository.NotificationRepository
}

func NewBatchIdempotencyService(
    idempotentSvc idempotent.IdempotencyService,
    repo repository.NotificationRepository,
) *BatchIdempotencyService {
    return &BatchIdempotencyService{
        idempotentSvc: idempotentSvc,
        repo:          repo,
    }
}

type NotificationClassification struct {
    NewNotifications        []domain.Notification
    IdempotentNotifications []domain.Notification
    NotificationIndexMap    map[string]int // key -> 原始索引的映射
}

func (s *BatchIdempotencyService) ClassifyNotifications(
    ctx context.Context,
    notifications []domain.Notification,
) (*NotificationClassification, error) {
    if len(notifications) == 0 {
        return &NotificationClassification{
            NewNotifications:        make([]domain.Notification, 0),
            IdempotentNotifications: make([]domain.Notification, 0),
            NotificationIndexMap:    make(map[string]int),
        }, nil
    }

    // 1. 构建幂等键和索引映射
    keys := make([]string, len(notifications))
    indexMap := make(map[string]int)

    for i, n := range notifications {
        key := fmt.Sprintf("%d-%s", n.BizID, n.Key)
        keys[i] = key
        indexMap[key] = i
    }

    // 2. 批量检测幂等性
    existsResults, err := s.idempotentSvc.MExists(ctx, keys...)
    if err != nil {
        return nil, fmt.Errorf("批量幂等检测失败: %w", err)
    }

    // 3. 分类通知
    classification := &NotificationClassification{
        NewNotifications:        make([]domain.Notification, 0),
        IdempotentNotifications: make([]domain.Notification, 0),
        NotificationIndexMap:    indexMap,
    }

    for i, exists := range existsResults {
        if exists {
            classification.IdempotentNotifications = append(
                classification.IdempotentNotifications,
                notifications[i],
            )
        } else {
            classification.NewNotifications = append(
                classification.NewNotifications,
                notifications[i],
            )
        }
    }

    return classification, nil
}

func (s *BatchIdempotencyService) HandleIdempotentNotifications(
    ctx context.Context,
    notifications []domain.Notification,
) ([]domain.SendResponse, error) {
    if len(notifications) == 0 {
        return []domain.SendResponse{}, nil
    }

    responses := make([]domain.SendResponse, 0, len(notifications))

    for _, notification := range notifications {
        // 查询已存在的通知记录
        existing, err := s.repo.GetByKey(ctx, notification.BizID, notification.Key)
        if err != nil {
            // 如果查询失败，可能是数据不一致，返回错误响应
            responses = append(responses, domain.SendResponse{
                NotificationID: notification.ID,
                Status:         domain.SendStatusFailed,
                IsIdempotent:   true,
                ProcessedAt:    time.Now(),
                Error:          fmt.Errorf("查询已存在通知失败: %w", err),
            })
            continue
        }

        // 返回已存在记录的状态
        responses = append(responses, domain.SendResponse{
            NotificationID: existing.ID,
            Status:         existing.Status,
            IsIdempotent:   true,
            ProcessedAt:    time.Now(),
        })
    }

    return responses, nil
}

// RollbackIdempotencyMarks 回滚幂等标记（当新通知处理失败时）
func (s *BatchIdempotencyService) RollbackIdempotencyMarks(
    ctx context.Context,
    notifications []domain.Notification,
) error {
    if len(notifications) == 0 {
        return nil
    }

    keys := make([]string, len(notifications))
    for i, n := range notifications {
        keys[i] = fmt.Sprintf("%d-%s", n.BizID, n.Key)
    }

    // 这里需要扩展IdempotencyService接口，增加删除方法
    // 或者依赖Redis的过期机制自动清理
    // 暂时记录日志，依赖过期机制
    // TODO: 实现幂等键的主动清理

    return nil
}
```

### 8.3 发送策略调整

```go
// internal/service/sendstrategy/immediate.go 调整
func (s *ImmediateSendStrategy) BatchSend(ctx context.Context, notifications []domain.Notification) ([]domain.SendResponse, error) {
    if len(notifications) == 0 {
        return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
    }

    for i := range notifications {
        notifications[i].SetSendTime()
    }

    // 尝试批量创建通知记录
    createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
    if err != nil {
        // 检查是否是部分唯一索引冲突
        if errors.Is(err, errs.ErrNotificationDuplicate) {
            // 处理部分冲突的情况
            return s.handlePartialDuplicateError(ctx, notifications, err)
        }
        return nil, fmt.Errorf("创建通知失败: %w", err)
    }

    // 立即发送
    return s.sender.BatchSend(ctx, createdNotifications)
}

// 处理部分重复的情况
func (s *ImmediateSendStrategy) handlePartialDuplicateError(
    ctx context.Context,
    notifications []domain.Notification,
    originalErr error,
) ([]domain.SendResponse, error) {
    responses := make([]domain.SendResponse, 0, len(notifications))

    // 逐个处理通知，区分成功和冲突
    for _, notification := range notifications {
        notification.SetSendTime()

        created, err := s.repo.Create(ctx, notification)
        if err == nil {
            // 创建成功，立即发送
            sendResp, sendErr := s.sender.Send(ctx, created)
            if sendErr != nil {
                sendResp = domain.SendResponse{
                    NotificationID: created.ID,
                    Status:         domain.SendStatusFailed,
                    ProcessedAt:    time.Now(),
                    Error:          sendErr,
                }
            }
            responses = append(responses, sendResp)
            continue
        }

        // 检查是否是唯一索引冲突
        if !errors.Is(err, errs.ErrNotificationDuplicate) {
            // 其他错误
            responses = append(responses, domain.SendResponse{
                NotificationID: notification.ID,
                Status:         domain.SendStatusFailed,
                ProcessedAt:    time.Now(),
                Error:          err,
            })
            continue
        }

        // 唯一索引冲突，查询已存在记录
        existing, queryErr := s.repo.GetByKey(ctx, notification.BizID, notification.Key)
        if queryErr != nil {
            responses = append(responses, domain.SendResponse{
                NotificationID: notification.ID,
                Status:         domain.SendStatusFailed,
                IsIdempotent:   true,
                ProcessedAt:    time.Now(),
                Error:          fmt.Errorf("查询已存在通知失败: %w", queryErr),
            })
            continue
        }

        // 返回已存在记录的状态
        responses = append(responses, domain.SendResponse{
            NotificationID: existing.ID,
            Status:         existing.Status,
            IsIdempotent:   true,
            ProcessedAt:    time.Now(),
        })
    }

    return responses, nil
}
```

### 8.4 结果排序工具

```go
// internal/service/notification/result_sorter.go
package notification

import (
    "gitee.com/flycash/notification-platform/internal/domain"
)

// ReorderResults 按照原始请求顺序重新排列结果
func ReorderResults(
    originalNotifications []domain.Notification,
    results []domain.SendResponse,
) []domain.SendResponse {
    if len(originalNotifications) != len(results) {
        // 长度不匹配，直接返回
        return results
    }

    // 创建通知ID到原始索引的映射
    originalIndexMap := make(map[uint64]int)
    for i, n := range originalNotifications {
        originalIndexMap[n.ID] = i
    }

    // 创建结果ID到结果的映射
    resultMap := make(map[uint64]domain.SendResponse)
    for _, r := range results {
        resultMap[r.NotificationID] = r
    }

    // 按原始顺序重新排列
    orderedResults := make([]domain.SendResponse, len(originalNotifications))
    for i, n := range originalNotifications {
        if result, exists := resultMap[n.ID]; exists {
            orderedResults[i] = result
        } else {
            // 如果找不到对应结果，创建一个错误响应
            orderedResults[i] = domain.SendResponse{
                NotificationID: n.ID,
                Status:         domain.SendStatusFailed,
                ProcessedAt:    time.Now(),
                Error:          fmt.Errorf("未找到处理结果"),
            }
        }
    }

    return orderedResults
}
```

## 9. 测试用例设计

### 9.1 单元测试场景

```go
// internal/service/idempotency/batch_idempotency_test.go
func TestBatchIdempotencyService_ClassifyNotifications(t *testing.T) {
    tests := []struct {
        name           string
        notifications  []domain.Notification
        existsResults  []bool
        expectedNew    int
        expectedIdem   int
    }{
        {
            name: "全部为新通知",
            notifications: createTestNotifications(3),
            existsResults: []bool{false, false, false},
            expectedNew:   3,
            expectedIdem:  0,
        },
        {
            name: "全部为幂等冲突",
            notifications: createTestNotifications(3),
            existsResults: []bool{true, true, true},
            expectedNew:   0,
            expectedIdem:  3,
        },
        {
            name: "部分幂等冲突",
            notifications: createTestNotifications(4),
            existsResults: []bool{false, true, false, true},
            expectedNew:   2,
            expectedIdem:  2,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 测试实现...
        })
    }
}
```

### 9.2 集成测试场景

```go
func TestBatchSendNotifications_IdempotencyScenarios(t *testing.T) {
    tests := []struct {
        name                string
        setupExistingData   func(t *testing.T)
        notifications       []domain.Notification
        expectedSuccess     int
        expectedIdempotent  int
        expectedFailed      int
    }{
        {
            name: "批次中部分幂等冲突",
            setupExistingData: func(t *testing.T) {
                // 预先创建部分通知记录
            },
            notifications:      createMixedNotifications(),
            expectedSuccess:    2,
            expectedIdempotent: 2,
            expectedFailed:     0,
        },
        {
            name: "业务方完全重试",
            setupExistingData: func(t *testing.T) {
                // 预先创建所有通知记录
            },
            notifications:      createTestNotifications(3),
            expectedSuccess:    0,
            expectedIdempotent: 3,
            expectedFailed:     0,
        },
    }

    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // 集成测试实现...
        })
    }
}
```

## 10. 性能优化考虑

### 10.1 批量查询优化
- 使用Redis Pipeline减少网络往返
- 数据库批量查询避免N+1问题
- 合理设置批次大小限制

### 10.2 内存使用优化
- 流式处理大批量请求
- 及时释放不需要的对象引用
- 使用对象池减少GC压力

### 10.3 并发处理优化
- 合理使用goroutine池
- 避免过度并发导致资源竞争
- 实现优雅的背压机制

这个解决方案能够优雅地处理批量接口的各种幂等场景，提供精确的处理结果反馈，同时保持良好的性能和用户体验。通过分阶段实施和充分的测试验证，可以确保系统的稳定性和可靠性。
