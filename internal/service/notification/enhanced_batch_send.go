package notification

import (
	"context"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	id "gitee.com/flycash/notification-platform/internal/pkg/id_generator"
	"gitee.com/flycash/notification-platform/internal/service/idempotency"
	"gitee.com/flycash/notification-platform/internal/service/sendstrategy"
)

// EnhancedBatchSendService 增强的批量发送服务，支持优雅的幂等处理
type EnhancedBatchSendService struct {
	idGenerator        *id.Generator
	sendStrategy       sendstrategy.SendStrategy
	idempotencyService *idempotency.BatchIdempotencyService
}

// NewEnhancedBatchSendService 创建增强的批量发送服务
func NewEnhancedBatchSendService(
	idGenerator *id.Generator,
	sendStrategy sendstrategy.SendStrategy,
	idempotencyService *idempotency.BatchIdempotencyService,
) *EnhancedBatchSendService {
	return &EnhancedBatchSendService{
		idGenerator:        idGenerator,
		sendStrategy:       sendStrategy,
		idempotencyService: idempotencyService,
	}
}

// BatchSendNotifications 批量发送通知，支持优雅的幂等处理
func (s *EnhancedBatchSendService) BatchSendNotifications(
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
		// 生成通知ID
		id := s.idGenerator.GenerateID(notifications[i].BizID, notifications[i].Key)
		notifications[i].ID = uint64(id)
	}

	// 3. 幂等性分类
	classification, err := s.idempotencyService.ClassifyNotifications(ctx, notifications)
	if err != nil {
		return response, fmt.Errorf("幂等性检测失败: %w", err)
	}

	// 4. 验证分类结果
	if err := s.idempotencyService.ValidateClassification(notifications, classification); err != nil {
		return response, fmt.Errorf("分类结果验证失败: %w", err)
	}

	var allResults []domain.SendResponse

	// 5. 处理幂等冲突的通知
	if len(classification.IdempotentNotifications) > 0 {
		idempotentResults, err := s.idempotencyService.HandleIdempotentNotifications(
			ctx,
			classification.IdempotentNotifications,
		)
		if err != nil {
			return response, fmt.Errorf("处理幂等通知失败: %w", err)
		}
		allResults = append(allResults, idempotentResults...)
	}

	// 6. 处理新通知
	if len(classification.NewNotifications) > 0 {
		newResults, err := s.sendStrategy.BatchSend(ctx, classification.NewNotifications)
		if err != nil {
			// 对于新通知的处理失败，需要回滚幂等标记
			rollbackErr := s.idempotencyService.RollbackIdempotencyMarks(ctx, classification.NewNotifications)
			if rollbackErr != nil {
				// 记录回滚失败的日志，但不影响主要错误的返回
				// TODO: 添加日志记录
			}
			return response, fmt.Errorf("发送新通知失败: %w", err)
		}

		// 为新处理的结果设置处理时间
		for i := range newResults {
			if newResults[i].ProcessedAt.IsZero() {
				newResults[i].ProcessedAt = time.Now()
			}
		}

		allResults = append(allResults, newResults...)
	}

	// 7. 按原始顺序重新排列结果
	response.Results = idempotency.ReorderResults(notifications, allResults)

	// 8. 计算统计信息
	s.calculateResponseStats(&response)

	return response, nil
}

// calculateResponseStats 计算响应统计信息
func (s *EnhancedBatchSendService) calculateResponseStats(response *domain.BatchSendResponse) {
	response.TotalCount = len(response.Results)
	response.SuccessCount = 0
	response.IdempotentCount = 0
	response.FailedCount = 0

	for _, result := range response.Results {
		switch {
		case result.Error != nil:
			response.FailedCount++
		case result.IsIdempotent:
			response.IdempotentCount++
		default:
			switch result.Status {
			case domain.SendStatusSucceeded, domain.SendStatusPending:
				response.SuccessCount++
			case domain.SendStatusFailed:
				response.FailedCount++
			default:
				// 其他状态暂时归类为成功
				response.SuccessCount++
			}
		}
	}
}

// BatchSendNotificationsAsync 异步批量发送通知，支持优雅的幂等处理
func (s *EnhancedBatchSendService) BatchSendNotificationsAsync(
	ctx context.Context,
	notifications ...domain.Notification,
) (domain.BatchSendAsyncResponse, error) {
	// 参数校验
	if len(notifications) == 0 {
		return domain.BatchSendAsyncResponse{}, fmt.Errorf("%w: 通知列表不能为空", errs.ErrInvalidParameter)
	}

	// 生成ID并进行校验
	for i := range notifications {
		if err := notifications[i].Validate(); err != nil {
			return domain.BatchSendAsyncResponse{}, fmt.Errorf("参数非法 %w", err)
		}
		// 生成通知ID
		id := s.idGenerator.GenerateID(notifications[i].BizID, notifications[i].Key)
		notifications[i].ID = uint64(id)
		notifications[i].ReplaceAsyncImmediate()
	}

	// 幂等性分类
	classification, err := s.idempotencyService.ClassifyNotifications(ctx, notifications)
	if err != nil {
		return domain.BatchSendAsyncResponse{}, fmt.Errorf("幂等性检测失败: %w", err)
	}

	// 处理幂等冲突的通知 - 异步场景下只需要返回已存在的ID
	var existingIDs []uint64
	if len(classification.IdempotentNotifications) > 0 {
		idempotentResults, err := s.idempotencyService.HandleIdempotentNotifications(
			ctx,
			classification.IdempotentNotifications,
		)
		if err != nil {
			return domain.BatchSendAsyncResponse{}, fmt.Errorf("处理幂等通知失败: %w", err)
		}

		for _, result := range idempotentResults {
			existingIDs = append(existingIDs, result.NotificationID)
		}
	}

	// 处理新通知
	var newIDs []uint64
	if len(classification.NewNotifications) > 0 {
		_, err := s.sendStrategy.BatchSend(ctx, classification.NewNotifications)
		if err != nil {
			// 回滚幂等标记
			rollbackErr := s.idempotencyService.RollbackIdempotencyMarks(ctx, classification.NewNotifications)
			if rollbackErr != nil {
				// 记录回滚失败的日志
				// TODO: 添加日志记录
			}
			return domain.BatchSendAsyncResponse{}, fmt.Errorf("发送失败 %w", errs.ErrSendNotificationFailed)
		}

		for i := range classification.NewNotifications {
			newIDs = append(newIDs, classification.NewNotifications[i].ID)
		}
	}

	// 合并所有ID并按原始顺序排列
	existingIDs = append(existingIDs, newIDs...)

	return domain.BatchSendAsyncResponse{
		NotificationIDs: s.reorderIDs(notifications, existingIDs),
	}, nil
}

// reorderIDs 按原始顺序重新排列ID
func (s *EnhancedBatchSendService) reorderIDs(
	originalNotifications []domain.Notification,
	resultIDs []uint64,
) []uint64 {
	if len(originalNotifications) != len(resultIDs) {
		// 长度不匹配，直接返回
		return resultIDs
	}

	// 创建ID到索引的映射
	idToIndex := make(map[uint64]int)
	for i := range originalNotifications {
		idToIndex[originalNotifications[i].ID] = i
	}

	// 按原始顺序排列
	orderedIDs := make([]uint64, len(originalNotifications))
	usedIndices := make(map[int]bool)

	for _, id := range resultIDs {
		if index, exists := idToIndex[id]; exists && !usedIndices[index] {
			orderedIDs[index] = id
			usedIndices[index] = true
		}
	}

	return orderedIDs
}

// GetBatchProcessingStats 获取批量处理统计信息（用于监控）
func (s *EnhancedBatchSendService) GetBatchProcessingStats(
	_ context.Context,
	results []domain.SendResponse,
) map[string]int {
	stats := map[string]int{
		"total":      len(results),
		"success":    0,
		"idempotent": 0,
		"failed":     0,
	}

	for _, result := range results {
		switch {
		case result.Error != nil:
			stats["failed"]++
		case result.IsIdempotent:
			stats["idempotent"]++
		default:
			stats["success"]++
		}
	}

	return stats
}
