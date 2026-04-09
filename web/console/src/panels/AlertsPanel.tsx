import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { StatusPill } from '../components/ui/StatusPill';
import { PlatformAlert, PlatformNotification, TelegramConfig } from '../types/domain';
import { formatTime } from '../utils/format';
import { alertLevelTone, alertScopeTone, telegramDeliveryTone } from '../utils/derivation';

interface AlertsPanelProps {
  alerts: PlatformAlert[];
  jumpToAlert: (alert: PlatformAlert) => void;
  notifications: PlatformNotification[];
  notificationAction: string | null;
  telegramAction: string | null;
  acknowledgeNotification: (item: PlatformNotification, ack: boolean) => void;
  telegramConfig: TelegramConfig | null;
  sendNotificationToTelegram: (item: PlatformNotification) => void;
}

export function AlertsPanel({
  alerts,
  jumpToAlert,
  notifications,
  notificationAction,
  telegramAction,
  acknowledgeNotification,
  telegramConfig,
  sendNotificationToTelegram
}: AlertsPanelProps) {
  return (
    <>
      <section id="alerts" className="panel panel-alerts">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Alerts</p>
            <h3>统一运行告警</h3>
          </div>
          <div className="range-box">
            <span>{alerts.length} alerts</span>
            <span>{alerts.filter((item) => item.level === "critical").length} critical</span>
            <span>{alerts.filter((item) => item.level === "warning").length} warning</span>
          </div>
        </div>
        {alerts.length > 0 ? (
          <div className="alerts-grid">
            {alerts.map((alert) => (
              <article key={alert.id || `${alert.scope}-${alert.title}-${alert.detail}`} className="alert-card">
                <div className="alert-card-header">
                  <div>
                    <StatusPill tone={alertLevelTone(String(alert.level ?? "warning"))}>{String(alert.level ?? "warning")}</StatusPill>
                    <StatusPill tone={alertScopeTone(String(alert.scope ?? "live"))}>{String(alert.scope ?? "live")}</StatusPill>
                  </div>
                  <span className="alert-time">{formatTime(String(alert.eventTime ?? ""))}</span>
                </div>
                <h4>{alert.title}</h4>
                <p>{alert.detail}</p>
                <div className="alert-meta">
                  <span>{alert.accountName || alert.accountId || "--"}</span>
                  <span>{alert.strategyName || alert.strategyId || "--"}</span>
                  <span>{alert.runtimeSessionId || alert.paperSessionId || "--"}</span>
                </div>
                {alert.anchor ? (
                  <div className="inline-actions">
                    <button
                      type="button"
                      className="filter-chip"
                      onClick={() => jumpToAlert(alert)}
                    >
                      Open
                    </button>
                  </div>
                ) : null}
              </article>
            ))}
          </div>
        ) : (
          <div className="empty-state empty-state-compact">No active alerts</div>
        )}
      </section>

      <section id="notifications" className="panel panel-alerts">
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Inbox</p>
            <h3>平台内通知中心</h3>
          </div>
          <div className="range-box">
            <span>{notifications.filter((item) => item.status !== "acked").length} active</span>
            <span>{notifications.filter((item) => item.status === "acked").length} acked</span>
          </div>
        </div>
        {notifications.length > 0 ? (
          <div className="alerts-grid">
            {notifications.map((item) => {
              const alert = item.alert ?? ({
                id: item.id,
                scope: String(item.metadata?.scope ?? "live"),
                level: String(item.metadata?.level ?? "warning"),
                title: "Notification",
                detail: "missing alert payload",
                anchor: "notifications",
                eventTime: String(item.updatedAt ?? ""),
              } as PlatformAlert);
              return (
                <article key={item.id} className="alert-card">
                  <div className="alert-card-header">
                    <div>
                      <StatusPill tone={alertLevelTone(String(alert.level ?? "warning"))}>{String(alert.level ?? "warning")}</StatusPill>
                      <StatusPill tone={item.status === "acked" ? "neutral" : alertScopeTone(String(alert.scope ?? "live"))}>{item.status}</StatusPill>
                      <StatusPill tone={telegramDeliveryTone(item.metadata?.telegramStatus)}>
                        telegram {item.metadata?.telegramStatus ?? "pending"}
                      </StatusPill>
                    </div>
                    <span className="alert-time">{formatTime(String(item.updatedAt ?? alert.eventTime ?? ""))}</span>
                  </div>
                  <h4>{alert.title}</h4>
                  <p>{alert.detail}</p>
                  <div className="alert-meta">
                    <span>{alert.accountName || alert.accountId || "--"}</span>
                    <span>{alert.strategyName || alert.strategyId || "--"}</span>
                    <span>{alert.runtimeSessionId || alert.paperSessionId || "--"}</span>
                    <span>
                      {item.metadata?.telegramStatus === "sent" && item.metadata?.telegramSentAt
                        ? `sent ${formatTime(String(item.metadata.telegramSentAt))}`
                        : item.metadata?.telegramStatus === "failed" && item.metadata?.telegramAttemptedAt
                          ? `failed ${formatTime(String(item.metadata.telegramAttemptedAt))}`
                          : "telegram pending"}
                    </span>
                  </div>
                  {item.metadata?.telegramStatus === "failed" && item.metadata?.telegramLastError ? (
                    <div className="note-item note-item-alert note-item-alert-critical">
                      <strong>Telegram failed</strong> {String(item.metadata.telegramLastError)}
                    </div>
                  ) : null}
                  <div className="inline-actions">
                    <ActionButton
                      label={item.status === "acked" ? "Unack" : "Acknowledge"}
                      variant="ghost"
                      disabled={notificationAction !== null || telegramAction !== null}
                      onClick={() => acknowledgeNotification(item, item.status !== "acked")}
                    />
                    <ActionButton
                      label={
                        telegramAction === `send:${item.id}`
                          ? "Sending..."
                          : item.metadata?.telegramStatus === "failed"
                            ? "Retry Telegram"
                            : "Send Telegram"
                      }
                      variant="ghost"
                      disabled={
                        notificationAction !== null ||
                        telegramAction !== null ||
                        !telegramConfig?.enabled ||
                        !telegramConfig?.hasBotToken ||
                        !telegramConfig?.chatId ||
                        item.metadata?.telegramStatus === "sent"
                      }
                      onClick={() => sendNotificationToTelegram(item)}
                    />
                    <button
                      type="button"
                      className="filter-chip"
                      onClick={() => jumpToAlert(alert)}
                    >
                      Open
                    </button>
                  </div>
                </article>
              );
            })}
          </div>
        ) : (
          <div className="empty-state empty-state-compact">No notifications</div>
        )}
      </section>
    </>
  );
}
