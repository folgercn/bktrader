import { useMemo } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { fetchJSON, API_BASE } from '../utils/api';
import { writeStoredAuthSession } from '../utils/auth';
import { 
  AccountRecord, LiveSession, StrategyRecord, BacktestRun, RuntimePolicy, 
  PlatformAlert, PlatformNotification, TelegramConfig, AuthSession,
  SignalBinding, SignalRuntimeSession, LiveNextAction, LaunchTemplate, LiveLaunchResult
} from '../types/domain';
import { strategyLabel, getRecord } from '../utils/derivation';

export function useTradingActions(loadDashboard: () => Promise<void>) {
  const setError = useUIStore(s => s.setError);
  const setLoading = useUIStore(s => s.setLoading);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const setLoginForm = useUIStore(s => s.setLoginForm);
  const setLoginAction = useUIStore(s => s.setLoginAction);
  const setSessionAction = useUIStore(s => s.setSessionAction);
  const setPaperCreateAction = useUIStore(s => s.setPaperCreateAction);
  const setPaperLaunchAction = useUIStore(s => s.setPaperLaunchAction);
  const setLiveCreateAction = useUIStore(s => s.setLiveCreateAction);
  const setLiveBindAction = useUIStore(s => s.setLiveBindAction);
  const setLiveSyncAction = useUIStore(s => s.setLiveSyncAction);
  const setLiveAccountSyncAction = useUIStore(s => s.setLiveAccountSyncAction);
  const setLiveFlowAction = useUIStore(s => s.setLiveFlowAction);
  const setLiveOrderAction = useUIStore(s => s.setLiveOrderAction);
  const setLiveSessionAction = useUIStore(s => s.setLiveSessionAction);
  const setLiveSessionCreateAction = useUIStore(s => s.setLiveSessionCreateAction);
  const setLiveSessionLaunchAction = useUIStore(s => s.setLiveSessionLaunchAction);
  const setLiveSessionDeleteAction = useUIStore(s => s.setLiveSessionDeleteAction);
  const setSignalBindingAction = useUIStore(s => s.setSignalBindingAction);
  const setSignalRuntimeAction = useUIStore(s => s.setSignalRuntimeAction);
  const setNotificationAction = useUIStore(s => s.setNotificationAction);
  const setTelegramAction = useUIStore(s => s.setTelegramAction);
  const setBacktestAction = useUIStore(s => s.setBacktestAction);
  const setRuntimePolicyAction = useUIStore(s => s.setRuntimePolicyAction);
  const setStrategyCreateAction = useUIStore(s => s.setStrategyCreateAction);
  const setStrategySaveAction = useUIStore(s => s.setStrategySaveAction);
  const setLiveAccountError = useUIStore(s => s.setLiveAccountError);
  const setLiveBindingError = useUIStore(s => s.setLiveBindingError);
  const setLiveSessionError = useUIStore(s => s.setLiveSessionError);
  const setLiveSessionNotice = useUIStore(s => s.setLiveSessionNotice);
  const setActiveSettingsModal = useUIStore(s => s.setActiveSettingsModal);
  const setSettingsMenuOpen = useUIStore(s => s.setSettingsMenuOpen);
  const setTelegramForm = useUIStore(s => s.setTelegramForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const setLiveAccountNotice = useUIStore(s => s.setLiveAccountNotice);
  const setLiveBindingNotice = useUIStore(s => s.setLiveBindingNotice);
  const setStrategySignalForm = useUIStore(s => s.setStrategySignalForm);
  const setSignalRuntimeForm = useUIStore(s => s.setSignalRuntimeForm);
  const setLiveAccountForm = useUIStore(s => s.setLiveAccountForm);
  const setLiveBindingForm = useUIStore(s => s.setLiveBindingForm);
  const setLiveSessionForm = useUIStore(s => s.setLiveSessionForm);
  const setLaunchingTemplate = useUIStore(s => s.setLaunchingTemplate);
  const setNotification = useUIStore(s => s.setNotification);
  const setPositionCloseAction = useUIStore(s => s.setPositionCloseAction);
  const launchingTemplate = useUIStore(s => s.launchingTemplate);
  
  const loginForm = useUIStore(s => s.loginForm);
  const strategyCreateForm = useUIStore(s => s.strategyCreateForm);
  const strategyEditorForm = useUIStore(s => s.strategyEditorForm);
  const liveAccountForm = useUIStore(s => s.liveAccountForm);
  const liveBindingForm = useUIStore(s => s.liveBindingForm);
  const liveOrderForm = useUIStore(s => s.liveOrderForm);
  const liveSessionForm = useUIStore(s => s.liveSessionForm);
  const strategySignalForm = useUIStore(s => s.strategySignalForm);
  const signalRuntimeForm = useUIStore(s => s.signalRuntimeForm);
  const runtimePolicyForm = useUIStore(s => s.runtimePolicyForm);
  const telegramForm = useUIStore(s => s.telegramForm);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const backtestForm = useUIStore(s => s.backtestForm);

  const strategies = useTradingStore(s => s.strategies);
  const accounts = useTradingStore(s => s.accounts);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const editingLiveSessionId = useTradingStore(s => s.editingLiveSessionId);
  const setEditingLiveSessionId = useTradingStore(s => s.setEditingLiveSessionId);
  const setSummaries = useTradingStore(s => s.setSummaries);
  const setAccounts = useTradingStore(s => s.setAccounts);
  const setOrders = useTradingStore(s => s.setOrders);
  const setFills = useTradingStore(s => s.setFills);
  const setPositions = useTradingStore(s => s.setPositions);
  const setSnapshots = useTradingStore(s => s.setSnapshots);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);
  const setSignalRuntimePlan = useTradingStore(s => s.setSignalRuntimePlan);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);
  const setRuntimePolicy = useTradingStore(s => s.setRuntimePolicy);
  const setTelegramConfig = useTradingStore(s => s.setTelegramConfig);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);

  const strategyIds = useMemo(() => new Set(strategies.map((s: StrategyRecord) => s.id)), [strategies]);
  const validLiveSessions = useMemo(
    () => liveSessions.filter((item: LiveSession) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  // --- Helper Helpers ---
  function normalizeStrategyId(candidate: string, fallback = "") {
    return strategyIds.has(candidate) ? candidate : fallback;
  }

  function normalizeLivePositionSizingMode(candidate: unknown, schedule: unknown) {
    const value = String(candidate ?? "").trim().toLowerCase().replace(/-/g, "_");
    if (value === "reentry_size_schedule" || value === "reentry_schedule" || value === "schedule") {
      return "reentry_size_schedule";
    }
    if (value === "fixed_quantity" || value === "fixed_size" || value === "fixed") {
      return "fixed_quantity";
    }
    if (value === "fixed_fraction" || value === "fraction" || value === "percent" || value === "percentage") {
      return "fixed_fraction";
    }
    if (value === "volatility_adjusted" || value === "vol_adjusted" || value === "atr_adjusted") {
      return "volatility_adjusted";
    }
    if (Array.isArray(schedule) && schedule.length >= 2 && schedule.some((item) => Number(item) > 0)) {
      return "reentry_size_schedule";
    }
    return "fixed_quantity";
  }

  function readReentrySizeSchedule(raw: unknown): [string, string] {
    if (!Array.isArray(raw)) {
      return ["0.20", "0.10"];
    }
    const first = Number(raw[0]);
    const second = Number(raw[1]);
    return [
      Number.isFinite(first) && first > 0 ? String(first) : "0.20",
      Number.isFinite(second) && second > 0 ? String(second) : "0.10",
    ];
  }

  function buildLiveSessionSizingPayload() {
    const first = Number(liveSessionForm.reentrySizeScheduleFirst);
    const second = Number(liveSessionForm.reentrySizeScheduleSecond);
    return {
      positionSizingMode: liveSessionForm.positionSizingMode || "fixed_quantity",
      defaultOrderQuantity: Number(liveSessionForm.defaultOrderQuantity) || 0.001,
      reentry_size_schedule: [
        Number.isFinite(first) && first > 0 ? first : 0.20,
        Number.isFinite(second) && second > 0 ? second : 0.10,
      ],
    };
  }

  function selectQuickLiveAccount(accountId: string) {
    setLiveBindingForm((current: any) => ({ ...current, accountId }));
    setLiveSessionForm((current: any) => ({ ...current, accountId }));
    useUIStore.getState().setLiveOrderForm((current: any) => ({ ...current, accountId }));
    setSignalRuntimeForm((current: any) => ({ ...current, accountId }));
  }

  // --- Handlers ---

  async function createStrategy() {
    if (!strategyCreateForm.name.trim()) {
      setError("策略名称不能为空");
      return;
    }
    setStrategyCreateAction(true);
    try {
      setError(null);
      await fetchJSON("/api/v1/strategies", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: strategyCreateForm.name.trim(),
          description: strategyCreateForm.description.trim(),
          parameters: {
            strategyEngine: "bk-default",
            signalTimeframe: "1d",
            executionDataSource: "tick",
          },
        }),
      });
      useUIStore.getState().setStrategyCreateForm({ name: "", description: "" });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "创建策略失败");
    } finally {
      setStrategyCreateAction(false);
    }
  }

  async function saveStrategyParameters() {
    if (!strategyEditorForm.strategyId) {
      setError("请先选择策略");
      return;
    }
    setStrategySaveAction(true);
    try {
      setError(null);
      const parsed = JSON.parse(strategyEditorForm.parametersJson || "{}") as Record<string, unknown>;
      const nextParameters: Record<string, unknown> = {
        ...parsed,
        strategyEngine: strategyEditorForm.strategyEngine,
        signalTimeframe: strategyEditorForm.signalTimeframe,
        executionDataSource: strategyEditorForm.executionDataSource,
      };
      await fetchJSON(`/api/v1/strategies/${strategyEditorForm.strategyId}/parameters`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ parameters: nextParameters }),
      });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "保存策略参数失败");
    } finally {
      setStrategySaveAction(false);
    }
  }

  async function createLiveAccount() {
    if (!liveAccountForm.name.trim()) {
      setLiveAccountError("Live account name is required");
      return;
    }
    if (!liveAccountForm.exchange.trim()) {
      setLiveAccountError("Exchange is required");
      return;
    }
    setLiveAccountError(null);
    setLiveCreateAction(true);
    try {
      const created = await fetchJSON<AccountRecord>("/api/v1/accounts", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          name: liveAccountForm.name,
          mode: "LIVE",
          exchange: liveAccountForm.exchange,
        }),
      });
      if (created?.id) {
        selectQuickLiveAccount(created.id);
        setLiveAccountNotice(`已创建并选中账户：${created.name} (${created.id})`);
      }
      await loadDashboard();
      setError(null);
    } catch (err) {
      setLiveAccountNotice(null);
      setLiveAccountError(err instanceof Error ? err.message : "Failed to create live account");
    } finally {
      setLiveCreateAction(false);
    }
  }

  async function bindLiveAccount() {
    if (!liveBindingForm.accountId) {
      setLiveBindingError("Live binding needs an account");
      return;
    }
    setLiveBindingError(null);
    setLiveBindAction(true);
    try {
      await fetchJSON(`/api/v1/live/accounts/${liveBindingForm.accountId}/binding`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          adapterKey: liveBindingForm.adapterKey,
          positionMode: liveBindingForm.positionMode,
          marginMode: liveBindingForm.marginMode,
          sandbox: liveBindingForm.sandbox,
          credentialRefs: {
            apiKeyRef: liveBindingForm.apiKeyRef,
            apiSecretRef: liveBindingForm.apiSecretRef,
          },
        }),
      });
      await loadDashboard();
      const boundAccount = accounts.find((item) => item.id === liveBindingForm.accountId) || null;
      setLiveBindingNotice(
        `绑定成功：${boundAccount?.name ?? liveBindingForm.accountId} · ${liveBindingForm.adapterKey} · sandbox=${String(liveBindingForm.sandbox)}`
      );
      setError(null);
    } catch (err) {
      setLiveBindingNotice(null);
      setLiveBindingError(err instanceof Error ? err.message : "Failed to bind live account");
    } finally {
      setLiveBindAction(false);
    }
  }

  async function unbindLiveAccount(accountId: string) {
    setLiveBindAction(true);
    try {
      await fetchJSON(`/api/v1/live/accounts/${accountId}/binding`, { method: "DELETE" });
      await loadDashboard();
      setNotification({ type: 'success', message: "已成功解除账户适配器绑定" });
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "解绑失败";
      setError(message);
      setNotification({ type: 'error', message: `解绑失败: ${message}` });
    } finally {
      setLiveBindAction(false);
    }
  }

  async function stopLiveFlow(accountId: string, force = false) {
    setLiveFlowAction(accountId);
    try {
      const sessions = validLiveSessions.filter((session: LiveSession) =>
        session.accountId === accountId && String(session.status ?? "").toUpperCase() !== "STOPPED"
      );
      if (sessions.length === 0) {
        throw new Error("No active live session found for this account");
      }
      await Promise.all(sessions.map((session: LiveSession) =>
        fetchJSON(`/api/v1/live/sessions/${session.id}/stop${force ? '?force=true' : ''}`, { method: "POST" })
      ));
      await loadDashboard();
      setNotification({
        type: 'success',
        message: force ? "已提交强制停止实盘流程意图" : "已提交停止实盘流程意图",
      });
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to stop live flow";
      setError(message);
      setNotification({ type: 'error', message: `停止实盘流程失败: ${message}` });
    } finally {
      setLiveFlowAction(null);
    }
  }

  async function unbindStrategySignalSource(strategyId: string, bindingId: string) {
    try {
      await fetchJSON(`/api/v1/strategies/${strategyId}/signal-bindings?id=${encodeURIComponent(bindingId)}`, { method: "DELETE" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to unbind strategy signal source");
    }
  }

  async function deleteSignalRuntimeSession(sessionId: string, selectedSignalRuntimeId: string | null, force = false) {
    setSignalRuntimeAction(sessionId);
    try {
      await fetchJSON(`/api/v1/signal-runtime/sessions/${sessionId}${force ? '?force=true' : ''}`, { method: "DELETE" });
      if (sessionId === selectedSignalRuntimeId) {
        setSelectedSignalRuntimeId(null);
        setSignalRuntimePlan(null);
      }
      await loadDashboard();
      setNotification({ type: 'success', message: "已成功删除运行时会话" });
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete signal runtime session";
      if (!force && (message.includes("订单") || message.includes("未平仓") || message.includes("活动的"))) {
        useUIStore.getState().openConfirmDialog(
          "运行时删除阻断",
          `操作失败：${message}。强制删除将停止一切托管逻辑，是否确认强制删除？`,
          () => deleteSignalRuntimeSession(sessionId, selectedSignalRuntimeId, true)
        );
      } else {
        setError(message);
        setNotification({ type: 'error', message: `删除运行时会话失败: ${message}` });
      }
    } finally {
      setSignalRuntimeAction(null);
    }
  }

  async function closePosition(positionId: string) {
    setPositionCloseAction(positionId);
    try {
      await fetchJSON(`/api/v1/positions/${encodeURIComponent(positionId)}/close`, { method: "POST" });
      await loadDashboard();
      setNotification({ type: 'success', message: "已成功下发市价平仓委托" });
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to close position";
      setError(message);
      setNotification({ type: 'error', message: `平仓请求失败: ${message}` });
    } finally {
      setPositionCloseAction(null);
    }
  }

  async function syncLiveOrder(orderId: string) {
    setLiveSyncAction(orderId);
    try {
      await fetchJSON(`/api/v1/orders/${orderId}/sync`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sync live order");
    } finally {
      setLiveSyncAction(null);
    }
  }

  async function syncLiveAccount(accountId: string) {
    setLiveAccountSyncAction(accountId);
    try {
      await fetchJSON(`/api/v1/live/accounts/${accountId}/sync`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sync live account");
    } finally {
      setLiveAccountSyncAction(null);
    }
  }

  async function createLiveOrder() {
    if (!liveOrderForm.accountId || !liveOrderForm.symbol || !liveOrderForm.side || !liveOrderForm.type) {
      setError("Live order needs account, symbol, side, and type");
      return;
    }
    setLiveOrderAction(true);
    try {
      await fetchJSON("/api/v1/orders", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: liveOrderForm.accountId,
          strategyVersionId: liveOrderForm.strategyVersionId || undefined,
          symbol: liveOrderForm.symbol,
          side: liveOrderForm.side,
          type: liveOrderForm.type,
          quantity: Number(liveOrderForm.quantity) || 0,
          price: Number(liveOrderForm.price) || 0,
          metadata: {
            source: "live-console",
          },
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create live order");
    } finally {
      setLiveOrderAction(false);
    }
  }

  async function createLiveSession() {
    const normalizedStrategyId = normalizeStrategyId(liveSessionForm.strategyId, strategies[0]?.id || "");
    if (!liveSessionForm.accountId || !normalizedStrategyId) {
      setLiveSessionError("Live session needs an account and strategy");
      return null;
    }
    setLiveSessionError(null);
    setLiveSessionCreateAction(true);
    try {
      const created = await fetchJSON<LiveSession>("/api/v1/live/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: liveSessionForm.accountId,
          strategyId: normalizedStrategyId,
          alias: liveSessionForm.alias,
          signalTimeframe: liveSessionForm.signalTimeframe,
          executionDataSource: liveSessionForm.executionDataSource,
          symbol: liveSessionForm.symbol,
          ...buildLiveSessionSizingPayload(),
          executionEntryOrderType: liveSessionForm.executionEntryOrderType,
          executionEntryMaxSpreadBps: Number(liveSessionForm.executionEntryMaxSpreadBps) || undefined,
          executionEntryWideSpreadMode: liveSessionForm.executionEntryWideSpreadMode,
          executionEntryTimeoutFallbackOrderType: liveSessionForm.executionEntryTimeoutFallbackOrderType,
          executionPTExitOrderType: liveSessionForm.executionPTExitOrderType,
          executionPTExitTimeInForce: liveSessionForm.executionPTExitTimeInForce,
          executionPTExitPostOnly: liveSessionForm.executionPTExitPostOnly,
          executionPTExitTimeoutFallbackOrderType: liveSessionForm.executionPTExitTimeoutFallbackOrderType,
          executionSLExitOrderType: liveSessionForm.executionSLExitOrderType,
          executionSLExitMaxSpreadBps: Number(liveSessionForm.executionSLExitMaxSpreadBps) || undefined,
          dispatchMode: liveSessionForm.dispatchMode,
          dispatchCooldownSeconds: Number(liveSessionForm.dispatchCooldownSeconds) || 30,
        }),
      });
      await loadDashboard();
      setLiveSessionNotice(
        `会话已创建：${created?.id ?? "--"} · ${liveSessionForm.symbol} · ${liveSessionForm.signalTimeframe} · ${liveSessionForm.dispatchMode}`
      );
      setActiveSettingsModal(null);
      window.location.hash = "live";
      setError(null);
      return created;
    } catch (err) {
      setLiveSessionNotice(null);
      setLiveSessionError(err instanceof Error ? err.message : "Failed to create live session");
      return null;
    } finally {
      setLiveSessionCreateAction(false);
    }
  }

  async function saveLiveSession() {
    if (!editingLiveSessionId) {
      return createLiveSession();
    }
    const normalizedStrategyId = normalizeStrategyId(liveSessionForm.strategyId, strategies[0]?.id || "");
    if (!liveSessionForm.accountId || !normalizedStrategyId) {
      setLiveSessionError("Live session needs an account and strategy");
      return null;
    }
    setLiveSessionError(null);
    setLiveSessionCreateAction(true);
    try {
      const updated = await fetchJSON<LiveSession>(`/api/v1/live/sessions/${editingLiveSessionId}`, {
        method: "PUT",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: liveSessionForm.accountId,
          strategyId: normalizedStrategyId,
          alias: liveSessionForm.alias,
          signalTimeframe: liveSessionForm.signalTimeframe,
          executionDataSource: liveSessionForm.executionDataSource,
          symbol: liveSessionForm.symbol,
          ...buildLiveSessionSizingPayload(),
          executionEntryOrderType: liveSessionForm.executionEntryOrderType,
          executionEntryMaxSpreadBps: Number(liveSessionForm.executionEntryMaxSpreadBps) || undefined,
          executionEntryWideSpreadMode: liveSessionForm.executionEntryWideSpreadMode,
          executionEntryTimeoutFallbackOrderType: liveSessionForm.executionEntryTimeoutFallbackOrderType,
          executionPTExitOrderType: liveSessionForm.executionPTExitOrderType,
          executionPTExitTimeInForce: liveSessionForm.executionPTExitTimeInForce,
          executionPTExitPostOnly: liveSessionForm.executionPTExitPostOnly,
          executionPTExitTimeoutFallbackOrderType: liveSessionForm.executionPTExitTimeoutFallbackOrderType,
          executionSLExitOrderType: liveSessionForm.executionSLExitOrderType,
          executionSLExitMaxSpreadBps: Number(liveSessionForm.executionSLExitMaxSpreadBps) || undefined,
          dispatchMode: liveSessionForm.dispatchMode,
          dispatchCooldownSeconds: Number(liveSessionForm.dispatchCooldownSeconds) || 30,
        }),
      });
      await loadDashboard();
      setLiveSessionNotice(`会话已更新：${updated?.id ?? editingLiveSessionId}`);
      setActiveSettingsModal(null);
      setEditingLiveSessionId(null);
      setError(null);
      return updated;
    } catch (err) {
      setLiveSessionError(err instanceof Error ? err.message : "Failed to update live session");
      return null;
    } finally {
      setLiveSessionCreateAction(false);
    }
  }

  async function createAndStartLiveSession() {
    setLiveSessionLaunchAction(true);
    setLiveSessionError(null);
    try {
      const created = await createLiveSession();
      if (!created?.id) return;
      setLiveSessionAction(`${created.id}:start`);
      await fetchJSON(`/api/v1/live/sessions/${created.id}/start`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setLiveSessionError(err instanceof Error ? err.message : "Failed to create and start live session");
    } finally {
      setLiveSessionAction(null);
      setLiveSessionLaunchAction(false);
    }
  }

  async function deleteLiveSession(sessionId: string, force = false) {
    setLiveSessionDeleteAction(sessionId);
    try {
      await fetchJSON(`/api/v1/live/sessions/${sessionId}${force ? '?force=true' : ''}`, { method: "DELETE" });
      await loadDashboard();
      if (activeSettingsModal === "live-session") {
        setLiveSessionNotice(`已删除会话：${sessionId}`);
      }
      setNotification({ type: 'success', message: `已成功删除实盘会话: ${sessionId}` });
      setError(null);
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to delete live session";
      if (!force && (message.includes("订单") || message.includes("未平仓") || message.includes("活动的"))) {
        useUIStore.getState().openConfirmDialog(
          "会话删除被阻断",
          `操作失败：${message}。强制删除可能导致幽灵仓单不受托管，是否确认强制跳过安全检查？`,
          () => deleteLiveSession(sessionId, true)
        );
      } else {
        setError(message);
        setNotification({ type: 'error', message: `会话删除失败: ${message}` });
      }
    } finally {
      setLiveSessionDeleteAction(null);
    }
  }

  async function launchLiveFlow(account: AccountRecord) {
    const strategyId = validLiveSessions.find((item: LiveSession) => item.accountId === account.id)?.strategyId ||
                       liveSessionForm.strategyId || strategies[0]?.id || "";
    if (!strategyId) {
      setError("Launch live flow needs a strategy");
      return;
    }

    setLiveFlowAction(account.id);
    setError(null);
    selectQuickLiveAccount(account.id);
    
    try {
      const strategyBindings = strategySignalBindingMap[strategyId] ?? [];
      if (strategyBindings.length === 0) {
        window.location.hash = "signals";
        throw new Error("Launch live flow needs strategy signal bindings before it can continue");
      }
      const launchResult = await fetchJSON<LiveLaunchResult>(`/api/v1/live/accounts/${account.id}/launch`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          strategyId,
          binding: {
            adapterKey: liveBindingForm.adapterKey,
            positionMode: liveBindingForm.positionMode,
            marginMode: liveBindingForm.marginMode,
            sandbox: liveBindingForm.sandbox,
            credentialRefs: {
              apiKeyRef: liveBindingForm.apiKeyRef,
              apiSecretRef: liveBindingForm.apiSecretRef,
            },
          },
          mirrorStrategySignals: true,
          startRuntime: false,
          startSession: false,
          liveSessionOverrides: {
            alias: liveSessionForm.alias,
            signalTimeframe: liveSessionForm.signalTimeframe,
            executionDataSource: liveSessionForm.executionDataSource,
            symbol: liveSessionForm.symbol,
            ...buildLiveSessionSizingPayload(),
            dispatchMode: liveSessionForm.dispatchMode,
            dispatchCooldownSeconds: Number(liveSessionForm.dispatchCooldownSeconds) || 30,
          },
        }),
      });
      if (launchResult.liveSession?.id) {
        await fetchJSON(`/api/v1/live/sessions/${launchResult.liveSession.id}/start`, { method: "POST" });
      }

      await loadDashboard();
      setNotification({ type: 'success', message: "已提交实盘流程启动意图" });
      window.location.hash = "live";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to launch live flow");
    } finally {
      setLiveFlowAction(null);
    }
  }

  async function runLiveSessionAction(sessionId: string, action: "start" | "stop", force = false) {
    try {
      setLiveSessionAction(`${sessionId}:${action}`);
      setError(null);
      setLiveSessionError(null);
      await fetchJSON(`/api/v1/live/sessions/${sessionId}/${action}${force ? '?force=true' : ''}`, { method: "POST" });
      await loadDashboard();
      if (action === "stop") {
        setNotification({ type: 'success', message: `已提交停用意图: ${sessionId}` });
      } else {
        setNotification({ type: 'success', message: `已提交启动意图: ${sessionId}` });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to execute live session action";
      
      if (!force && action === "stop" && (message.includes("订单") || message.includes("未平仓") || message.includes("活动的"))) {
        useUIStore.getState().openConfirmDialog(
          "会话停用被阻断",
          `操作失败：${message}。强制停用可能导致存活的仓单无法按照预定逻辑平出，是否确认强制停用？`,
          () => runLiveSessionAction(sessionId, action, true)
        );
        return;
      }

      setError(message);
      setLiveSessionError(message);
      setNotification({ type: 'error', message: `实盘操作失败: ${message}` });

      if (action === "start" && message.includes("is not configured")) {
        const session = liveSessions.find((item) => item.id === sessionId);
        if (session?.accountId) {
          selectQuickLiveAccount(session.accountId);
          setLiveBindingError(message);
          setActiveSettingsModal("live-binding");
        }
      }
    } finally {
      setLiveSessionAction(null);
    }
  }

  async function dispatchLiveSessionIntent(sessionId: string) {
    try {
      setLiveSessionAction(`${sessionId}:dispatch`);
      setError(null);
      setLiveSessionError(null);
      await fetchJSON(`/api/v1/live/sessions/${sessionId}/dispatch`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to dispatch live session intent";
      setError(message);
      setLiveSessionError(message);
      setNotification({ type: 'error', message: `分发意图失败: ${message}` });
    } finally {
      setLiveSessionAction(null);
    }
  }

  async function syncLiveSession(sessionId: string) {
    try {
      setLiveSessionAction(`${sessionId}:sync`);
      setError(null);
      await fetchJSON(`/api/v1/live/sessions/${sessionId}/sync`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to sync live session");
    } finally {
      setLiveSessionAction(null);
    }
  }

  async function executeLaunchTemplate(template: LaunchTemplate, accountId: string) {
    if (!accountId) {
      setError("请先选择或创建一个实盘账户");
      return;
    }

    setLaunchingTemplate(template.key);
    setError(null);
    
    let completedSteps = 0;
    const totalSteps = template.steps.length;

    try {
      let lastLaunchResult: LiveLaunchResult | null = null;
      for (const step of template.steps) {
        setNotification({ 
          type: 'info', 
          message: `正在执行 (${completedSteps + 1}/${totalSteps}): ${step.label || '正在处理...'}` 
        });

        const path = step.pathTemplate
          .replace(":accountId", encodeURIComponent(accountId))
          .replace(":strategyId", encodeURIComponent(template.strategyId));
        
        // 独占切换核心：对于 launch 步，直接应用模板内的完整 payload
        let body = {};
        if (path.endsWith("/launch") && template.launchPayload) {
          body = {
            ...template.launchPayload,
            liveSessionOverrides: {
              ...(template.launchPayload.liveSessionOverrides || {}),
              dispatchMode: template.defaultDispatchMode || "manual-review"
            }
          };
        } else {
          const payloadRef = step.payloadRef;
          body = template[payloadRef] || {};
        }

        const result = await fetchJSON<any>(path, {
          method: step.method,
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify(body),
        });

        // 如果是最后一步启动，捕获审计信息
        if (path.endsWith("/launch") && result) {
          lastLaunchResult = result as LiveLaunchResult;
        }

        completedSteps++;
      }

      await loadDashboard();
      
      let successMsg = `模板 "${template.name}" 应用成功：已刷新订阅。`;
      if (lastLaunchResult?.stoppedLiveSessions && lastLaunchResult.stoppedLiveSessions > 0) {
        successMsg += ` 并由于独占切换关停了 ${lastLaunchResult.stoppedLiveSessions} 个旧会话。`;
      }

      setNotification({ type: 'success', message: successMsg });
      window.location.hash = "monitor";
    } catch (err) {
      const message = err instanceof Error ? err.message : "模板应用失败";
      setError(message);
      setNotification({ 
        type: 'error', 
        message: `配置中断 (第 ${completedSteps + 1}/${totalSteps} 步): ${message}` 
      });
    } finally {
      setLaunchingTemplate(null);
    }
  }

  async function bindStrategySignalSource() {
    if (!strategySignalForm.strategyId || !strategySignalForm.sourceKey) {
      setError("Strategy signal binding needs a strategy and source");
      return;
    }
    setSignalBindingAction("strategy");
    try {
      await fetchJSON(`/api/v1/strategies/${strategySignalForm.strategyId}/signal-bindings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sourceKey: strategySignalForm.sourceKey,
          role: strategySignalForm.role,
          symbol: strategySignalForm.symbol,
          options: strategySignalForm.role === "signal" ? { timeframe: strategySignalForm.timeframe } : undefined,
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to bind strategy signal source");
    } finally {
      setSignalBindingAction(null);
    }
  }

  async function createSignalRuntimeSession() {
    if (!signalRuntimeForm.accountId || !signalRuntimeForm.strategyId) {
      setError("Signal runtime session needs an account and strategy");
      return;
    }
    setSignalRuntimeAction("create");
    try {
      await fetchJSON("/api/v1/signal-runtime/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: signalRuntimeForm.accountId,
          strategyId: signalRuntimeForm.strategyId,
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create signal runtime session");
    } finally {
      setSignalRuntimeAction(null);
    }
  }

  async function updateRuntimePolicy() {
    setRuntimePolicyAction(true);
    try {
      const payload = {
        tradeTickFreshnessSeconds: Math.max(0, Number(runtimePolicyForm.tradeTickFreshnessSeconds) || 0),
        orderBookFreshnessSeconds: Math.max(0, Number(runtimePolicyForm.orderBookFreshnessSeconds) || 0),
        signalBarFreshnessSeconds: Math.max(0, Number(runtimePolicyForm.signalBarFreshnessSeconds) || 0),
        runtimeQuietSeconds: Math.max(0, Number(runtimePolicyForm.runtimeQuietSeconds) || 0),
        strategyEvaluationQuietSeconds: Math.max(0, Number(runtimePolicyForm.strategyEvaluationQuietSeconds) || 0),
        liveAccountSyncFreshnessSeconds: Math.max(0, Number(runtimePolicyForm.liveAccountSyncFreshnessSeconds) || 0),
        paperStartReadinessTimeoutSeconds: Math.max(0, Number(runtimePolicyForm.paperStartReadinessTimeoutSeconds) || 0),
      };
      const updated = await fetchJSON<RuntimePolicy>("/api/v1/runtime-policy", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(payload),
      });
      setRuntimePolicy(updated);
      setRuntimePolicyForm({
        tradeTickFreshnessSeconds: String(updated.tradeTickFreshnessSeconds ?? payload.tradeTickFreshnessSeconds),
        orderBookFreshnessSeconds: String(updated.orderBookFreshnessSeconds ?? payload.orderBookFreshnessSeconds),
        signalBarFreshnessSeconds: String(updated.signalBarFreshnessSeconds ?? payload.signalBarFreshnessSeconds),
        runtimeQuietSeconds: String(updated.runtimeQuietSeconds ?? payload.runtimeQuietSeconds),
        strategyEvaluationQuietSeconds: String(
          updated.strategyEvaluationQuietSeconds ?? payload.strategyEvaluationQuietSeconds
        ),
        liveAccountSyncFreshnessSeconds: String(
          updated.liveAccountSyncFreshnessSeconds ?? payload.liveAccountSyncFreshnessSeconds
        ),
        paperStartReadinessTimeoutSeconds: String(
          updated.paperStartReadinessTimeoutSeconds ?? payload.paperStartReadinessTimeoutSeconds
        ),
        dispatchMode: String(updated.dispatchMode ?? runtimePolicyForm.dispatchMode),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update runtime policy");
    } finally {
      setRuntimePolicyAction(false);
    }
  }

  async function runSignalRuntimeAction(sessionId: string, action: "start" | "stop", force = false) {
    setSignalRuntimeAction(`${sessionId}:${action}`);
    try {
      await fetchJSON(`/api/v1/signal-runtime/sessions/${sessionId}/${action}${force ? '?force=true' : ''}`, { method: "POST" });
      await loadDashboard();
      setError(null);
      if (action === "stop") {
        setNotification({ type: 'success', message: "已停用运行时会话" });
      }
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to execute signal runtime action";
      if (!force && action === "stop" && (message.includes("订单") || message.includes("未平仓") || message.includes("活动的"))) {
        useUIStore.getState().openConfirmDialog(
          "运行时停用阻断",
          `操作失败：${message}。强制停用将中断所有信号与处理流程，是否确认强制停用？`,
          () => runSignalRuntimeAction(sessionId, action, true)
        );
      } else {
        setError(message);
        setNotification({ type: 'error', message: `操作失败: ${message}` });
      }
    } finally {
      setSignalRuntimeAction(null);
    }
  }

  async function acknowledgeNotification(notification: PlatformNotification, acknowledged: boolean) {
    setNotificationAction(`${notification.id}:${acknowledged ? "ack" : "unack"}`);
    try {
      await fetchJSON(`/api/v1/notifications/${encodeURIComponent(notification.id)}/ack`, {
        method: acknowledged ? "POST" : "DELETE",
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update notification");
    } finally {
      setNotificationAction(null);
    }
  }

  async function sendNotificationToTelegram(notification: PlatformNotification) {
    setTelegramAction(`send:${notification.id}`);
    try {
      await fetchJSON(`/api/v1/notifications/${encodeURIComponent(notification.id)}/telegram`, {
        method: "POST",
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send Telegram notification");
    } finally {
      setTelegramAction(null);
    }
  }

  async function saveTelegramConfig() {
    setTelegramAction("save-config");
    try {
      const updated = await fetchJSON<TelegramConfig>("/api/v1/telegram/config", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          enabled: telegramForm.enabled,
          botToken: telegramForm.botToken || undefined,
          chatId: telegramForm.chatId,
          sendLevels: telegramForm.sendLevels
            .split(",")
            .map((item: string) => item.trim().toLowerCase())
            .filter(Boolean),
          tradeEventsEnabled: telegramForm.tradeEventsEnabled,
          positionReportEnabled: telegramForm.positionReportEnabled,
          positionReportIntervalMinutes: Number(telegramForm.positionReportIntervalMinutes) || 30,
        }),
      });
      setTelegramConfig(updated);
      useUIStore.getState().setTelegramForm((current) => ({ 
        ...current, 
        enabled: Boolean(updated.enabled),
        chatId: String(updated.chatId ?? ""),
        botToken: "",
        tradeEventsEnabled: Boolean(updated.tradeEventsEnabled),
        positionReportEnabled: Boolean(updated.positionReportEnabled),
        positionReportIntervalMinutes: String(updated.positionReportIntervalMinutes ?? 30),
      }));
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to save Telegram config");
    } finally {
      setTelegramAction(null);
    }
  }

  async function sendTelegramTest() {
    setTelegramAction("test");
    try {
      await fetchJSON("/api/v1/telegram/test", { method: "POST" });
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to send Telegram test message");
    } finally {
      setTelegramAction(null);
    }
  }

  async function createBacktestRun() {
    try {
      setBacktestAction(true);
      setError(null);
      await fetchJSON<BacktestRun>("/api/v1/backtests", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          strategyVersionId: backtestForm.strategyVersionId,
          parameters: {
            signalTimeframe: backtestForm.signalTimeframe,
            executionDataSource: backtestForm.executionDataSource,
            symbol: backtestForm.symbol,
            from: backtestForm.from || undefined,
            to: backtestForm.to || undefined,
          },
        }),
      });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create backtest");
    } finally {
      setBacktestAction(false);
    }
  }

  async function login() {
    if (!loginForm.username.trim() || !loginForm.password) {
      setError("请输入用户名和密码");
      return;
    }
    setLoginAction(true);
    try {
      const response = await fetch(`${API_BASE}/api/v1/auth/login`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          username: loginForm.username.trim(),
          password: loginForm.password,
        }),
      });
      if (!response.ok) {
        const payload = (await response.json().catch(() => null)) as { error?: string } | null;
        throw new Error(payload?.error || `HTTP ${response.status} for /api/v1/auth/login`);
      }
      const payload = (await response.json()) as { token: string; username: string; expiresAt?: string };
      const session: AuthSession = {
        token: payload.token,
        username: payload.username,
        expiresAt: payload.expiresAt,
      };
      writeStoredAuthSession(session);
      setAuthSession(session);
      useUIStore.getState().setLoginForm((current) => ({ ...current, password: "" }));
      setError(null);
      setLoading(true);
      await loadDashboard();
      setLoading(false);
    } catch (err) {
      setError(err instanceof Error ? err.message : "登录失败");
    } finally {
      setLoginAction(false);
    }
  }

  function logout() {
    writeStoredAuthSession(null);
    setAuthSession(null);
    setSummaries([]);
    setAccounts([]);
    setOrders([]);
    setFills([]);
    setPositions([]);
    setSnapshots([]);
    useTradingStore.getState().setLiveSessions([]);
    setError(null);
    setLoading(false);
  }

  // --- Modal Helpers ---
  function openLiveAccountModal() {
    const baseName = "Binance Testnet";
    const existingNames = new Set(accounts.map((item: AccountRecord) => item.name));
    let nextName = baseName;
    let suffix = 2;
    while (existingNames.has(nextName)) {
      nextName = `${baseName} ${suffix}`;
      suffix += 1;
    }
    useUIStore.getState().setLiveAccountForm((current) => ({
      ...current,
      name: current.name.trim() === "" || existingNames.has(current.name) ? nextName : current.name,
      exchange: current.exchange || "binance-futures",
    }));
    setError(null);
    setLiveAccountError(null);
    setLiveAccountNotice(null);
    setActiveSettingsModal("live-account");
  }

  function openLiveBindingModal(quickLiveAccountId: string) {
    if (quickLiveAccountId) {
      selectQuickLiveAccount(quickLiveAccountId);
    }
    setError(null);
    setLiveBindingError(null);
    setLiveBindingNotice(null);
    setActiveSettingsModal("live-binding");
  }

  function openLiveSessionModal(session: LiveSession | null, quickLiveAccountId: string, strategies: StrategyRecord[]) {
    const nextAccountId = session?.accountId || quickLiveAccountId || accounts[0]?.id || "";
    const nextStrategyId = normalizeStrategyId(session?.strategyId ?? "", strategies[0]?.id || "");
    const sessionState = getRecord(session?.state);
    const isEditingExistingSession = Boolean(session);
    const [reentrySizeScheduleFirst, reentrySizeScheduleSecond] = readReentrySizeSchedule(sessionState.reentry_size_schedule);
    const positionSizingMode = normalizeLivePositionSizingMode(sessionState.positionSizingMode, sessionState.reentry_size_schedule);
    if (nextAccountId) {
      selectQuickLiveAccount(nextAccountId);
    }
    useUIStore.getState().setLiveSessionForm((current) => ({
      ...current,
      accountId: nextAccountId || current.accountId,
      strategyId: nextStrategyId,
      signalTimeframe: String(sessionState.signalTimeframe ?? (isEditingExistingSession ? "" : current.signalTimeframe ?? "1d")),
      executionDataSource: String(sessionState.executionDataSource ?? (isEditingExistingSession ? "" : current.executionDataSource ?? "tick")),
      symbol: String(sessionState.symbol ?? (isEditingExistingSession ? "" : current.symbol ?? "BTCUSDT")),
      positionSizingMode: String(
        sessionState.positionSizingMode != null || Array.isArray(sessionState.reentry_size_schedule)
          ? positionSizingMode
          : (isEditingExistingSession ? "fixed_quantity" : current.positionSizingMode ?? "fixed_quantity")
      ),
      defaultOrderQuantity: String(
        sessionState.defaultOrderQuantity ?? (isEditingExistingSession ? "" : current.defaultOrderQuantity ?? "0.001")
      ),
      reentrySizeScheduleFirst: String(
        sessionState.reentry_size_schedule != null ? reentrySizeScheduleFirst : current.reentrySizeScheduleFirst ?? "0.20"
      ),
      reentrySizeScheduleSecond: String(
        sessionState.reentry_size_schedule != null ? reentrySizeScheduleSecond : current.reentrySizeScheduleSecond ?? "0.10"
      ),
      executionEntryOrderType: String(
        sessionState.executionEntryOrderType ?? (isEditingExistingSession ? "" : current.executionEntryOrderType ?? "MARKET")
      ),
      executionEntryMaxSpreadBps: String(
        sessionState.executionEntryMaxSpreadBps ?? (isEditingExistingSession ? "" : current.executionEntryMaxSpreadBps ?? "8")
      ),
      executionEntryWideSpreadMode: String(
        sessionState.executionEntryWideSpreadMode ?? (isEditingExistingSession ? "" : current.executionEntryWideSpreadMode ?? "limit-maker")
      ),
      executionEntryTimeoutFallbackOrderType: String(
        sessionState.executionEntryTimeoutFallbackOrderType ??
          (isEditingExistingSession ? "" : current.executionEntryTimeoutFallbackOrderType ?? "MARKET")
      ),
      executionPTExitOrderType: String(
        sessionState.executionPTExitOrderType ?? (isEditingExistingSession ? "" : current.executionPTExitOrderType ?? "LIMIT")
      ),
      executionPTExitTimeInForce: String(
        sessionState.executionPTExitTimeInForce ?? (isEditingExistingSession ? "" : current.executionPTExitTimeInForce ?? "GTX")
      ),
      executionPTExitPostOnly: isEditingExistingSession
        ? Boolean(sessionState.executionPTExitPostOnly)
        : Boolean(sessionState.executionPTExitPostOnly ?? current.executionPTExitPostOnly),
      executionPTExitTimeoutFallbackOrderType: String(
        sessionState.executionPTExitTimeoutFallbackOrderType ??
          (isEditingExistingSession ? "" : current.executionPTExitTimeoutFallbackOrderType ?? "MARKET")
      ),
      executionSLExitOrderType: String(
        sessionState.executionSLExitOrderType ?? (isEditingExistingSession ? "" : current.executionSLExitOrderType ?? "MARKET")
      ),
      executionSLExitMaxSpreadBps: String(
        sessionState.executionSLExitMaxSpreadBps ?? (isEditingExistingSession ? "" : current.executionSLExitMaxSpreadBps ?? "999")
      ),
      dispatchMode: String(sessionState.dispatchMode ?? (isEditingExistingSession ? "" : current.dispatchMode ?? "manual-review")),
      dispatchCooldownSeconds: String(
        sessionState.dispatchCooldownSeconds ?? (isEditingExistingSession ? "" : current.dispatchCooldownSeconds ?? "30")
      ),
    }));
    setEditingLiveSessionId(session?.id ?? null);
    setError(null);
    setLiveSessionError(null);
    setLiveSessionNotice(null);
    setActiveSettingsModal("live-session");
  }

  function runLiveNextAction(account: AccountRecord, action: LiveNextAction, activeRuntime: SignalRuntimeSession | null) {
    switch (action.key) {
      case "bind-live-adapter":
        useUIStore.getState().setLiveBindingForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "live";
        break;
      case "bind-signals":
        useUIStore.getState().setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "create-runtime":
        useUIStore.getState().setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "start-runtime":
      case "inspect-runtime":
        if (activeRuntime) {
          jumpToSignalRuntimeSession(activeRuntime.id);
        } else {
          useUIStore.getState().setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
          window.location.hash = "signals";
        }
        break;
      case "pass-strategy-version":
        window.location.hash = "live";
        break;
      case "submit-live-order":
        window.location.hash = "orders";
        break;
      default:
        window.location.hash = "signals";
        break;
    }
  }

  function jumpToSignalRuntimeSession(sessionId: string) {
    setSelectedSignalRuntimeId(sessionId);
    useUIStore.getState().setSidebarTab("monitor");
    window.location.hash = "signals";
  }

  return {
    createStrategy, saveStrategyParameters, createLiveAccount, bindLiveAccount,
    stopLiveFlow, unbindStrategySignalSource,
    deleteSignalRuntimeSession, syncLiveOrder, syncLiveAccount, createLiveOrder,
    createLiveSession, saveLiveSession, createAndStartLiveSession, deleteLiveSession,
    closePosition,
    launchLiveFlow, runLiveSessionAction, dispatchLiveSessionIntent, syncLiveSession,
    bindStrategySignalSource, createSignalRuntimeSession,
    updateRuntimePolicy, runSignalRuntimeAction, acknowledgeNotification,
    sendNotificationToTelegram, saveTelegramConfig, sendTelegramTest, createBacktestRun,
    executeLaunchTemplate,
    unbindLiveAccount,
    selectQuickLiveAccount,
    login, logout,
    openLiveAccountModal, openLiveBindingModal, openLiveSessionModal,
    runLiveNextAction, jumpToSignalRuntimeSession,
    setLoginForm, setLiveAccountForm, setLiveBindingForm, setLiveSessionForm, 
    setStrategySignalForm, setSignalRuntimeForm,
    setTelegramForm,
    setLiveSessionLaunchAction, setLiveSessionAction, setLiveSessionError, setError
  };
}
