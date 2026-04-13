import React from 'react';
import { ActionButton } from '../components/ui/ActionButton';
import { StatusPill } from '../components/ui/StatusPill';
import { TelegramConfig, TelegramForm, ActiveSettingsModal } from '../types/domain';

interface TelegramModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  telegramConfig: TelegramConfig | null;
  telegramForm: TelegramForm;
  setTelegramForm: (valOrUpdater: TelegramForm | ((prev: TelegramForm) => TelegramForm)) => void;
  telegramAction: string | null;
  saveTelegramConfig: () => void;
  sendTelegramTest: () => void;
}

export function TelegramModal({
  activeSettingsModal,
  setActiveSettingsModal,
  telegramConfig,
  telegramForm,
  setTelegramForm,
  telegramAction,
  saveTelegramConfig,
  sendTelegramTest
}: TelegramModalProps) {
  if (activeSettingsModal !== "telegram") return null;

  return (
    <div
      className="modal-overlay"
      onClick={() => setActiveSettingsModal(null)}
    >
      <div className="modal-panel" onClick={(event) => event.stopPropagation()}>
        <div className="panel-header">
          <div>
            <p className="panel-kicker">Telegram</p>
            <h3>Telegram 通知配置</h3>
          </div>
          <div className="flex flex-col items-end space-y-1">
            <button type="button" className="hero-menu-button" onClick={() => setActiveSettingsModal(null)}>
              关闭
            </button>
            {telegramAction === null && telegramConfig && (
              <span className="text-[10px] text-emerald-500 font-mono opacity-80 animate-pulse">
                已同步最新配置
              </span>
            )}
          </div>
        </div>
        <div className="range-box">
          <div className="flex items-center space-x-2">
            <StatusPill tone={telegramConfig?.enabled ? "ready" : "neutral"}>
              {telegramConfig?.enabled ? "active" : "disabled"}
            </StatusPill>
            <span className="text-zinc-500 text-xs">·</span>
            <span className="font-mono text-zinc-400">{telegramConfig?.maskedBotToken || "no-token"}</span>
            <span className="text-zinc-500 text-xs">·</span>
            <span className="font-mono text-zinc-400">ID: {telegramConfig?.chatId || "no-chat"}</span>
          </div>
        </div>
        <div className="backtest-form modal-form">
          <div className="form-grid">
            <label className="form-field form-field-checkbox">
              <span>Enabled</span>
              <input
                type="checkbox"
                checked={telegramForm.enabled}
                onChange={(event) => setTelegramForm((current) => ({ ...current, enabled: event.target.checked }))}
              />
            </label>
            <label className="form-field">
              <span>Chat ID</span>
              <input
                value={telegramForm.chatId}
                onChange={(event) => setTelegramForm((current) => ({ ...current, chatId: event.target.value }))}
                placeholder="123456789"
              />
            </label>
            <label className="form-field form-field-wide">
              <span>Bot Token</span>
              <input
                value={telegramForm.botToken}
                onChange={(event) => setTelegramForm((current) => ({ ...current, botToken: event.target.value }))}
                placeholder={telegramConfig?.hasBotToken ? "leave blank to keep current token" : "123456:ABCDEF..."}
              />
            </label>
            <label className="form-field form-field-wide">
              <span>Send Levels</span>
              <input
                value={telegramForm.sendLevels}
                onChange={(event) => setTelegramForm((current) => ({ ...current, sendLevels: event.target.value }))}
                placeholder="critical,warning"
              />
            </label>
          </div>
          <div className="backtest-actions inline-actions">
            <ActionButton
              label={telegramAction === "save-config" ? "Saving..." : "Save Telegram Config"}
              disabled={telegramAction !== null}
              onClick={saveTelegramConfig}
            />
            <ActionButton
              label={telegramAction === "test" ? "Sending..." : "Send Test Message"}
              variant="ghost"
              disabled={telegramAction !== null || !telegramConfig?.enabled || !telegramConfig?.hasBotToken || !telegramConfig?.chatId}
              onClick={sendTelegramTest}
            />
          </div>
        </div>
      </div>
    </div>
  );
}
