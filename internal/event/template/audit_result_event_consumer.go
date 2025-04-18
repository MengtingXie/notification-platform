package template

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"gitee.com/flycash/notification-platform/internal/domain"
	auditevt "gitee.com/flycash/notification-platform/internal/event/audit"
	templatesvc "gitee.com/flycash/notification-platform/internal/service/template/manage"
	"github.com/ecodeclub/mq-api"
	"github.com/gotomicro/ego/core/elog"
)

const (
	auditEventName = "audit_result_events"
)

type AuditResultConsumer struct {
	svc      templatesvc.ChannelTemplateService
	consumer mq.Consumer
	logger   *elog.Component
}

func NewAuditResultConsumer(svc templatesvc.ChannelTemplateService, q mq.MQ) (*AuditResultConsumer, error) {
	const groupID = "template"
	consumer, err := q.Consumer(auditEventName, groupID)
	if err != nil {
		return nil, err
	}
	return &AuditResultConsumer{
		svc:      svc,
		consumer: consumer,
		logger:   elog.DefaultLogger,
	}, nil
}

func (c *AuditResultConsumer) Start(ctx context.Context) {
	go func() {
		for {
			er := c.Consume(ctx)
			if er != nil {
				c.logger.Error("消费审核状态结果事件失败", elog.FieldErr(er))
			}
		}
	}()
}

func (c *AuditResultConsumer) Consume(ctx context.Context) error {
	msgCh, err := c.consumer.ConsumeChan(ctx)
	if err != nil {
		return fmt.Errorf("获取消息失败: %w", err)
	}

	// 限制批次大小
	const batchSize = 20

	// 限制时间
	const timeLimit = 3 * time.Second
	timer := time.NewTimer(timeLimit)
	defer timer.Stop()

	versions := make([]domain.ChannelTemplateVersion, 0, batchSize)
	versionIDs := make([]int64, 0, batchSize)
	var evt auditevt.CallbackResultEvent

CollectBatch:
	for {
		select {
		case msg, ok := <-msgCh:
			// 检查通道是否已关闭
			if !ok {
				break CollectBatch
			}

			err = json.Unmarshal(msg.Value, &evt)
			if err != nil {
				c.logger.Warn("解析消息失败",
					elog.FieldErr(err),
					elog.Any("msg", msg.Value))
				continue
			}

			if !evt.ResourceType.IsTemplate() {
				continue
			}

			version := domain.ChannelTemplateVersion{
				ID:           evt.ResourceID,
				AuditID:      evt.AuditID,
				AuditorID:    evt.AuditorID,
				AuditTime:    evt.AuditTime,
				AuditStatus:  domain.AuditStatus(evt.AuditStatus),
				RejectReason: evt.RejectReason,
			}

			if version.AuditStatus.IsApproved() {
				versionIDs = append(versionIDs, version.ID)
			}

			versions = append(versions, version)

			// 达到批量大小限制，跳出循环
			if len(versions) == batchSize {
				break CollectBatch
			}

		case <-timer.C:
			// 达到时间限制，跳出循环
			break CollectBatch
		case <-ctx.Done():
			// 上下文被取消，不返回err
			break CollectBatch
		}
	}

	// 没有可以更新的内容
	if len(versions) == 0 {
		return nil
	}

	err = c.svc.BatchUpdateVersionAuditStatus(ctx, versions)
	if err != nil {
		c.logger.Warn("更新模版版本内部审核信息失败",
			elog.FieldErr(err),
			elog.Any("versions", versions),
		)
		return err
	}

	err = c.svc.BatchSubmitForProviderReview(ctx, versionIDs)
	if err != nil {
		c.logger.Warn("内部审核通过的模版，提交到供应商侧审核失败",
			elog.FieldErr(err),
			elog.Any("versionIDs", versionIDs),
		)
		return err
	}

	return nil
}
