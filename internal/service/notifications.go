package service

import (
	"strings"

	"github.com/wuyaocheng/bktrader/internal/domain"
)

func (p *Platform) ListNotifications(includeAcked bool) ([]domain.PlatformNotification, error) {
	alerts, err := p.ListAlerts()
	if err != nil {
		return nil, err
	}
	acks, err := p.store.ListNotificationAcks()
	if err != nil {
		return nil, err
	}
	ackByID := make(map[string]domain.NotificationAck, len(acks))
	for _, ack := range acks {
		ackByID[ack.ID] = ack
	}

	items := make([]domain.PlatformNotification, 0, len(alerts))
	for _, alert := range alerts {
		notification := domain.PlatformNotification{
			ID:        alert.ID,
			Status:    "active",
			Alert:     alert,
			Metadata:  map[string]any{"scope": alert.Scope, "level": alert.Level},
			UpdatedAt: alert.EventTime,
		}
		if ack, ok := ackByID[alert.ID]; ok {
			ackedAt := ack.AckedAt
			notification.Status = "acked"
			notification.AckedAt = &ackedAt
			if ack.UpdatedAt.After(notification.UpdatedAt) {
				notification.UpdatedAt = ack.UpdatedAt
			}
		}
		if !includeAcked && notification.Status == "acked" {
			continue
		}
		items = append(items, notification)
	}
	return items, nil
}

func (p *Platform) AckNotification(notificationID string) (domain.NotificationAck, error) {
	return p.store.UpsertNotificationAck(strings.TrimSpace(notificationID))
}

func (p *Platform) UnackNotification(notificationID string) error {
	return p.store.DeleteNotificationAck(strings.TrimSpace(notificationID))
}
