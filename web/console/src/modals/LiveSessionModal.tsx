import React from "react";
import {
  Layers,
  Zap,
  Target,
  ShieldAlert,
} from "lucide-react";

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
  ModalGroup,
  ModalInput,
  ModalMetaItem,
  ModalMetaStrip,
  ModalNotice,
  ModalSectionHeader,
  ModalSelect,
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
  return (
    <ModalField label={label}>
      <ModalSelect
        value={value}
        onValueChange={onValueChange}
        options={options}
        placeholder={label}
      />
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

  const currentAccount = liveAccounts.find((account) => account.id === liveSessionForm.accountId);
  const currentStrategy = strategies.find((strategy) => strategy.id === liveSessionForm.strategyId);

  return (
    <SettingsModalFrame
      open={open}
      onOpenChange={(nextOpen) => !nextOpen && setActiveSettingsModal(null)}
      kicker="Live Session"
      title="配置实盘会话"
      description="在此统一管理账户、策略执行和分发参数。系统会严格遵守人工审核边界以确保交易安全。"
      className="max-w-[min(810px,calc(100vw-2rem))]"

    >
      {liveSessionError ? <ModalNotice tone="error">{liveSessionError}</ModalNotice> : null}
      {liveSessionNotice ? <ModalNotice tone="success">{liveSessionNotice}</ModalNotice> : null}

      <ModalMetaStrip>
        <ModalMetaItem
          label="当前账户"
          value={currentAccount?.name ?? liveSessionForm.accountId ?? "--"}
        />
        <ModalMetaItem
          label="绑定策略"
          value={strategyLabel(currentStrategy) || "未选择"}
        />
        <ModalMetaItem
          label="有效会话"
          value={validLiveSessions.filter((s) => s.accountId === liveSessionForm.accountId).length}
        />
        <ModalMetaItem
          label="数据源"
          value={liveSessionForm.executionDataSource || "--"}
        />
        {editingLiveSessionId && (
          <ModalMetaItem label="编辑中" value={editingLiveSessionId} />
        )}

      </ModalMetaStrip>

      <div className="grid grid-cols-1 gap-6 lg:grid-cols-2">
        {/* Core Config Section */}
        <ModalGroup>
          <ModalSectionHeader 
            icon={Layers} 
            title="核心配置" 
            description="设置交易账户、策略及基本交易参数" 
          />
          <ModalFormGrid>

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
                { value: "tick", label: "Tick (High Precision)" },
                { value: "1min", label: "1 min (Optimized)" },
              ]}
            />
            <ModalField label="交易对 (Symbol)">
              <ModalInput
                placeholder="例如: BTCUSDT"
                value={liveSessionForm.symbol}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
              />
            </ModalField>
            <ModalField label="默认下单量">
              <ModalInput
                placeholder="0.00"
                value={liveSessionForm.defaultOrderQuantity}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, defaultOrderQuantity: event.target.value }))}
              />

            </ModalField>
          </ModalFormGrid>
        </ModalGroup>

        {/* Entry Execution Section */}
        <ModalGroup>
          <ModalSectionHeader 
            icon={Zap} 
            title="进场执行" 
            description="定义开仓时的委托类型及滑点保护机制" 
          />
          <ModalFormGrid>

            <LiveSelectField
              label="进场订单类型"
              value={liveSessionForm.executionEntryOrderType}
              onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionEntryOrderType: value }))}
              options={[
                { value: "MARKET", label: "MARKET" },
                { value: "LIMIT", label: "LIMIT" },
              ]}
            />
            <ModalField label="最大价差 (bps)">
              <ModalInput
                placeholder="BPS"
                value={liveSessionForm.executionEntryMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionEntryMaxSpreadBps: event.target.value }))}
              />
            </ModalField>

            <LiveSelectField
              label="宽价差处理"
              value={liveSessionForm.executionEntryWideSpreadMode}
              onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionEntryWideSpreadMode: value }))}
              options={[
                { value: "limit-maker", label: "Limit Maker (Post-Only)" },
                { value: "", label: "Wait / Skip" },
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
                { value: "", label: "Disabled" },
              ]}
            />
          </ModalFormGrid>
        </ModalGroup>

        {/* Exit Strategy Section */}
        <ModalGroup>
          <ModalSectionHeader 
            icon={Target} 
            title="出场策略 (PT/SL)" 
            description="配置止盈(Take Profit)与止损(Stop Loss)的执行细节" 
          />
          <ModalFormGrid>

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
                { value: "GTC", label: "GTC (Good Till Cancel)" },
                { value: "GTX", label: "GTX (Post Only)" },
                { value: "IOC", label: "IOC (Immediate or Cancel)" },
              ]}
            />
            <ModalCheckboxField
              label="止盈仅做 Maker"
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
                { value: "", label: "Disabled" },
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
              <ModalInput
                placeholder="BPS"
                value={liveSessionForm.executionSLExitMaxSpreadBps}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionSLExitMaxSpreadBps: event.target.value }))}
              />
            </ModalField>

          </ModalFormGrid>
        </ModalGroup>

        {/* Dispatch & Risk Section */}
        <ModalGroup>
          <ModalSectionHeader 
            icon={ShieldAlert} 
            title="分发与风控" 
            description="控制订单下发模式及执行频率限制" 
          />
          <ModalFormGrid columns="wide">
            <LiveSelectField
              label="分发模式"
              value={liveSessionForm.dispatchMode}
              onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, dispatchMode: value }))}
              options={[
                { value: "manual-review", label: "Manual Review (Safety)" },
                { value: "auto-dispatch", label: "Auto Dispatch (Aggressive)" },
              ]}
            />
            <ModalField label="分发冷却 (秒)">
              <ModalInput
                placeholder="30"
                value={liveSessionForm.dispatchCooldownSeconds}
                onChange={(event) => setLiveSessionForm((current) => ({ ...current, dispatchCooldownSeconds: event.target.value }))}
              />
            </ModalField>

          </ModalFormGrid>
        </ModalGroup>
      </div>

      <ModalActions>
        <Button
          variant="bento-outline"
          className="h-11 rounded-2xl px-6 text-sm font-bold shadow-sm transition-all hover:bg-[var(--bk-surface-overlay)]"
          disabled={liveSessionCreateAction || liveSessionLaunchAction || !liveSessionForm.accountId || !liveSessionForm.strategyId}
          onClick={saveLiveSession}
        >
          {liveSessionCreateAction
            ? editingLiveSessionId ? "正在保存..." : "正在创建..."
            : editingLiveSessionId ? "更新会话配置" : "仅保存配置"}
        </Button>
        <Button
          variant="bento"
          className="h-11 rounded-2xl bg-[var(--bk-status-success)] px-8 text-sm font-black text-white shadow-md transition-all hover:brightness-110 active:scale-[0.98]"

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
            ? editingLiveSessionId ? "保存并启动中..." : "启动中..."
            : editingLiveSessionId ? "保存并启动会话" : "立即创建并启动"}
        </Button>
      </ModalActions>
    </SettingsModalFrame>
  );
}
