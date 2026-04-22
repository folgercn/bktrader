import React from "react";

import { Badge } from "../components/ui/badge";
import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import { ActiveSettingsModal, TelegramConfig, TelegramForm } from "../types/domain";
import {
  ModalActions,
  ModalCheckboxField,
  ModalField,
  ModalFormGrid,
  ModalMetaItem,
  ModalMetaStrip,
  SettingsModalFrame,
} from "./modal-frame";

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
  sendTelegramTest,
}: TelegramModalProps) {
  const open = activeSettingsModal === "telegram";

  return (
    <SettingsModalFrame
      open={open}
      onOpenChange={(nextOpen) => !nextOpen && setActiveSettingsModal(null)}
      kicker="Telegram"
      title="Telegram 通知配置"
      description="机器人令牌、聊天目标和发送等级都统一收在这里，保留现有通知行为。"
      className="max-w-[min(720px,calc(100vw-2rem))]"
    >
      <ModalMetaStrip>
        <Badge variant={telegramConfig?.enabled ? "success" : "neutral"}>
          {telegramConfig?.enabled ? "active" : "disabled"}
        </Badge>
        <ModalMetaItem label="Token" value={telegramConfig?.maskedBotToken || "no-token"} />
        <ModalMetaItem label="Chat" value={telegramConfig?.chatId || "no-chat"} />
        {telegramAction === null && telegramConfig ? (
          <span className="text-[10px] font-mono text-[var(--bk-status-success)] animate-pulse">
            已同步最新配置
          </span>
        ) : null}
      </ModalMetaStrip>

      <ModalFormGrid>
        <div className="md:col-span-2">
          <ModalCheckboxField
            label="Enabled"
            checked={telegramForm.enabled}
            onChange={(checked) => setTelegramForm((current) => ({ ...current, enabled: checked }))}
          />
        </div>
        <ModalField label="Chat ID">
          <Input
            value={telegramForm.chatId}
            onChange={(event) => setTelegramForm((current) => ({ ...current, chatId: event.target.value }))}
            placeholder="123456789"
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
        <ModalField label="Bot Token">
          <Input
            value={telegramForm.botToken}
            onChange={(event) => setTelegramForm((current) => ({ ...current, botToken: event.target.value }))}
            placeholder={telegramConfig?.hasBotToken ? "leave blank to keep current token" : "123456:ABCDEF..."}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
        <ModalField label="Send Levels" wide>
          <Input
            value={telegramForm.sendLevels}
            onChange={(event) => setTelegramForm((current) => ({ ...current, sendLevels: event.target.value }))}
            placeholder="critical,warning"
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
        <ModalCheckboxField
          label="开平事件提醒"
          checked={telegramForm.tradeEventsEnabled}
          onChange={(checked) => setTelegramForm((current) => ({ ...current, tradeEventsEnabled: checked }))}
        />
        <ModalCheckboxField
          label="持仓定时播报"
          checked={telegramForm.positionReportEnabled}
          onChange={(checked) => setTelegramForm((current) => ({ ...current, positionReportEnabled: checked }))}
        />
        <ModalField label="持仓播报间隔">
          <Input
            type="number"
            min={5}
            max={1440}
            step={5}
            value={telegramForm.positionReportIntervalMinutes}
            onChange={(event) => setTelegramForm((current) => ({ ...current, positionReportIntervalMinutes: event.target.value }))}
            className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3"
          />
        </ModalField>
      </ModalFormGrid>

      <ModalActions>
        <Button
          variant="bento-outline"
          className="h-10 rounded-xl px-4 font-bold"
          disabled={telegramAction !== null || !telegramConfig?.enabled || !telegramConfig?.hasBotToken || !telegramConfig?.chatId}
          onClick={sendTelegramTest}
        >
          {telegramAction === "test" ? "Sending..." : "Send Test Message"}
        </Button>
        <Button
          variant="bento"
          className="h-10 rounded-xl px-5 font-black"
          disabled={telegramAction !== null}
          onClick={saveTelegramConfig}
        >
          {telegramAction === "save-config" ? "Saving..." : "Save Telegram Config"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
