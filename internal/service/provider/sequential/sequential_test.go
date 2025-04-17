//go:build unit

package sequential

import (
	"testing"

	"gitee.com/flycash/notification-platform/internal/domain"
	"gitee.com/flycash/notification-platform/internal/errs"
	"gitee.com/flycash/notification-platform/internal/service/provider"
	providermocks "gitee.com/flycash/notification-platform/internal/service/provider/mocks"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
)

func TestNewSelectorBuilder(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name             string
		getProvidersFunc func(ctrl *gomock.Controller) []provider.Provider
	}{
		{
			name: "空供应商列表",
			getProvidersFunc: func(_ *gomock.Controller) []provider.Provider {
				return []provider.Provider{}
			},
		},
		{
			name: "单个供应商",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				return []provider.Provider{
					providermocks.NewMockProvider(ctrl),
				}
			},
		},
		{
			name: "多个供应商",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				return []provider.Provider{
					providermocks.NewMockProvider(ctrl),
					providermocks.NewMockProvider(ctrl),
					providermocks.NewMockProvider(ctrl),
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			providers := tt.getProvidersFunc(ctrl)

			builder := NewSelectorBuilder(providers)
			assert.NotNil(t, builder)
		})
	}
}

func TestSelectorBuilder_Build(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name            string
		setupFunc       func(ctrl *gomock.Controller) []provider.Provider
		errorAssertFunc assert.ErrorAssertionFunc
	}{
		{
			name: "创建空选择器",
			setupFunc: func(_ *gomock.Controller) []provider.Provider {
				return []provider.Provider{}
			},
			errorAssertFunc: assert.NoError,
		},
		{
			name: "创建包含供应商的选择器",
			setupFunc: func(ctrl *gomock.Controller) []provider.Provider {
				return []provider.Provider{
					providermocks.NewMockProvider(ctrl),
					providermocks.NewMockProvider(ctrl),
				}
			},
			errorAssertFunc: assert.NoError,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			providers := tt.setupFunc(ctrl)
			builder := NewSelectorBuilder(providers)

			selector, err := builder.Build()
			tt.errorAssertFunc(t, err)

			if err != nil {
				return
			}
			assert.NotNil(t, selector)
		})
	}
}

func TestSelector_Next(t *testing.T) {
	t.Parallel()

	testNotification := domain.Notification{
		ID:      uint64(12345),
		Channel: domain.ChannelSMS,
		Template: domain.Template{
			ID:        1,
			VersionID: 1,
			Params:    map[string]string{"code": "123456"},
		},
		Receivers: []string{"13800138000"},
	}

	tests := []struct {
		name             string
		getProvidersFunc func(ctrl *gomock.Controller) []provider.Provider
		calls            int
		wantErr          error
	}{
		{
			name: "空供应商列表",
			getProvidersFunc: func(_ *gomock.Controller) []provider.Provider {
				return []provider.Provider{}
			},
			calls:   1,
			wantErr: errs.ErrNoAvailableProvider,
		},
		{
			name: "单个供应商-获取成功",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				mockProvider := providermocks.NewMockProvider(ctrl)
				return []provider.Provider{mockProvider}
			},
			calls:   1,
			wantErr: nil,
		},
		{
			name: "单个供应商-获取两次-第二次失败",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				mockProvider := providermocks.NewMockProvider(ctrl)
				return []provider.Provider{mockProvider}
			},
			calls:   2,
			wantErr: errs.ErrNoAvailableProvider,
		},
		{
			name: "多个供应商-获取全部",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				provider1 := providermocks.NewMockProvider(ctrl)
				provider2 := providermocks.NewMockProvider(ctrl)
				provider3 := providermocks.NewMockProvider(ctrl)
				return []provider.Provider{provider1, provider2, provider3}
			},
			calls:   3,
			wantErr: nil,
		},
		{
			name: "多个供应商-获取超出范围",
			getProvidersFunc: func(ctrl *gomock.Controller) []provider.Provider {
				provider1 := providermocks.NewMockProvider(ctrl)
				provider2 := providermocks.NewMockProvider(ctrl)
				provider3 := providermocks.NewMockProvider(ctrl)
				return []provider.Provider{provider1, provider2, provider3}
			},
			calls:   4,
			wantErr: errs.ErrNoAvailableProvider,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			providers := tt.getProvidersFunc(ctrl)

			// 创建选择器
			builder := NewSelectorBuilder(providers)
			selector, err := builder.Build()
			require.NoError(t, err)
			require.NotNil(t, selector)

			// 执行测试 - 调用Next方法多次
			var lastErr error

			for i := 0; i < tt.calls; i++ {
				p, err1 := selector.Next(t.Context(), testNotification)
				lastErr = err1

				switch {
				case i == tt.calls-1 && tt.wantErr != nil:
					// 如果是最后一次调用且预期有错误
					assert.ErrorIs(t, err1, tt.wantErr)
				case i < len(providers):
					// 如果在前几次调用且仍在供应商个数范围内
					assert.NoError(t, err1)
					assert.Equal(t, providers[i], p)
				default:
					// 如果超出供应商个数范围
					assert.ErrorIs(t, err1, errs.ErrNoAvailableProvider)
					assert.Nil(t, p)
				}
			}

			// 验证最终结果
			if tt.wantErr != nil {
				assert.ErrorIs(t, lastErr, tt.wantErr)
			}
		})
	}
}
