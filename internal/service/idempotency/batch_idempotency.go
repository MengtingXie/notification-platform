package idempotency

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/pkg/idempotent"
	"gitee.com/flycash/notification-platform/internal/repository"
)

// BatchIdempotencyService 批量幂等服务
type BatchIdempotencyService struct {
	idempotentSvc idempotent.IdempotencyService
	repo          repository.NotificationRepository
}

// NewBatchIdempotencyService 创建批量幂等服务
func NewBatchIdempotencyService(
	idempotentSvc idempotent.IdempotencyService,
	repo repository.NotificationRepository,
) *BatchIdempotencyService {
	return &BatchIdempotencyService{
		idempotentSvc: idempotentSvc,
		repo:          repo,
	}
}

// NotificationClassification 通知分类结果
type NotificationClassification struct {
	NewNotifications        []domain.Notification // 新通知
	IdempotentNotifications []domain.Notification // 幂等冲突通知
	NotificationIndexMap    map[string]int        // key -> 原始索引的映射
}

// ClassifyNotifications 对通知进行幂等性分类
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

// HandleIdempotentNotifications 处理幂等冲突的通知
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

// BatchProcessResult 批量处理结果
type BatchProcessResult struct {
	SuccessResults    []domain.SendResponse // 成功处理的通知
	IdempotentResults []domain.SendResponse // 幂等冲突的通知
	FailedResults     []domain.SendResponse // 处理失败的通知
}

// CombineResults 合并处理结果
func (r *BatchProcessResult) CombineResults() []domain.SendResponse {
	totalLen := len(r.SuccessResults) + len(r.IdempotentResults) + len(r.FailedResults)
	combined := make([]domain.SendResponse, 0, totalLen)

	combined = append(combined, r.SuccessResults...)
	combined = append(combined, r.IdempotentResults...)
	combined = append(combined, r.FailedResults...)

	return combined
}

// GetCounts 获取各种状态的数量统计
func (r *BatchProcessResult) GetCounts() (success, idempotent, failed int) {
	return len(r.SuccessResults), len(r.IdempotentResults), len(r.FailedResults)
}

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

// ValidateClassification 验证分类结果的正确性
func (s *BatchIdempotencyService) ValidateClassification(
	original []domain.Notification,
	classification *NotificationClassification,
) error {
	totalClassified := len(classification.NewNotifications) + len(classification.IdempotentNotifications)
	if totalClassified != len(original) {
		return fmt.Errorf("分类结果数量不匹配: 原始=%d, 分类=%d", len(original), totalClassified)
	}

	// 验证没有重复的通知
	seen := make(map[string]bool)
	for _, n := range classification.NewNotifications {
		key := fmt.Sprintf("%d-%s", n.BizID, n.Key)
		if seen[key] {
			return fmt.Errorf("发现重复的新通知: %s", key)
		}
		seen[key] = true
	}

	for _, n := range classification.IdempotentNotifications {
		key := fmt.Sprintf("%d-%s", n.BizID, n.Key)
		if seen[key] {
			return fmt.Errorf("发现重复的幂等通知: %s", key)
		}
		seen[key] = true
	}

	return nil
}
