package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gitee.com/flycash/notification-platform/internal/service/executor/internal/service/sender"
	notificationsvc "gitee.com/flycash/notification-platform/internal/service/notification"
)

// NotificationScheduler 通知调度服务接口
type NotificationScheduler interface {
	// Start 启动调度服务
	Start() error

	// Stop 停止调度服务
	Stop() error
}

// scheduler 通知调度服务实现
type scheduler struct {
	notificationSvc notificationsvc.Service
	sender          sender.NotificationSender

	batchSize       int
	intervalSeconds int
	running         bool
	stopCh          chan struct{}
	wg              sync.WaitGroup
}

// NewScheduler 创建通知调度服务
func NewScheduler(
	notificationSvc notificationsvc.Service,
	dispatcher sender.NotificationSender,
	batchSize int,
	intervalSeconds int,
) NotificationScheduler {
	return &scheduler{
		notificationSvc: notificationSvc,
		sender:          dispatcher,
		batchSize:       batchSize,
		intervalSeconds: intervalSeconds,
		stopCh:          make(chan struct{}),
	}
}

// Start 启动调度服务
func (s *scheduler) Start() error {
	if s.running {
		return fmt.Errorf("调度服务已经运行")
	}

	s.running = true
	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		s.scheduleLoop()
	}()

	return nil
}

// Stop 停止调度服务
func (s *scheduler) Stop() error {
	if !s.running {
		return nil
	}

	s.running = false
	close(s.stopCh)
	s.wg.Wait()

	return nil
}

// scheduleLoop 调度循环
func (s *scheduler) scheduleLoop() {
	ticker := time.NewTicker(time.Duration(s.intervalSeconds) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.processPendingNotifications()
		case <-s.stopCh:
			return
		}
	}
}

// processPendingNotifications 处理待发送的通知
func (s *scheduler) processPendingNotifications() {
	const thirty = 30
	ctx, cancel := context.WithTimeout(context.Background(), thirty*time.Second)
	defer cancel()

	// 当前时间必须大于等于开始时间
	now := time.Now().UnixMilli()

	// 获取符合发送时间的待发送通知
	notifications, err := s.getNotifications(
		ctx,
		now,
		s.batchSize,
	)
	if err != nil {
		fmt.Printf("获取待发送通知失败: %v\n", err)
		return
	}

	if len(notifications) == 0 {
		return
	}

	// 按业务ID分组，避免单个业务超出限制影响其他业务
	bizGroups := groupByBizID(notifications)

	// 对每个业务分组单独处理
	for _, group := range bizGroups {
		_, err := s.sender.Send(ctx, group)
		if err != nil {
			fmt.Printf("发送通知组失败: %v\n", err)
			// 继续处理其他组
		}
	}
}

func (s *scheduler) getNotifications(ctx context.Context, stime int64, limit int) ([]notificationsvc.Notification, error) {
	// 从Notification模块中获取可以发送
	// where status = PENDING and SSTime <= stime limit  GROUP BY bizID
	// stime = time.Now() // 当前时间必须大于等于开始时间
	// notifications, err := s.notificationSvc.ListByStatus(ctx, stime, s.limit)
	panic("implement me" + fmt.Sprintf("%v %v, %v", ctx, stime, limit))
}

// groupByBizID 按业务ID分组通知
func groupByBizID(notifications []notificationsvc.Notification) [][]notificationsvc.Notification {
	bizMap := make(map[int64][]notificationsvc.Notification)
	// 按业务ID归类
	for i := range notifications {
		bizMap[notifications[i].BizID] = append(bizMap[notifications[i].BizID], notifications[i])
	}
	// 转换为分组列表
	result := make([][]notificationsvc.Notification, 0, len(bizMap))
	for _, group := range bizMap {
		result = append(result, group)
	}
	return result
}
