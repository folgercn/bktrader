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
import { AccountRecord, ActiveSettingsModal, LiveSession, LiveSessionForm, RuntimePolicy, StrategyRecord } from "../types/domain";
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
  runtimePolicy: RuntimePolicy | null;
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
  runtimePolicy,
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
      className="max-w-[min(880px,calc(100vw-2rem))]"
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

      <div className="grid grid-cols-1 gap-4 lg:grid-cols-2">
        <div className="space-y-4">
          {/* Core Config Section */}
          <ModalGroup>
            <ModalSectionHeader 
              icon={Layers} 
              title="核心配置" 
              description="设置账户、策略及基本参数" 
            />
            <ModalFormGrid columns="wide">
              <ModalField label="会话别名">
                <ModalInput
                  placeholder="例如：趋势策略-2024 (可选)"
                  value={liveSessionForm.alias}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, alias: event.target.value }))}
                />
              </ModalField>
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
                  { value: "15m", label: "15m" },
                  { value: "30m", label: "30m" },
                  { value: "4h", label: "4h" },
                  { value: "1d", label: "1d" },
                ]}
              />
              <LiveSelectField
                label="数据源"
                value={liveSessionForm.executionDataSource}
                onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, executionDataSource: value }))}
                options={[
                  { value: "tick", label: "Tick" },
                  { value: "1min", label: "1 min" },
                ]}
              />
              <ModalField label="交易对">
                <ModalInput
                  placeholder="BTCUSDT"
                  value={liveSessionForm.symbol}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
                />
              </ModalField>
              <LiveSelectField
                label="数量模式"
                value={liveSessionForm.positionSizingMode}
                onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, positionSizingMode: value }))}
                options={[
                  { value: "fixed_quantity", label: "自定义固定数量" },
                  { value: "reentry_size_schedule", label: "按 Reentry Schedule" },
                ]}
              />
              <ModalField label={liveSessionForm.positionSizingMode === "reentry_size_schedule" ? "固定回退数量" : "固定下单量"}>
                <ModalInput
                  placeholder="0.00"
                  value={liveSessionForm.defaultOrderQuantity}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, defaultOrderQuantity: event.target.value }))}
                />
              </ModalField>
              {liveSessionForm.positionSizingMode === "reentry_size_schedule" && (
                <>
                  <ModalField label="首笔 Reentry 比例">
                    <ModalInput
                      placeholder="0.20"
                      value={liveSessionForm.reentrySizeScheduleFirst}
                      onChange={(event) => setLiveSessionForm((current) => ({ ...current, reentrySizeScheduleFirst: event.target.value }))}
                    />
                  </ModalField>
                  <ModalField label="次笔 Reentry 比例">
                    <ModalInput
                      placeholder="0.10"
                      value={liveSessionForm.reentrySizeScheduleSecond}
                      onChange={(event) => setLiveSessionForm((current) => ({ ...current, reentrySizeScheduleSecond: event.target.value }))}
                    />
                  </ModalField>
                  <div className="md:col-span-3 rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] px-4 py-3 text-[11px] font-semibold leading-relaxed text-[var(--bk-text-muted)]">
                    Reentry schedule 使用账户可用余额按比例换算下单量，例如 0.20 / 0.10 表示同一 signal bar 内第 1 笔真实 reentry 使用 20%，第 2 笔使用 10%；SL/PT 仍按当前持仓数量 reduceOnly 平仓。
                  </div>
                </>
              )}
            </ModalFormGrid>
          </ModalGroup>

          {/* Exit Strategy Section */}
          <ModalGroup>
            <ModalSectionHeader 
              icon={Target} 
              title="出场策略 (PT/SL)" 
              description="配置止盈(Take Profit)与止损(Stop Loss)执行" 
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
                  { value: "GTC", label: "GTC" },
                  { value: "GTX", label: "GTX (Post Only)" },
                  { value: "IOC", label: "IOC" },
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
              <ModalField label="止损最大价差">
                <ModalInput
                  placeholder="BPS"
                  value={liveSessionForm.executionSLExitMaxSpreadBps}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, executionSLExitMaxSpreadBps: event.target.value }))}
                />
              </ModalField>
            </ModalFormGrid>
          </ModalGroup>
        </div>

        <div className="space-y-4">
          {/* Entry Execution Section */}
          <ModalGroup>
            <ModalSectionHeader 
              icon={Zap} 
              title="进场执行" 
              description="定义开仓委托及滑点保护" 
            />
            <ModalFormGrid>
              <LiveSelectField
                label="订单类型"
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
                  { value: "limit-maker", label: "Limit Maker" },
                  { value: "", label: "Wait / Skip" },
                ]}
              />
              <LiveSelectField
                label="超时备选"
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

          {/* Dispatch & Risk Section */}
          <ModalGroup>
            <ModalSectionHeader 
              icon={ShieldAlert} 
              title="分发与风控" 
              description="控制订单下发与频率" 
            />
            <ModalFormGrid>
              <LiveSelectField
                label="分发模式"
                value={liveSessionForm.dispatchMode}
                onValueChange={(value) => setLiveSessionForm((current) => ({ ...current, dispatchMode: value }))}
                options={[
                  { value: "manual-review", label: "Manual Review" },
                  { value: "auto-dispatch", label: "Auto Dispatch" },
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

          {/* Freshness Overrides Section */}
          <ModalGroup>
            <ModalSectionHeader 
              icon={ShieldAlert} 
              title="新鲜度覆盖" 
              description="（可选）留空则使用全局默认" 
            />
            <ModalFormGrid columns="wide">
              <ModalField label="信号 (秒)">
                <ModalInput
                  placeholder={`${runtimePolicy?.signalBarFreshnessSeconds ?? "--"}`}
                  value={liveSessionForm.freshnessOverrideSignalBarFreshnessSeconds ?? ""}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, freshnessOverrideSignalBarFreshnessSeconds: event.target.value }))}
                />
              </ModalField>
              <ModalField label="成交 (秒)">
                <ModalInput
                  placeholder={`${runtimePolicy?.tradeTickFreshnessSeconds ?? "--"}`}
                  value={liveSessionForm.freshnessOverrideTradeTickFreshnessSeconds ?? ""}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, freshnessOverrideTradeTickFreshnessSeconds: event.target.value }))}
                />
              </ModalField>
              <ModalField label="盘口 (秒)">
                <ModalInput
                  placeholder={`${runtimePolicy?.orderBookFreshnessSeconds ?? "--"}`}
                  value={liveSessionForm.freshnessOverrideOrderBookFreshnessSeconds ?? ""}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, freshnessOverrideOrderBookFreshnessSeconds: event.target.value }))}
                />
              </ModalField>
              <ModalField label="运行时静默">
                <ModalInput
                  placeholder={`${runtimePolicy?.runtimeQuietSeconds ?? "--"}`}
                  value={liveSessionForm.freshnessOverrideRuntimeQuietSeconds ?? ""}
                  onChange={(event) => setLiveSessionForm((current) => ({ ...current, freshnessOverrideRuntimeQuietSeconds: event.target.value }))}
                />
              </ModalField>
            </ModalFormGrid>
          </ModalGroup>
        </div>
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
