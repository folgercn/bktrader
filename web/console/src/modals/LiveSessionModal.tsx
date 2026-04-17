import React from "react";

import { Button } from "../components/ui/button";
import { Input } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";
import { AccountRecord, ActiveSettingsModal, LiveSession, LiveSessionForm, StrategyRecord } from "../types/domain";
import { strategyLabel } from "../utils/derivation";
import {
  ModalActions,
  ModalCheckboxField,
  ModalField,
  ModalFormGrid,
  ModalMetaItem,
  ModalMetaStrip,
  ModalNotice,
  SettingsModalFrame,
} from "./modal-frame";

interface LiveSessionModalProps {
  activeSettingsModal: ActiveSettingsModal;
  setActiveSettingsModal: (modal: ActiveSettingsModal) => void;
  liveSessionError: string | null;
  liveSessionNotice: string | null;
  liveAccounts: AccountRecord[];
  liveSessionForm: LiveSessionForm;
  setLiveSessionForm: (valOrUpdater: LiveSessionForm | ((prev: LiveSessionForm) => LiveSessionForm)) => void;
  strategies: StrategyRecord[];
  validLiveSessions: LiveSession[];
  editingLiveSessionId: string | null;
  strategyOptions: Array<{ value: string; label: string }>;
  liveSessionCreateAction: boolean;
  liveSessionLaunchAction: boolean;
  liveSessionAction: string | null;
  saveLiveSession: () => Promise<LiveSession | null>;
  createAndStartLiveSession: () => Promise<void>;
  setLiveSessionLaunchAction: (val: boolean) => void;
  setLiveSessionAction: (val: string | null) => void;
  setLiveSessionError: (val: string | null) => void;
  loadDashboard: () => Promise<void>;
  setError: (val: string | null) => void;
  fetchJSON: <T>(path: string, init?: RequestInit) => Promise<T>;
}

const inputClassName =
  "h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3";
const EMPTY_SELECT_VALUE = "__empty__";

function LiveSelectField({
  label,
  value,
  onValueChange,
  options,
}: {
  label: string;
  value: string;
  onValueChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
}) {
  const normalizedValue = value === "" ? EMPTY_SELECT_VALUE : value;

  return (
    <ModalField label={label}>
      <Select
        value={normalizedValue}
        onValueChange={(nextValue) => onValueChange(nextValue === EMPTY_SELECT_VALUE ? "" : nextValue ?? "")}
      >
        <SelectTrigger tone="bento" className="h-10 w-full rounded-xl">
          <SelectValue placeholder={label} />
        </SelectTrigger>
        <SelectContent tone="bento" className="rounded-xl">
          {options.map((option) => (
            <SelectItem
              key={`${label}-${option.value === "" ? EMPTY_SELECT_VALUE : option.value}`}
              value={option.value === "" ? EMPTY_SELECT_VALUE : option.value}
            >
              {option.label}
            </SelectItem>
          ))}
        </SelectContent>
      </Select>
    </ModalField>
  );
}

export function LiveSessionModal({
  activeSettingsModal,
  setActiveSettingsModal,
  liveSessionError,
  liveSessionNotice,
  liveAccounts,
  liveSessionForm,
  setLiveSessionForm,
  strategies,
  validLiveSessions,
  editingLiveSessionId,
  strategyOptions,
  liveSessionCreateAction,
  liveSessionLaunchAction,
  liveSessionAction,
  saveLiveSession,
  createAndStartLiveSession,
  setLiveSessionLaunchAction,
  setLiveSessionAction,
  setLiveSessionError,
  loadDashboard,
  setError,
  fetchJSON,
}: LiveSessionModalProps) {
  const open = activeSettingsModal === "live-session";

  return (
    <SettingsModalFrame
      open={open}
      onOpenChange={(nextOpen) => !nextOpen && setActiveSettingsModal(null)}
      kicker="实盘会话"
      title="配置实盘会话"
      description="账户、策略、执行和分发参数都在这里统一编辑，保持现有的人工审查边界。"
      className="max-w-[min(1080px,calc(100vw-2rem))]"
    >
      {liveSessionError ? <ModalNotice tone="error">{liveSessionError}</ModalNotice> : null}
      {liveSessionNotice ? <ModalNotice tone="success">{liveSessionNotice}</ModalNotice> : null}

      <ModalMetaStrip>
        <ModalMetaItem
          label="账户"
          value={liveAccounts.find((account) => account.id === liveSessionForm.accountId)?.name ?? liveSessionForm.accountId ?? "--"}
        />
        <ModalMetaItem
          label="策略"
          value={strategyLabel(strategies.find((strategy) => strategy.id === liveSessionForm.strategyId))}
        />
        <ModalMetaItem
          label="有效会话"
          value={String(validLiveSessions.filter((session) => session.accountId === liveSessionForm.accountId).length)}
        />
        {editingLiveSessionId ? <ModalMetaItem label="编辑" value={editingLiveSessionId} /> : null}
      </ModalMetaStrip>

      <ModalFormGrid columns="wide">
        <LiveSelectField
          label="实盘账户"
          value={liveSessionForm.accountId}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, accountId: value }))}
          options={liveAccounts.map((account) => ({
            value: account.id,
            label: `${account.name} (${account.status})`,
          }))}
        />
        <LiveSelectField
          label="绑定策略"
          value={liveSessionForm.strategyId}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, strategyId: value }))}
          options={strategyOptions}
        />
        <LiveSelectField
          label="信号周期"
          value={liveSessionForm.signalTimeframe}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, signalTimeframe: value }))}
          options={[
            { value: "5m", label: "5m" },
            { value: "4h", label: "4h" },
            { value: "1d", label: "1d" },
          ]}
        />

        <LiveSelectField
          label="执行数据源"
          value={liveSessionForm.executionDataSource}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionDataSource: value }))}
          options={[
            { value: "tick", label: "tick" },
            { value: "1min", label: "1min" },
          ]}
        />
        <ModalField label="交易对 (Symbol)">
          <Input
            value={liveSessionForm.symbol}
            onChange={(event) => setLiveSessionForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
            className={inputClassName}
          />
        </ModalField>
        <ModalField label="默认下单量">
          <Input
            value={liveSessionForm.defaultOrderQuantity}
            onChange={(event) => setLiveSessionForm((current) => ({ ...current, defaultOrderQuantity: event.target.value }))}
            className={inputClassName}
          />
        </ModalField>

        <LiveSelectField
          label="进场订单类型"
          value={liveSessionForm.executionEntryOrderType}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionEntryOrderType: value }))}
          options={[
            { value: "MARKET", label: "MARKET" },
            { value: "LIMIT", label: "LIMIT" },
          ]}
        />
        <ModalField label="进场最大价差 (bps)">
          <Input
            value={liveSessionForm.executionEntryMaxSpreadBps}
            onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryMaxSpreadBps: event.target.value }))}
            className={inputClassName}
          />
        </ModalField>
        <LiveSelectField
          label="宽价差处理模式"
          value={liveSessionForm.executionEntryWideSpreadMode}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionEntryWideSpreadMode: value }))}
          options={[
            { value: "limit-maker", label: "limit-maker" },
            { value: "", label: "wait" },
          ]}
        />

        <LiveSelectField
          label="进场超时备选"
          value={liveSessionForm.executionEntryTimeoutFallbackOrderType}
          onValueChange={(value) =>
            setLiveSessionForm((current) => ({ ...current, executionEntryTimeoutFallbackOrderType: value }))
          }
          options={[
            { value: "MARKET", label: "MARKET" },
            { value: "LIMIT", label: "LIMIT" },
            { value: "", label: "disabled" },
          ]}
        />
        <LiveSelectField
          label="止盈订单类型"
          value={liveSessionForm.executionPTExitOrderType}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionPTExitOrderType: value }))}
          options={[
            { value: "LIMIT", label: "LIMIT" },
            { value: "MARKET", label: "MARKET" },
          ]}
        />
        <LiveSelectField
          label="止盈 TIF"
          value={liveSessionForm.executionPTExitTimeInForce}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionPTExitTimeInForce: value }))}
          options={[
            { value: "GTX", label: "GTX" },
            { value: "GTC", label: "GTC" },
            { value: "IOC", label: "IOC" },
          ]}
        />

        <ModalCheckboxField
          label="止盈只做 Maker"
          checked={liveSessionForm.executionPTExitPostOnly}
          onChange={(checked) => setLiveSessionForm((current) => ({ ...current, executionPTExitPostOnly: checked }))}
        />
        <LiveSelectField
          label="止盈超时备选"
          value={liveSessionForm.executionPTExitTimeoutFallbackOrderType}
          onValueChange={(value) =>
            setLiveSessionForm((current) => ({ ...current, executionPTExitTimeoutFallbackOrderType: value }))
          }
          options={[
            { value: "MARKET", label: "MARKET" },
            { value: "LIMIT", label: "LIMIT" },
            { value: "", label: "disabled" },
          ]}
        />
        <LiveSelectField
          label="止损订单类型"
          value={liveSessionForm.executionSLExitOrderType}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionSLExitOrderType: value }))}
          options={[
            { value: "MARKET", label: "MARKET" },
            { value: "LIMIT", label: "LIMIT" },
          ]}
        />

        <ModalField label="止损最大价差 (bps)">
          <Input
            value={liveSessionForm.executionSLExitMaxSpreadBps}
            onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionSLExitMaxSpreadBps: event.target.value }))}
            className={inputClassName}
          />
        </ModalField>
        <LiveSelectField
          label="分发模式"
          value={liveSessionForm.dispatchMode}
          onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, dispatchMode: value }))}
          options={[
            { value: "manual-review", label: "manual-review" },
            { value: "auto-dispatch", label: "auto-dispatch" },
          ]}
        />
        <ModalField label="分发冷却 (秒)">
          <Input
            value={liveSessionForm.dispatchCooldownSeconds}
            onChange={(event) => setLiveSessionForm((current) => ({ ...current, dispatchCooldownSeconds: event.target.value }))}
            className={inputClassName}
          />
        </ModalField>
      </ModalFormGrid>

      <ModalActions>
        <Button
          variant="bento-outline"
          className="h-10 rounded-xl px-4 font-bold"
          disabled={liveSessionCreateAction || liveSessionLaunchAction || !liveSessionForm.accountId || !liveSessionForm.strategyId}
          onClick={saveLiveSession}
        >
          {liveSessionCreateAction
            ? editingLiveSessionId
              ? "保存中..."
              : "创建中..."
            : editingLiveSessionId
              ? "保存实盘会话"
              : "创建实盘会话"}
        </Button>
        <Button
          variant="bento"
          className="h-10 rounded-xl px-5 font-black"
          disabled={
            liveSessionCreateAction ||
            liveSessionLaunchAction ||
            liveSessionAction !== null ||
            !liveSessionForm.accountId ||
            !liveSessionForm.strategyId
          }
          onClick={async () => {
            if (!editingLiveSessionId) {
              await createAndStartLiveSession();
              return;
            }
            setLiveSessionLaunchAction(true);
            try {
              const updated = await saveLiveSession();
              if (!updated?.id) {
                return;
              }
              setLiveSessionAction(`${updated.id}:start`);
              await fetchJSON(`/api/v1/live/sessions/${updated.id}/start`, { method: "POST" });
              await loadDashboard();
              setError(null);
            } catch (err) {
              setLiveSessionError(err instanceof Error ? err.message : "Failed to save and start live session");
            } finally {
              setLiveSessionAction(null);
              setLiveSessionLaunchAction(false);
            }
          }}
        >
          {liveSessionLaunchAction
            ? editingLiveSessionId
              ? "保存中..."
              : "启动中..."
            : editingLiveSessionId
              ? "保存并启动"
              : "创建并启动"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
