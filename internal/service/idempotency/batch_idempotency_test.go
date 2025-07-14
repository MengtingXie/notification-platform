package idempotency

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"gitee.com/flycash/notification-platform/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockIdempotencyService 模拟幂等服务
type MockIdempotencyService struct {
	mock.Mock
}

func (m *MockIdempotencyService) Exists(ctx context.Context, key string) (bool, error) {
	args := m.Called(ctx, key)
	return args.Bool(0), args.Error(1)
}

func (m *MockIdempotencyService) MExists(ctx context.Context, keys ...string) ([]bool, error) {
	args := m.Called(ctx, keys)
	return args.Get(0).([]bool), args.Error(1)
}

// MockNotificationRepository 模拟通知仓库
type MockNotificationRepository struct {
	mock.Mock
}

func (m *MockNotificationRepository) UpdateStatus(ctx context.Context, notification domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkTimeoutSendingAsFailed(ctx context.Context, batchSize int) (int64, error) {
	args := m.Called(ctx, batchSize)
	return args.Get(0).(int64), args.Error(1)
}

func (m *MockNotificationRepository) MarkSuccess(ctx context.Context, notification domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) MarkFailed(ctx context.Context, notification domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) GetByKeys(ctx context.Context, bizID int64, keys ...string) ([]domain.Notification, error) {
	args := m.Called(ctx, bizID, keys)
	return args.Get(0).([]domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByID(ctx context.Context, id uint64) (domain.Notification, error) {
	args := m.Called(ctx, id)
	return args.Get(0).(domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) FindReadyNotifications(ctx context.Context, offset int, limit int) ([]domain.Notification, error) {
	args := m.Called(ctx, offset, limit)
	return args.Get(0).([]domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) CreateWithCallbackLog(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	args := m.Called(ctx, notification)
	return args.Get(0).(domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) CASStatus(ctx context.Context, notification domain.Notification) error {
	args := m.Called(ctx, notification)
	return args.Error(0)
}

func (m *MockNotificationRepository) BatchUpdateStatusSucceededOrFailed(ctx context.Context, succeededNotifications, failedNotifications []domain.Notification) error {
	args := m.Called(ctx, succeededNotifications, failedNotifications)
	return args.Error(0)
}

func (m *MockNotificationRepository) BatchGetByIDs(ctx context.Context, ids []uint64) (map[uint64]domain.Notification, error) {
	args := m.Called(ctx, ids)
	return args.Get(0).(map[uint64]domain.Notification), args.Error(1)
}

// BatchCreateWithCallbackLog mocks the batch creation of notifications with callback log
func (m *MockNotificationRepository) BatchCreateWithCallbackLog(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	args := m.Called(ctx, notifications)
	return args.Get(0).([]domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) GetByKey(ctx context.Context, bizID int64, key string) (domain.Notification, error) {
	args := m.Called(ctx, bizID, key)
	return args.Get(0).(domain.Notification), args.Error(1)
}

// 实现其他必要的接口方法（简化版）
func (m *MockNotificationRepository) Create(ctx context.Context, notification domain.Notification) (domain.Notification, error) {
	args := m.Called(ctx, notification)
	return args.Get(0).(domain.Notification), args.Error(1)
}

func (m *MockNotificationRepository) BatchCreate(ctx context.Context, notifications []domain.Notification) ([]domain.Notification, error) {
	args := m.Called(ctx, notifications)
	return args.Get(0).([]domain.Notification), args.Error(1)
}

// 创建测试通知
func createTestNotifications(count int) []domain.Notification {
	notifications := make([]domain.Notification, count)
	for i := 0; i < count; i++ {
		notifications[i] = domain.Notification{
			ID:     uint64(i + 1),
			BizID:  12345,
			Key:    fmt.Sprintf("test-key-%d", i+1),
			Status: domain.SendStatusPending,
		}
	}
	return notifications
}

func TestBatchIdempotencyService_ClassifyNotifications(t *testing.T) {
	tests := []struct {
		name          string
		notifications []domain.Notification
		existsResults []bool
		existsError   error
		expectedNew   int
		expectedIdem  int
		expectError   bool
	}{
		{
			name:          "全部为新通知",
			notifications: createTestNotifications(3),
			existsResults: []bool{false, false, false},
			expectedNew:   3,
			expectedIdem:  0,
			expectError:   false,
		},
		{
			name:          "全部为幂等冲突",
			notifications: createTestNotifications(3),
			existsResults: []bool{true, true, true},
			expectedNew:   0,
			expectedIdem:  3,
			expectError:   false,
		},
		{
			name:          "部分幂等冲突",
			notifications: createTestNotifications(4),
			existsResults: []bool{false, true, false, true},
			expectedNew:   2,
			expectedIdem:  2,
			expectError:   false,
		},
		{
			name:          "空通知列表",
			notifications: []domain.Notification{},
			existsResults: []bool{},
			expectedNew:   0,
			expectedIdem:  0,
			expectError:   false,
		},
		{
			name:          "幂等检测失败",
			notifications: createTestNotifications(2),
			existsError:   errors.New("redis connection failed"),
			expectError:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟服务
			mockIdempotent := &MockIdempotencyService{}
			mockRepo := &MockNotificationRepository{}

			service := NewBatchIdempotencyService(mockIdempotent, mockRepo)

			// 设置期望
			if len(tt.notifications) > 0 {
				keys := make([]string, len(tt.notifications))
				for i, n := range tt.notifications {
					keys[i] = fmt.Sprintf("%d-%s", n.BizID, n.Key)
				}
				mockIdempotent.On("MExists", mock.Anything, keys).Return(tt.existsResults, tt.existsError)
			}

			// 执行测试
			ctx := context.Background()
			result, err := service.ClassifyNotifications(ctx, tt.notifications)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, result)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, result)
				assert.Equal(t, tt.expectedNew, len(result.NewNotifications))
				assert.Equal(t, tt.expectedIdem, len(result.IdempotentNotifications))

				// 验证分类结果的正确性
				err = service.ValidateClassification(tt.notifications, result)
				assert.NoError(t, err)
			}

			mockIdempotent.AssertExpectations(t)
		})
	}
}

func TestBatchIdempotencyService_HandleIdempotentNotifications(t *testing.T) {
	tests := []struct {
		name          string
		notifications []domain.Notification
		existingData  []domain.Notification
		queryErrors   []error
		expectedCount int
		expectError   bool
	}{
		{
			name:          "成功处理幂等通知",
			notifications: createTestNotifications(2),
			existingData: []domain.Notification{
				{ID: 100, BizID: 12345, Key: "test-key-1", Status: domain.SendStatusSucceeded},
				{ID: 101, BizID: 12345, Key: "test-key-2", Status: domain.SendStatusPending},
			},
			queryErrors:   []error{nil, nil},
			expectedCount: 2,
			expectError:   false,
		},
		{
			name:          "查询失败",
			notifications: createTestNotifications(1),
			existingData:  []domain.Notification{{}},
			queryErrors:   []error{errors.New("database error")},
			expectedCount: 1, // 仍然返回结果，但包含错误
			expectError:   false,
		},
		{
			name:          "空通知列表",
			notifications: []domain.Notification{},
			expectedCount: 0,
			expectError:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建模拟服务
			mockIdempotent := &MockIdempotencyService{}
			mockRepo := &MockNotificationRepository{}

			service := NewBatchIdempotencyService(mockIdempotent, mockRepo)

			// 设置期望
			for i, notification := range tt.notifications {
				if i < len(tt.existingData) && i < len(tt.queryErrors) {
					mockRepo.On("GetByKey", mock.Anything, notification.BizID, notification.Key).
						Return(tt.existingData[i], tt.queryErrors[i])
				}
			}

			// 执行测试
			ctx := context.Background()
			results, err := service.HandleIdempotentNotifications(ctx, tt.notifications)

			// 验证结果
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expectedCount, len(results))

				// 验证所有结果都标记为幂等
				for _, result := range results {
					assert.True(t, result.IsIdempotent)
					assert.False(t, result.ProcessedAt.IsZero())
				}
			}

			mockRepo.AssertExpectations(t)
		})
	}
}

func TestReorderResults(t *testing.T) {
	tests := []struct {
		name          string
		original      []domain.Notification
		results       []domain.SendResponse
		expectedOrder []uint64
	}{
		{
			name: "正常排序",
			original: []domain.Notification{
				{ID: 1, BizID: 123, Key: "key1"},
				{ID: 2, BizID: 123, Key: "key2"},
				{ID: 3, BizID: 123, Key: "key3"},
			},
			results: []domain.SendResponse{
				{NotificationID: 3, Status: domain.SendStatusSucceeded},
				{NotificationID: 1, Status: domain.SendStatusSucceeded},
				{NotificationID: 2, Status: domain.SendStatusSucceeded},
			},
			expectedOrder: []uint64{1, 2, 3},
		},
		{
			name: "长度不匹配",
			original: []domain.Notification{
				{ID: 1, BizID: 123, Key: "key1"},
				{ID: 2, BizID: 123, Key: "key2"},
			},
			results: []domain.SendResponse{
				{NotificationID: 1, Status: domain.SendStatusSucceeded},
			},
			expectedOrder: []uint64{1}, // 直接返回原结果
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reordered := ReorderResults(tt.original, tt.results)

			if len(tt.original) != len(tt.results) {
				// 长度不匹配时，应该直接返回原结果
				assert.Equal(t, len(tt.results), len(reordered))
			} else {
				// 验证排序结果
				assert.Equal(t, len(tt.expectedOrder), len(reordered))
				for i, expected := range tt.expectedOrder {
					assert.Equal(t, expected, reordered[i].NotificationID)
				}
			}
		})
	}
}

func TestBatchIdempotencyService_ValidateClassification(t *testing.T) {
	tests := []struct {
		name           string
		original       []domain.Notification
		classification *NotificationClassification
		expectError    bool
		errorContains  string
	}{
		{
			name:     "有效分类",
			original: createTestNotifications(3),
			classification: &NotificationClassification{
				NewNotifications:        createTestNotifications(3)[1:],
				IdempotentNotifications: createTestNotifications(1),
			},
			expectError: false,
		},
		{
			name:     "数量不匹配",
			original: createTestNotifications(3),
			classification: &NotificationClassification{
				NewNotifications:        createTestNotifications(1),
				IdempotentNotifications: createTestNotifications(1),
			},
			expectError:   true,
			errorContains: "数量不匹配",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			service := &BatchIdempotencyService{}
			err := service.ValidateClassification(tt.original, tt.classification)

			if tt.expectError {
				assert.Error(t, err)
				if tt.errorContains != "" {
					assert.Contains(t, err.Error(), tt.errorContains)
				}
			} else {
				assert.NoError(t, err)
			}
		})
	}
}
