# 批量接口幂等问题解决方案 - 实现示例

## 1. 现有代码的问题分析

### 当前实现的问题
查看现有的批量发送实现，主要问题在于：

1. **immediate.go 第84行**：`// 只要有一个唯一索引冲突整批失败`
2. **server.go 第276-292行**：当发送失败时，所有通知都标记为失败
3. **缺乏细粒度的错误处理**：无法区分哪些是幂等冲突，哪些是真正的失败

## 2. 集成步骤

### 步骤1：修改现有的SendService

```go
// internal/service/notification/send_notification.go
// 在现有的sendService中添加幂等服务

type sendService struct {
    idGenerator        *id.Generator
    sendStrategy       sendstrategy.SendStrategy
    idempotencyService *idempotency.BatchIdempotencyService // 新增
}

func NewSendService(
    idGenerator *id.Generator,
    sendStrategy sendstrategy.SendStrategy,
    idempotencyService *idempotency.BatchIdempotencyService, // 新增参数
) SendService {
    return &sendService{
        idGenerator:        idGenerator,
        sendStrategy:       sendStrategy,
        idempotencyService: idempotencyService,
    }
}
```

### 步骤2：重构BatchSendNotifications方法

```go
// 替换现有的BatchSendNotifications实现
func (e *sendService) BatchSendNotifications(
    ctx context.Context, 
    notifications ...domain.Notification,
) (domain.BatchSendResponse, error) {
    response := domain.BatchSendResponse{
        Results: make([]domain.SendResponse, 0, len(notifications)),
    }

    // 1. 参数校验
    if len(notifications) == 0 {
        return response, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
    }

    // 2. 校验并生成ID
    for i := range notifications {
        if err := notifications[i].Validate(); err != nil {
            return domain.BatchSendResponse{}, fmt.Errorf("参数非法 %w", err)
        }
        id := e.idGenerator.GenerateID(notifications[i].BizID, notifications[i].Key)
        notifications[i].ID = uint64(id)
    }

    // 3. 幂等性分类
    classification, err := e.idempotencyService.ClassifyNotifications(ctx, notifications)
    if err != nil {
        return response, fmt.Errorf("幂等性检测失败: %w", err)
    }

    var allResults []domain.SendResponse

    // 4. 处理幂等冲突的通知
    if len(classification.IdempotentNotifications) > 0 {
        idempotentResults, err := e.idempotencyService.HandleIdempotentNotifications(
            ctx, 
            classification.IdempotentNotifications,
        )
        if err != nil {
            return response, fmt.Errorf("处理幂等通知失败: %w", err)
        }
        allResults = append(allResults, idempotentResults...)
    }

    // 5. 处理新通知
    if len(classification.NewNotifications) > 0 {
        newResults, err := e.sendStrategy.BatchSend(ctx, classification.NewNotifications)
        if err != nil {
            // 回滚幂等标记
            rollbackErr := e.idempotencyService.RollbackIdempotencyMarks(ctx, classification.NewNotifications)
            if rollbackErr != nil {
                // 记录日志但不影响主错误
                // log.Error("回滚幂等标记失败", rollbackErr)
            }
            return response, fmt.Errorf("发送新通知失败: %w", err)
        }

        // 设置处理时间
        for i := range newResults {
            if newResults[i].ProcessedAt.IsZero() {
                newResults[i].ProcessedAt = time.Now()
            }
        }
        allResults = append(allResults, newResults...)
    }

    // 6. 按原始顺序重新排列结果
    response.Results = idempotency.ReorderResults(notifications, allResults)

    // 7. 计算统计信息
    e.calculateResponseStats(&response)

    return response, nil
}

func (e *sendService) calculateResponseStats(response *domain.BatchSendResponse) {
    response.TotalCount = len(response.Results)
    response.SuccessCount = 0
    response.IdempotentCount = 0
    response.FailedCount = 0

    for _, result := range response.Results {
        if result.Error != nil {
            response.FailedCount++
        } else if result.IsIdempotent {
            response.IdempotentCount++
        } else {
            switch result.Status {
            case domain.SendStatusSucceeded, domain.SendStatusPending:
                response.SuccessCount++
            case domain.SendStatusFailed:
                response.FailedCount++
            default:
                response.SuccessCount++
            }
        }
    }
}
```

### 步骤3：修改gRPC服务层

```go
// internal/api/grpc/server.go
// 修改BatchSendNotifications方法

func (s *NotificationServer) BatchSendNotifications(
    ctx context.Context, 
    req *notificationv1.BatchSendNotificationsRequest,
) (*notificationv1.BatchSendNotificationsResponse, error) {
    // ... 现有的验证逻辑保持不变 ...

    // 执行发送
    responses, err := s.sendSvc.BatchSendNotifications(ctx, notifications...)
    if err != nil {
        if s.isSystemError(err) {
            return nil, status.Errorf(codes.Internal, "%v", err)
        } else {
            // 系统级错误，所有通知都失败
            for i := range results {
                results[i] = &notificationv1.SendNotificationResponse{
                    ErrorCode:    s.convertToGRPCErrorCode(err),
                    ErrorMessage: err.Error(),
                    Status:       notificationv1.SendStatus_FAILED,
                }
            }
            return &notificationv1.BatchSendNotificationsResponse{
                TotalCount:   int32(len(results)),
                SuccessCount: int32(0),
                Results:      results,
            }, nil
        }
    }

    // 转换结果 - 新的逻辑
    results = make([]*notificationv1.SendNotificationResponse, len(responses.Results))
    for i, result := range responses.Results {
        results[i] = s.buildGRPCSendResponseWithIdempotent(result)
    }

    return &notificationv1.BatchSendNotificationsResponse{
        TotalCount:   int32(responses.TotalCount),
        SuccessCount: int32(responses.SuccessCount),
        Results:      results,
    }, nil
}

// 新增方法：构建包含幂等信息的gRPC响应
func (s *NotificationServer) buildGRPCSendResponseWithIdempotent(
    result domain.SendResponse,
) *notificationv1.SendNotificationResponse {
    response := &notificationv1.SendNotificationResponse{
        NotificationId: result.NotificationID,
        Status:         s.convertToGRPCSendStatus(result.Status),
        IsIdempotent:   result.IsIdempotent,
        ProcessedAt:    result.ProcessedAt.Unix(),
    }

    if result.Error != nil {
        response.ErrorMessage = result.Error.Error()
        response.ErrorCode = s.convertToGRPCErrorCode(result.Error)
        if response.Status != notificationv1.SendStatus_FAILED {
            response.Status = notificationv1.SendStatus_FAILED
        }
    }

    return response
}
```

### 步骤4：更新Proto定义

```protobuf
// api/proto/notification/v1/notification.proto
// 在SendNotificationResponse中添加字段

message SendNotificationResponse {
  uint64 notification_id = 1;
  SendStatus status = 2;
  ErrorCode error_code = 3;
  string error_message = 4;
  bool is_idempotent = 5;    // 新增：是否为幂等响应
  int64 processed_at = 6;    // 新增：处理时间戳
}

message BatchSendNotificationsResponse {
  repeated SendNotificationResponse results = 1;
  int32 total_count = 2;
  int32 success_count = 3;
  int32 idempotent_count = 4;  // 新增：幂等冲突数量
  int32 failed_count = 5;      // 新增：失败数量
}
```

### 步骤5：修改发送策略

```go
// internal/service/sendstrategy/immediate.go
// 修改BatchSend方法以支持部分失败处理

func (s *ImmediateSendStrategy) BatchSend(
    ctx context.Context, 
    notifications []domain.Notification,
) ([]domain.SendResponse, error) {
    if len(notifications) == 0 {
        return nil, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
    }

    for i := range notifications {
        notifications[i].SetSendTime()
    }

    // 尝试批量创建
    createdNotifications, err := s.repo.BatchCreate(ctx, notifications)
    if err != nil {
        // 如果是唯一索引冲突，说明幂等检测有遗漏，进行单个处理
        if errors.Is(err, errs.ErrNotificationDuplicate) {
            return s.handlePartialConflicts(ctx, notifications)
        }
        return nil, fmt.Errorf("创建通知失败: %w", err)
    }

    // 立即发送
    return s.sender.BatchSend(ctx, createdNotifications)
}

// 处理部分冲突的情况（兜底逻辑）
func (s *ImmediateSendStrategy) handlePartialConflicts(
    ctx context.Context, 
    notifications []domain.Notification,
) ([]domain.SendResponse, error) {
    responses := make([]domain.SendResponse, 0, len(notifications))

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

        // 处理唯一索引冲突
        if errors.Is(err, errs.ErrNotificationDuplicate) {
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

            responses = append(responses, domain.SendResponse{
                NotificationID: existing.ID,
                Status:         existing.Status,
                IsIdempotent:   true,
                ProcessedAt:    time.Now(),
            })
            continue
        }

        // 其他错误
        responses = append(responses, domain.SendResponse{
            NotificationID: notification.ID,
            Status:         domain.SendStatusFailed,
            ProcessedAt:    time.Now(),
            Error:          err,
        })
    }

    return responses, nil
}
```

## 3. 依赖注入配置

```go
// cmd/platform/ioc/wire.go 或相应的依赖注入文件
// 添加BatchIdempotencyService的构建

func InitBatchIdempotencyService(
    idempotentSvc idempotent.IdempotencyService,
    repo repository.NotificationRepository,
) *idempotency.BatchIdempotencyService {
    return idempotency.NewBatchIdempotencyService(idempotentSvc, repo)
}

// 更新sendService的构建
func InitSendService(
    idGenerator *id.Generator,
    sendStrategy sendstrategy.SendStrategy,
    batchIdempotencyService *idempotency.BatchIdempotencyService,
) notification.SendService {
    return notification.NewSendService(
        idGenerator,
        sendStrategy,
        batchIdempotencyService,
    )
}
```

## 4. 测试验证

### 集成测试示例

```go
func TestBatchSendNotifications_IdempotencyHandling(t *testing.T) {
    // 设置测试环境
    suite := setupTestSuite(t)
    defer suite.tearDown()

    // 测试场景1：部分幂等冲突
    t.Run("部分幂等冲突", func(t *testing.T) {
        // 预先创建一些通知
        existingNotifications := createTestNotifications(2)
        for _, n := range existingNotifications {
            _, err := suite.repo.Create(context.Background(), n)
            require.NoError(t, err)
        }

        // 构建混合批次（包含已存在和新的通知）
        batchNotifications := append(existingNotifications, createTestNotifications(2)...)

        // 执行批量发送
        response, err := suite.sendService.BatchSendNotifications(
            context.Background(), 
            batchNotifications...,
        )

        // 验证结果
        require.NoError(t, err)
        assert.Equal(t, 4, response.TotalCount)
        assert.Equal(t, 2, response.SuccessCount)
        assert.Equal(t, 2, response.IdempotentCount)
        assert.Equal(t, 0, response.FailedCount)

        // 验证幂等响应的正确性
        idempotentCount := 0
        for _, result := range response.Results {
            if result.IsIdempotent {
                idempotentCount++
                assert.NotZero(t, result.NotificationID)
                assert.False(t, result.ProcessedAt.IsZero())
            }
        }
        assert.Equal(t, 2, idempotentCount)
    })

    // 测试场景2：完全重试
    t.Run("完全重试", func(t *testing.T) {
        // 预先创建所有通知
        notifications := createTestNotifications(3)
        for _, n := range notifications {
            _, err := suite.repo.Create(context.Background(), n)
            require.NoError(t, err)
        }

        // 重试相同的批次
        response, err := suite.sendService.BatchSendNotifications(
            context.Background(), 
            notifications...,
        )

        // 验证结果
        require.NoError(t, err)
        assert.Equal(t, 3, response.TotalCount)
        assert.Equal(t, 0, response.SuccessCount)
        assert.Equal(t, 3, response.IdempotentCount)
        assert.Equal(t, 0, response.FailedCount)
    })
}
```

## 5. 监控和告警

### 关键指标监控

```go
// 在发送服务中添加指标收集
func (e *sendService) collectMetrics(response domain.BatchSendResponse) {
    // 记录批量处理指标
    metrics.BatchProcessingTotal.Add(float64(response.TotalCount))
    metrics.BatchProcessingSuccess.Add(float64(response.SuccessCount))
    metrics.BatchProcessingIdempotent.Add(float64(response.IdempotentCount))
    metrics.BatchProcessingFailed.Add(float64(response.FailedCount))

    // 计算幂等冲突率
    if response.TotalCount > 0 {
        idempotentRate := float64(response.IdempotentCount) / float64(response.TotalCount)
        metrics.IdempotentConflictRate.Set(idempotentRate)
    }
}
```

这个实现方案提供了：
1. **向后兼容性**：保持现有API不变
2. **优雅的错误处理**：区分成功、失败和幂等冲突
3. **精确的结果反馈**：业务方可以准确知道每个通知的处理状态
4. **良好的性能**：批量操作减少网络往返
5. **完整的监控**：提供详细的处理统计信息

通过这种方式，可以彻底解决批量接口的幂等问题，提供更好的用户体验。
