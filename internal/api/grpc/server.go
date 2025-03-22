package grpc

import (
	"context"

	notificationv1 "gitee.com/flycash/notification-platform/api/proto/gen/notification/v1"
)

type Server struct {
	notificationv1.UnimplementedNotificationServiceServer
}

func (s *Server) SendNotification(ctx context.Context, request *notificationv1.SendNotificationRequest) (*notificationv1.SendNotificationResponse, error) {
	return &notificationv1.SendNotificationResponse{}, nil
}

func (s *Server) SendNotificationAsync(ctx context.Context, request *notificationv1.SendNotificationAsyncRequest) (*notificationv1.SendNotificationAsyncResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (s *Server) BatchSendNotifications(ctx context.Context, request *notificationv1.BatchSendNotificationsRequest) (*notificationv1.BatchSendNotificationsResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (s *Server) BatchSendNotificationsAsync(ctx context.Context, request *notificationv1.BatchSendNotificationsAsyncRequest) (*notificationv1.BatchSendNotificationsAsyncResponse, error) {
	// TODO implement me
	panic("implement me")
}

func (s *Server) QueryNotification(ctx context.Context, request *notificationv1.QueryNotificationRequest) (*notificationv1.QueryNotificationResponse, error) {
	// TODO implement me
	panic("implement me")
}
