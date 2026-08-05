package biz

import (
	"fmt"

	"github.com/leafney/filedock/internal/dto"
	"github.com/leafney/filedock/internal/service"
)

type NotificationBiz struct {
	notifications *service.NotificationSvc
}

func NewNotificationBiz(notifications *service.NotificationSvc) (*NotificationBiz, error) {
	if notifications == nil {
		return nil, fmt.Errorf("notification service is required")
	}
	return &NotificationBiz{notifications: notifications}, nil
}

func (b *NotificationBiz) List(userID, cursor string, limit int) (dto.NotificationPageDTO, error) {
	page, err := b.notifications.List(userID, cursor, limit)
	if err != nil {
		return dto.NotificationPageDTO{}, err
	}
	items := make([]dto.NotificationItemDTO, 0, len(page.Items))
	for _, item := range page.Items {
		items = append(items, dto.NotificationItemDTO{
			Key:                    item.Key,
			Type:                   item.Type,
			RoomCode:               item.RoomCode,
			RoomTitle:              item.RoomTitle,
			RequestID:              item.RequestID,
			ActorUserID:            item.ActorUserID,
			ActorDisplayName:       item.ActorDisplayName,
			CreatedAt:              item.CreatedAt,
			ExpiresAt:              item.ExpiresAt,
			ConversationID:         item.ConversationID,
			PeerUserID:             item.PeerUserID,
			PeerDisplayName:        item.PeerDisplayName,
			LatestMessageText:      item.LatestMessageText,
			UnreadCount:            item.UnreadCount,
			LatestMessageAt:        item.LatestMessageAt,
			CounterpartUserID:      item.CounterpartUserID,
			CounterpartDisplayName: item.CounterpartDisplayName,
			LatestFileName:         item.LatestFileName,
			FileCount:              item.FileCount,
			LatestFileEventAt:      item.LatestFileEventAt,
			ReadToken:              item.ReadToken,
		})
	}
	return dto.NotificationPageDTO{Items: items, TotalCount: page.TotalCount, NextCursor: page.NextCursor}, nil
}

func (b *NotificationBiz) MarkRead(userID string, request dto.MarkNotificationReadRequest) (dto.MarkNotificationReadDTO, error) {
	updated, err := b.notifications.MarkFileResultsRead(userID, request.Key, request.ReadToken)
	if err != nil {
		return dto.MarkNotificationReadDTO{}, err
	}
	return dto.MarkNotificationReadDTO{UpdatedCount: updated}, nil
}
