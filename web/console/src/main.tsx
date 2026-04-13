/// <reference types="vite/client" />
import { SlideOver } from './components/SlideOver';
import { WorkbenchLayout } from './layouts/WorkbenchLayout';
import { useUIStore } from './store/useUIStore';
import { useTradingStore } from './store/useTradingStore';
import React, { useEffect, useMemo, useRef, useState } from "react";
import ReactDOM from "react-dom/client";
import "./styles.css";

import { API_BASE, fetchJSON } from './utils/api';
import { readStoredAuthSession, writeStoredAuthSession } from './utils/auth';
import { ErrorBoundary } from './components/ErrorBoundary';
import { ActionButton } from './components/ui/ActionButton';
import { StatusPill } from './components/ui/StatusPill';
import { MetricCard } from './components/ui/MetricCard';
import { FilterGroup } from './components/ui/FilterGroup';
import { SimpleTable } from './components/ui/SimpleTable';
import { SampleCard } from './components/ui/SampleCard';
import { TradingChart } from './components/charts/TradingChart';
import { SignalBarChart } from './components/charts/SignalBarChart';
import { SignalMonitorChart } from './components/charts/SignalMonitorChart';
import { LoginModal } from './modals/LoginModal';
import { LiveAccountModal } from './modals/LiveAccountModal';
import { LiveBindingModal } from './modals/LiveBindingModal';
import { LiveSessionModal } from './modals/LiveSessionModal';
import { TelegramModal } from './modals/TelegramModal';
import { OrdersPanel } from './panels/OrdersPanel';
import { AlertsPanel } from './panels/AlertsPanel';
import { PositionsPanel } from './panels/PositionsPanel';
import { FillsPanel } from './panels/FillsPanel';
import { StrategySidePanel } from './pages/StrategySidePanel';
import { AccountSidePanel } from './pages/AccountSidePanel';
import { MonitorStage } from './pages/MonitorStage';
import { StrategyStage } from './pages/StrategyStage';
import { AccountStage } from './pages/AccountStage';

import { AccountSummary, AccountRecord, StrategyVersion, StrategyRecord, AccountEquitySnapshot, Order, Fill, Position, PaperSession, LiveSession, ChartCandle, ChartAnnotation, MarkerLegendItem, BacktestRun, BacktestOptions, LiveAdapter, SignalSourceDefinition, SignalSourceCatalog, SignalSourceType, SignalBinding, SignalRuntimeAdapter, SignalRuntimeSession, ReplayReasonStats, ReplaySample, ExecutionTrade, SourceFilter, EventFilter, TimeWindow, MarkerDetail, ChartOverrideRange, SelectedSample, SelectableSample, RuntimeMarketSnapshot, RuntimeSourceSummary, RuntimeReadiness, SignalBarCandle, AlertItem, PlatformAlert, PlatformNotification, TelegramConfig, RuntimePolicy, LivePreflightSummary, LiveNextAction, LiveDispatchPreview, LiveSessionExecutionSummary, LiveSessionHealth, HighlightedLiveSession, LiveSessionFlowStep, SessionMarker, AuthSession } from './types/domain';
import { sampleStatus, buildLinePath, summarizeRange, summarizeTimeRange, filterChartAnnotations, matchesEventFilter, resolveChartAnchor, buildTimeRange, buildSampleRange, buildSampleKey, annotationMatchesSample, findNearestAnnotation, toMarkerDetail, markerShape, markerPosition, markerColor, markerText, annotationTone, paperAccountsFromSummaries, strategyLabel, getNumber, getRecord, getList, deriveRuntimeMarketSnapshot, deriveRuntimeSourceSummary, deriveRuntimeReadiness, deriveSignalBarCandles, mapChartCandlesToSignalBarCandles, applyDefaultChartWindow, derivePrimarySignalBarState, buildRuntimeEventNotes, buildSourceStateNotes, buildSignalBarDecisionNotes, buildSignalBarStateNotes, deriveSignalActionSummary, deriveLivePreflightSummary, deriveLiveDispatchPreview, deriveLiveSessionExecutionSummary, derivePaperSessionExecutionSummary, deriveSessionMarkers, deriveLiveSessionHealth, deriveHighlightedLiveSession, deriveLiveSessionFlow, liveSessionHealthPriority, deriveLiveNextAction, liveSessionHealthTone, buildSignalActionNotes, buildTimelineNotes, summarizeOrderPreflight, derivePaperAlerts, deriveLiveAlerts, dedupeAlerts, buildAlertNotes, alertLevelTone, alertScopeTone, telegramDeliveryTone, runtimeReadinessTone, decisionStateTone, signalKindTone, signalActionTone, boolTone, boolLabel } from './utils/derivation';
import { formatMoney, formatSigned, formatPercent, formatNumber, formatMaybeNumber, formatTime, formatShortTime, shrink } from './utils/format';

function App() {

  const sidebarTab = useUIStore(s => s.sidebarTab);
  const setSidebarTab = useUIStore(s => s.setSidebarTab);
  const dockTab = useUIStore(s => s.dockTab);
  const setDockTab = useUIStore(s => s.setDockTab);
  const loading = useUIStore(s => s.loading);
  const setLoading = useUIStore(s => s.setLoading);
  const error = useUIStore(s => s.error);
  const setError = useUIStore(s => s.setError);
  const authSession = useUIStore(s => s.authSession);
  const setAuthSession = useUIStore(s => s.setAuthSession);
  const loginForm = useUIStore(s => s.loginForm);
  const setLoginForm = useUIStore(s => s.setLoginForm);
  const loginAction = useUIStore(s => s.loginAction);
  const setLoginAction = useUIStore(s => s.setLoginAction);
  const sessionAction = useUIStore(s => s.sessionAction);
  const setSessionAction = useUIStore(s => s.setSessionAction);
  const paperCreateAction = useUIStore(s => s.paperCreateAction);
  const setPaperCreateAction = useUIStore(s => s.setPaperCreateAction);
  const paperLaunchAction = useUIStore(s => s.paperLaunchAction);
  const setPaperLaunchAction = useUIStore(s => s.setPaperLaunchAction);
  const liveCreateAction = useUIStore(s => s.liveCreateAction);
  const setLiveCreateAction = useUIStore(s => s.setLiveCreateAction);
  const liveBindAction = useUIStore(s => s.liveBindAction);
  const setLiveBindAction = useUIStore(s => s.setLiveBindAction);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);
  const setLiveSyncAction = useUIStore(s => s.setLiveSyncAction);
  const liveAccountSyncAction = useUIStore(s => s.liveAccountSyncAction);
  const setLiveAccountSyncAction = useUIStore(s => s.setLiveAccountSyncAction);
  const liveFlowAction = useUIStore(s => s.liveFlowAction);
  const setLiveFlowAction = useUIStore(s => s.setLiveFlowAction);
  const liveOrderAction = useUIStore(s => s.liveOrderAction);
  const setLiveOrderAction = useUIStore(s => s.setLiveOrderAction);
  const liveSessionAction = useUIStore(s => s.liveSessionAction);
  const setLiveSessionAction = useUIStore(s => s.setLiveSessionAction);
  const liveSessionCreateAction = useUIStore(s => s.liveSessionCreateAction);
  const setLiveSessionCreateAction = useUIStore(s => s.setLiveSessionCreateAction);
  const liveSessionLaunchAction = useUIStore(s => s.liveSessionLaunchAction);
  const setLiveSessionLaunchAction = useUIStore(s => s.setLiveSessionLaunchAction);
  const liveSessionDeleteAction = useUIStore(s => s.liveSessionDeleteAction);
  const setLiveSessionDeleteAction = useUIStore(s => s.setLiveSessionDeleteAction);
  const signalBindingAction = useUIStore(s => s.signalBindingAction);
  const setSignalBindingAction = useUIStore(s => s.setSignalBindingAction);
  const signalRuntimeAction = useUIStore(s => s.signalRuntimeAction);
  const setSignalRuntimeAction = useUIStore(s => s.setSignalRuntimeAction);
  const notificationAction = useUIStore(s => s.notificationAction);
  const setNotificationAction = useUIStore(s => s.setNotificationAction);
  const telegramAction = useUIStore(s => s.telegramAction);
  const setTelegramAction = useUIStore(s => s.setTelegramAction);
  const backtestAction = useUIStore(s => s.backtestAction);
  const setBacktestAction = useUIStore(s => s.setBacktestAction);
  const runtimePolicyAction = useUIStore(s => s.runtimePolicyAction);
  const setRuntimePolicyAction = useUIStore(s => s.setRuntimePolicyAction);
  const strategyCreateAction = useUIStore(s => s.strategyCreateAction);
  const setStrategyCreateAction = useUIStore(s => s.setStrategyCreateAction);
  const strategySaveAction = useUIStore(s => s.strategySaveAction);
  const setStrategySaveAction = useUIStore(s => s.setStrategySaveAction);
  const sourceFilter = useUIStore(s => s.sourceFilter);
  const setSourceFilter = useUIStore(s => s.setSourceFilter);
  const eventFilter = useUIStore(s => s.eventFilter);
  const setEventFilter = useUIStore(s => s.setEventFilter);
  const timeWindow = useUIStore(s => s.timeWindow);
  const setTimeWindow = useUIStore(s => s.setTimeWindow);
  const focusNonce = useUIStore(s => s.focusNonce);
  const setFocusNonce = useUIStore(s => s.setFocusNonce);
  const hoveredMarker = useUIStore(s => s.hoveredMarker);
  const setHoveredMarker = useUIStore(s => s.setHoveredMarker);
  const selectedBacktestId = useUIStore(s => s.selectedBacktestId);
  const setSelectedBacktestId = useUIStore(s => s.setSelectedBacktestId);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);
  const setChartOverrideRange = useUIStore(s => s.setChartOverrideRange);
  const selectedSample = useUIStore(s => s.selectedSample);
  const setSelectedSample = useUIStore(s => s.setSelectedSample);
  const backtestForm = useUIStore(s => s.backtestForm);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);
  const paperForm = useUIStore(s => s.paperForm);
  const setPaperForm = useUIStore(s => s.setPaperForm);
  const liveAccountForm = useUIStore(s => s.liveAccountForm);
  const setLiveAccountForm = useUIStore(s => s.setLiveAccountForm);
  const liveBindingForm = useUIStore(s => s.liveBindingForm);
  const setLiveBindingForm = useUIStore(s => s.setLiveBindingForm);
  const liveOrderForm = useUIStore(s => s.liveOrderForm);
  const setLiveOrderForm = useUIStore(s => s.setLiveOrderForm);
  const liveSessionForm = useUIStore(s => s.liveSessionForm);
  const setLiveSessionForm = useUIStore(s => s.setLiveSessionForm);
  const accountSignalForm = useUIStore(s => s.accountSignalForm);
  const setAccountSignalForm = useUIStore(s => s.setAccountSignalForm);
  const strategySignalForm = useUIStore(s => s.strategySignalForm);
  const setStrategySignalForm = useUIStore(s => s.setStrategySignalForm);
  const strategyCreateForm = useUIStore(s => s.strategyCreateForm);
  const setStrategyCreateForm = useUIStore(s => s.setStrategyCreateForm);
  const strategyEditorForm = useUIStore(s => s.strategyEditorForm);
  const setStrategyEditorForm = useUIStore(s => s.setStrategyEditorForm);
  const signalRuntimeForm = useUIStore(s => s.signalRuntimeForm);
  const setSignalRuntimeForm = useUIStore(s => s.setSignalRuntimeForm);
  const runtimePolicyForm = useUIStore(s => s.runtimePolicyForm);
  const setRuntimePolicyForm = useUIStore(s => s.setRuntimePolicyForm);
  const telegramForm = useUIStore(s => s.telegramForm);
  const setTelegramForm = useUIStore(s => s.setTelegramForm);
  const liveAccountError = useUIStore(s => s.liveAccountError);
  const setLiveAccountError = useUIStore(s => s.setLiveAccountError);
  const liveBindingError = useUIStore(s => s.liveBindingError);
  const setLiveBindingError = useUIStore(s => s.setLiveBindingError);
  const liveSessionError = useUIStore(s => s.liveSessionError);
  const setLiveSessionError = useUIStore(s => s.setLiveSessionError);
  const liveAccountNotice = useUIStore(s => s.liveAccountNotice);
  const setLiveAccountNotice = useUIStore(s => s.setLiveAccountNotice);
  const liveBindingNotice = useUIStore(s => s.liveBindingNotice);
  const setLiveBindingNotice = useUIStore(s => s.setLiveBindingNotice);
  const liveSessionNotice = useUIStore(s => s.liveSessionNotice);
  const setLiveSessionNotice = useUIStore(s => s.setLiveSessionNotice);
  const settingsMenuOpen = useUIStore(s => s.settingsMenuOpen);
  const setSettingsMenuOpen = useUIStore(s => s.setSettingsMenuOpen);
  const activeSettingsModal = useUIStore(s => s.activeSettingsModal);
  const setActiveSettingsModal = useUIStore(s => s.setActiveSettingsModal);

  const summaries = useTradingStore(s => s.summaries);
  const setSummaries = useTradingStore(s => s.setSummaries);
  const accounts = useTradingStore(s => s.accounts);
  const setAccounts = useTradingStore(s => s.setAccounts);
  const orders = useTradingStore(s => s.orders);
  const setOrders = useTradingStore(s => s.setOrders);
  const fills = useTradingStore(s => s.fills);
  const setFills = useTradingStore(s => s.setFills);
  const positions = useTradingStore(s => s.positions);
  const setPositions = useTradingStore(s => s.setPositions);
  const snapshots = useTradingStore(s => s.snapshots);
  const setSnapshots = useTradingStore(s => s.setSnapshots);
  const strategies = useTradingStore(s => s.strategies);
  const setStrategies = useTradingStore(s => s.setStrategies);
  const backtests = useTradingStore(s => s.backtests);
  const setBacktests = useTradingStore(s => s.setBacktests);
  const backtestOptions = useTradingStore(s => s.backtestOptions);
  const setBacktestOptions = useTradingStore(s => s.setBacktestOptions);
  const paperSessions = useTradingStore(s => s.paperSessions);
  const setPaperSessions = useTradingStore(s => s.setPaperSessions);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const setLiveSessions = useTradingStore(s => s.setLiveSessions);
  const liveAdapters = useTradingStore(s => s.liveAdapters);
  const setLiveAdapters = useTradingStore(s => s.setLiveAdapters);
  const signalCatalog = useTradingStore(s => s.signalCatalog);
  const setSignalCatalog = useTradingStore(s => s.setSignalCatalog);
  const signalSourceTypes = useTradingStore(s => s.signalSourceTypes);
  const setSignalSourceTypes = useTradingStore(s => s.setSignalSourceTypes);
  const signalRuntimeAdapters = useTradingStore(s => s.signalRuntimeAdapters);
  const setSignalRuntimeAdapters = useTradingStore(s => s.setSignalRuntimeAdapters);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const setSignalRuntimeSessions = useTradingStore(s => s.setSignalRuntimeSessions);
  const runtimePolicy = useTradingStore(s => s.runtimePolicy);
  const setRuntimePolicy = useTradingStore(s => s.setRuntimePolicy);
  const alerts = useTradingStore(s => s.alerts);
  const setAlerts = useTradingStore(s => s.setAlerts);
  const notifications = useTradingStore(s => s.notifications);
  const setNotifications = useTradingStore(s => s.setNotifications);
  const telegramConfig = useTradingStore(s => s.telegramConfig);
  const setTelegramConfig = useTradingStore(s => s.setTelegramConfig);
  const accountSignalBindings = useTradingStore(s => s.accountSignalBindings);
  const setAccountSignalBindings = useTradingStore(s => s.setAccountSignalBindings);
  const strategySignalBindings = useTradingStore(s => s.strategySignalBindings);
  const setStrategySignalBindings = useTradingStore(s => s.setStrategySignalBindings);
  const accountSignalBindingMap = useTradingStore(s => s.accountSignalBindingMap);
  const setAccountSignalBindingMap = useTradingStore(s => s.setAccountSignalBindingMap);
  const strategySignalBindingMap = useTradingStore(s => s.strategySignalBindingMap);
  const setStrategySignalBindingMap = useTradingStore(s => s.setStrategySignalBindingMap);
  const signalRuntimePlan = useTradingStore(s => s.signalRuntimePlan);
  const setSignalRuntimePlan = useTradingStore(s => s.setSignalRuntimePlan);
  const selectedSignalRuntimeId = useTradingStore(s => s.selectedSignalRuntimeId);
  const setSelectedSignalRuntimeId = useTradingStore(s => s.setSelectedSignalRuntimeId);
  const selectedStrategyId = useTradingStore(s => s.selectedStrategyId);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);
  const candles = useTradingStore(s => s.candles);
  const setCandles = useTradingStore(s => s.setCandles);
  const monitorCandles = useTradingStore(s => s.monitorCandles);
  const setMonitorCandles = useTradingStore(s => s.setMonitorCandles);
  const annotations = useTradingStore(s => s.annotations);
  const setAnnotations = useTradingStore(s => s.setAnnotations);
  const editingLiveSessionId = useTradingStore(s => s.editingLiveSessionId);
  const setEditingLiveSessionId = useTradingStore(s => s.setEditingLiveSessionId);

  const liveAccounts = accounts;

  // --- Derived State ---

  const highlightedLiveSession = useMemo(
    () => deriveHighlightedLiveSession(liveSessions, orders, fills, positions),
    [liveSessions, orders, fills, positions]
  );

  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";

  const quickLiveAccountId = liveSessionForm.accountId || liveBindingForm.accountId || liveAccounts[0]?.id || "";

  const strategyIds = useMemo(() => new Set(strategies.map((item) => item.id)), [strategies]);
  
  const strategyOptions = useMemo(
    () =>
      strategies.map((strategy) => ({
        value: strategy.id,
        label: strategyLabel(strategy),
      })),
    [strategies]
  );

  const validLiveSessions = useMemo(
    () => liveSessions.filter((item) => strategyIds.has(item.strategyId)),
    [liveSessions, strategyIds]
  );

  const quickLiveAccount = useMemo(
    () => liveAccounts.find((a) => a.id === quickLiveAccountId) || null,
    [liveAccounts, quickLiveAccountId]
  );

  const selectedLiveOrderPreflight = useMemo(
    () => summarizeOrderPreflight({ liveOrderForm, liveAccounts, signalRuntimeSessions, runtimePolicy }),
    [liveOrderForm, liveAccounts, signalRuntimeSessions, runtimePolicy]
  );

  const selectedLiveOrderActiveRuntime = useMemo(() => {
    const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === liveOrderForm.accountId);
    return runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
  }, [signalRuntimeSessions, liveOrderForm.accountId]);

  const selectedLiveOrderRuntimeState = getRecord(selectedLiveOrderActiveRuntime?.state);
  const selectedLiveOrderRuntimeSummary = getRecord(selectedLiveOrderRuntimeState.lastEventSummary);
  const selectedLiveOrderMarket = deriveRuntimeMarketSnapshot(getRecord(selectedLiveOrderRuntimeState.sourceStates), selectedLiveOrderRuntimeSummary);
  const selectedLiveOrderSignalBarState = derivePrimarySignalBarState(getRecord(selectedLiveOrderRuntimeState.signalBarStates));
  const selectedLiveOrderSignalAction = deriveSignalActionSummary(selectedLiveOrderSignalBarState);

  // --- Helpers ---

  function normalizeStrategyId(candidate: string, fallback = "") {
    return strategyIds.has(candidate) ? candidate : fallback;
  }

  function selectQuickLiveAccount(accountId: string) {
    setLiveBindingForm((current) => ({ ...current, accountId }));
    setLiveSessionForm((current) => ({ ...current, accountId }));
    setLiveOrderForm((current) => ({ ...current, accountId }));
    setAccountSignalForm((current) => ({ ...current, accountId }));
    setSignalRuntimeForm((current) => ({ ...current, accountId }));
  }

  function openLiveAccountModal() {
    const baseName = "Binance Testnet";
    const existingNames = new Set(liveAccounts.map((item) => item.name));
    let nextName = baseName;
    let suffix = 2;
    while (existingNames.has(nextName)) {
      nextName = `${baseName} ${suffix}`;
      suffix += 1;
    }
    setLiveAccountForm((current) => ({
      ...current,
      name: current.name.trim() === "" || existingNames.has(current.name) ? nextName : current.name,
      exchange: current.exchange || "binance-futures",
    }));
    setError(null);
    setLiveAccountError(null);
    setLiveAccountNotice(null);
    setActiveSettingsModal("live-account");
  }

  function openLiveBindingModal() {
    if (quickLiveAccountId) {
      selectQuickLiveAccount(quickLiveAccountId);
    }
    setError(null);
    setLiveBindingError(null);
    setLiveBindingNotice(null);
    setActiveSettingsModal("live-binding");
  }

  function openLiveSessionModal(session?: LiveSession | null) {
    const nextAccountId = session?.accountId || quickLiveAccountId || liveAccounts[0]?.id || "";
    const nextStrategyId = normalizeStrategyId(session?.strategyId ?? "", strategies[0]?.id || "");
    const sessionState = getRecord(session?.state);
    if (nextAccountId) {
      selectQuickLiveAccount(nextAccountId);
    }
    setLiveSessionForm((current) => ({
      ...current,
      accountId: nextAccountId || current.accountId,
      strategyId: nextStrategyId,
      signalTimeframe: String(sessionState.signalTimeframe ?? current.signalTimeframe ?? "1d"),
      executionDataSource: String(sessionState.executionDataSource ?? current.executionDataSource ?? "tick"),
      symbol: String(sessionState.symbol ?? current.symbol ?? "BTCUSDT"),
      defaultOrderQuantity: String(sessionState.defaultOrderQuantity ?? current.defaultOrderQuantity ?? "0.001"),
      executionEntryOrderType: String(sessionState.executionEntryOrderType ?? current.executionEntryOrderType ?? "MARKET"),
      executionEntryMaxSpreadBps: String(sessionState.executionEntryMaxSpreadBps ?? current.executionEntryMaxSpreadBps ?? "8"),
      executionEntryWideSpreadMode: String(sessionState.executionEntryWideSpreadMode ?? current.executionEntryWideSpreadMode ?? "limit-maker"),
      executionEntryTimeoutFallbackOrderType: String(
        sessionState.executionEntryTimeoutFallbackOrderType ?? current.executionEntryTimeoutFallbackOrderType ?? "MARKET"
      ),
      executionPTExitOrderType: String(sessionState.executionPTExitOrderType ?? current.executionPTExitOrderType ?? "LIMIT"),
      executionPTExitTimeInForce: String(sessionState.executionPTExitTimeInForce ?? current.executionPTExitTimeInForce ?? "GTX"),
      executionPTExitPostOnly: Boolean(sessionState.executionPTExitPostOnly ?? current.executionPTExitPostOnly),
      executionPTExitTimeoutFallbackOrderType: String(
        sessionState.executionPTExitTimeoutFallbackOrderType ?? current.executionPTExitTimeoutFallbackOrderType ?? "MARKET"
      ),
      executionSLExitOrderType: String(sessionState.executionSLExitOrderType ?? current.executionSLExitOrderType ?? "MARKET"),
      executionSLExitMaxSpreadBps: String(sessionState.executionSLExitMaxSpreadBps ?? current.executionSLExitMaxSpreadBps ?? "999"),
      dispatchMode: String(sessionState.dispatchMode ?? current.dispatchMode ?? "manual-review"),
      dispatchCooldownSeconds: String(sessionState.dispatchCooldownSeconds ?? current.dispatchCooldownSeconds ?? "30"),
    }));
    setEditingLiveSessionId(session?.id ?? null);
    setError(null);
    setLiveSessionError(null);
    setLiveSessionNotice(null);
    setActiveSettingsModal("live-session");
  }

  // --- Handlers ---

  async function loadDashboard() {
    const [
      summaryData,
      accountData,
      ordersData,
      fillsData,
      positionsData,
      paperSessionData,
      liveSessionData,
      strategyData,
      backtestData,
      backtestOptionsData,
      liveAdapterData,
      signalCatalogData,
      signalSourceTypeData,
      signalRuntimeAdapterData,
      signalRuntimeSessionData,
      runtimePolicyData,
      alertData,
      notificationData,
      telegramConfigData,
    ] = await Promise.all([
      fetchJSON<AccountSummary[]>("/api/v1/account-summaries"),
      fetchJSON<AccountRecord[]>("/api/v1/accounts"),
      fetchJSON<Order[]>("/api/v1/orders"),
      fetchJSON<Fill[]>("/api/v1/fills"),
      fetchJSON<Position[]>("/api/v1/positions"),
      Promise.resolve([] as PaperSession[]),
      fetchJSON<LiveSession[]>("/api/v1/live/sessions"),
      fetchJSON<StrategyRecord[]>("/api/v1/strategies"),
      fetchJSON<BacktestRun[]>("/api/v1/backtests"),
      fetchJSON<BacktestOptions>("/api/v1/backtests/options"),
      fetchJSON<LiveAdapter[]>("/api/v1/live-adapters"),
      fetchJSON<SignalSourceCatalog>("/api/v1/signal-sources"),
      fetchJSON<SignalSourceType[]>("/api/v1/signal-source-types"),
      fetchJSON<SignalRuntimeAdapter[]>("/api/v1/signal-runtime/adapters"),
      fetchJSON<SignalRuntimeSession[]>("/api/v1/signal-runtime/sessions"),
      fetchJSON<RuntimePolicy>("/api/v1/runtime-policy"),
      fetchJSON<PlatformAlert[]>("/api/v1/alerts"),
      fetchJSON<PlatformNotification[]>("/api/v1/notifications?includeAcked=true"),
      fetchJSON<TelegramConfig>("/api/v1/telegram/config"),
    ]);
    const accountBindingEntries = await Promise.all(
      accountData.map(async (account) => [
        account.id,
        await fetchJSON<SignalBinding[]>(`/api/v1/accounts/${account.id}/signal-bindings`),
      ] as const)
    );
    const strategyBindingEntries = await Promise.all(
      strategyData.map(async (strategy) => [
        strategy.id,
        await fetchJSON<SignalBinding[]>(`/api/v1/strategies/${strategy.id}/signal-bindings`),
      ] as const)
    );

    let runtimePlanData = null;
    const selectedRuntimeId = useTradingStore.getState().selectedSignalRuntimeId;
    if (selectedRuntimeId) {
      const session = signalRuntimeSessionData.find(s => s.id === selectedRuntimeId);
      if (session) {
        try {
          runtimePlanData = await fetchJSON<Record<string, unknown>>(
            `/api/v1/signal-runtime/plan?accountId=${encodeURIComponent(session.accountId)}&strategyId=${encodeURIComponent(session.strategyId)}`
          );
        } catch (e) {
          console.warn("Failed to fetch runtime plan", e);
        }
      }
    }


    const anchorDate = resolveChartAnchor(liveSessionData[0] ?? null, ordersData);
    const range = chartOverrideRange ?? buildTimeRange(anchorDate, timeWindow);
    const from = range.from;
    const to = range.to;

    const monitorSessionForChart = liveSessionData[0] ?? null;
    const monitorSignalTimeframe = String(monitorSessionForChart?.state?.signalTimeframe ?? "1d");
    const monitorResolution = monitorSignalTimeframe.toLowerCase() === "4h" ? "240" : "1D";

    const [snapshotData, candleData, monitorCandleData, annotationData] = await Promise.all([
      summaryData[0]?.accountId
        ? fetchJSON<AccountEquitySnapshot[]>(
            `/api/v1/account-equity-snapshots?accountId=${encodeURIComponent(summaryData[0].accountId)}`
          )
        : Promise.resolve([]),
      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=BTCUSDT&resolution=1&from=${from}&to=${to}&limit=840`
      ),
      fetchJSON<{ candles: ChartCandle[] }>(
        `/api/v1/chart/candles?symbol=BTCUSDT&resolution=${encodeURIComponent(monitorResolution)}&limit=400`
      ),
      fetchJSON<ChartAnnotation[]>(
        `/api/v1/chart/annotations?symbol=BTCUSDT&from=${from}&to=${to}&limit=300`
      ),
    ]);

    const normalizedSummaries = Array.isArray(summaryData) ? summaryData : [];
    const normalizedAccounts = Array.isArray(accountData) ? accountData : [];
    const normalizedOrders = Array.isArray(ordersData) ? ordersData : [];
    const normalizedFills = Array.isArray(fillsData) ? fillsData : [];
    const normalizedPositions = Array.isArray(positionsData) ? positionsData : [];
    const normalizedPaperSessions = Array.isArray(paperSessionData) ? paperSessionData : [];
    const normalizedLiveSessions = Array.isArray(liveSessionData) ? liveSessionData : [];
    const normalizedStrategies = Array.isArray(strategyData) ? strategyData : [];
    const normalizedBacktests = Array.isArray(backtestData) ? backtestData : [];
    const normalizedLiveAdapters = Array.isArray(liveAdapterData) ? liveAdapterData : [];
    const normalizedSignalSourceTypes = Array.isArray(signalSourceTypeData) ? signalSourceTypeData : [];
    const normalizedSignalRuntimeAdapters = Array.isArray(signalRuntimeAdapterData) ? signalRuntimeAdapterData : [];
    const normalizedSignalRuntimeSessions = Array.isArray(signalRuntimeSessionData) ? signalRuntimeSessionData : [];
    const normalizedAlerts = Array.isArray(alertData) ? alertData : [];
    const normalizedNotifications = Array.isArray(notificationData) ? notificationData : [];
    const normalizedSnapshots = Array.isArray(snapshotData) ? snapshotData : [];
    const normalizedAnnotations = Array.isArray(annotationData) ? annotationData : [];
    const normalizedCandles = Array.isArray(candleData?.candles) ? candleData.candles : [];
    const normalizedMonitorCandles = Array.isArray(monitorCandleData?.candles) ? monitorCandleData.candles : [];
    const normalizedSignalCatalog = signalCatalogData && typeof signalCatalogData === "object" ? signalCatalogData : { sources: [], notes: [] };
    const normalizedBacktestOptions =
      backtestOptionsData && typeof backtestOptionsData === "object" ? backtestOptionsData : ({} as BacktestOptions);

    setSummaries(normalizedSummaries);
    setAccounts(normalizedAccounts);
    setOrders(normalizedOrders);
    setFills(normalizedFills);
    setPositions(normalizedPositions);
    setSnapshots(normalizedSnapshots);
    setMonitorCandles(normalizedMonitorCandles);
    setStrategies(normalizedStrategies);
    setSelectedStrategyId((current) => {
      if (current && normalizedStrategies.some((item) => item.id === current)) {
        return current;
      }
      return normalizedStrategies[0]?.id ?? null;
    });
    setBacktests(normalizedBacktests);
    setSelectedBacktestId((current) => {
      if (current && normalizedBacktests.some((item) => item.id === current)) {
        return current;
      }
      return normalizedBacktests.length > 0 ? normalizedBacktests[normalizedBacktests.length - 1].id : null;
    });
    setBacktestOptions(normalizedBacktestOptions);
    setPaperSessions(normalizedPaperSessions);
    setLiveSessions(normalizedLiveSessions);
    setLiveAdapters(normalizedLiveAdapters);
    setSignalCatalog(normalizedSignalCatalog as SignalSourceCatalog);
    setSignalSourceTypes(normalizedSignalSourceTypes);
    setSignalRuntimeAdapters(normalizedSignalRuntimeAdapters);
    setSignalRuntimeSessions(normalizedSignalRuntimeSessions);
    setRuntimePolicy(runtimePolicyData);
    setAlerts(normalizedAlerts);
    setNotifications(normalizedNotifications);
    setTelegramConfig(telegramConfigData);
    
    // Only update telegramForm from backend if user is NOT currently looking at the modal
    // to avoid wiping out their in-progress typing.
    const currentActiveModal = useUIStore.getState().activeSettingsModal;
    if (currentActiveModal !== "telegram") {
      setTelegramForm({
        enabled: Boolean(telegramConfigData.enabled),
        botToken: "",
        chatId: String(telegramConfigData.chatId ?? ""),
        sendLevels: (telegramConfigData.sendLevels ?? []).join(",") || "critical,warning",
      });
    }
    setRuntimePolicyForm({
      tradeTickFreshnessSeconds: String(runtimePolicyData.tradeTickFreshnessSeconds ?? 15),
      orderBookFreshnessSeconds: String(runtimePolicyData.orderBookFreshnessSeconds ?? 10),
      signalBarFreshnessSeconds: String(runtimePolicyData.signalBarFreshnessSeconds ?? 30),
      runtimeQuietSeconds: String(runtimePolicyData.runtimeQuietSeconds ?? 30),
      paperStartReadinessTimeoutSeconds: String(runtimePolicyData.paperStartReadinessTimeoutSeconds ?? 5),
    });
    setAccountSignalBindingMap(Object.fromEntries(accountBindingEntries));
    setStrategySignalBindingMap(Object.fromEntries(strategyBindingEntries));
    setSignalRuntimePlan(runtimePlanData);
    setSelectedSignalRuntimeId((current) => {
      if (current && normalizedSignalRuntimeSessions.some((item) => item.id === current)) {
        return current;
      }
      return normalizedSignalRuntimeSessions[0]?.id ?? null;
    });
    setCandles(normalizedCandles);
    setAnnotations(normalizedAnnotations);
    setBacktestForm((current) => ({
      strategyVersionId: current.strategyVersionId || normalizedStrategies[0]?.currentVersion?.id || "",
      signalTimeframe: current.signalTimeframe || normalizedBacktestOptions.defaultSignalTimeframe,
      executionDataSource: current.executionDataSource || normalizedBacktestOptions.defaultExecutionDataSource,
      symbol: current.symbol || "BTCUSDT",
      from: current.from || "",
      to: current.to || "",
    }));
    setPaperForm((current) => current);
    const strategyIDSet = new Set(normalizedStrategies.map((item) => item.id));
    const normalizeLoadedStrategyId = (candidate: string, fallback = "") =>
      candidate && strategyIDSet.has(candidate) ? candidate : fallback;
    setLiveBindingForm((current) => ({
      accountId: current.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      adapterKey: current.adapterKey || normalizedLiveAdapters[0]?.key || "binance-futures",
      positionMode: current.positionMode || "ONE_WAY",
      marginMode: current.marginMode || "CROSSED",
      sandbox: current.sandbox,
      apiKeyRef: current.apiKeyRef,
      apiSecretRef: current.apiSecretRef,
    }));
    setLiveOrderForm((current) => ({
      accountId: current.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      strategyVersionId: current.strategyVersionId || normalizedStrategies[0]?.currentVersion?.id || "",
      symbol: current.symbol || "BTCUSDT",
      side: current.side || "BUY",
      type: current.type || "LIMIT",
      quantity: current.quantity || "0.001",
      price: current.price || "",
    }));
    setLiveSessionForm((current) => ({
      ...current,
      accountId: current.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      strategyId: normalizeLoadedStrategyId(current.strategyId, normalizedStrategies[0]?.id || ""),
          signalTimeframe: current.signalTimeframe || "1d",
          executionDataSource: current.executionDataSource || "tick",
          symbol: current.symbol || "BTCUSDT",
          defaultOrderQuantity: current.defaultOrderQuantity || "0.001",
          dispatchMode: current.dispatchMode || "manual-review",
          dispatchCooldownSeconds: current.dispatchCooldownSeconds || "30",
        }));
    const availableSignalSources = (normalizedSignalCatalog as SignalSourceCatalog).sources ?? [];
    setAccountSignalForm((current) => ({
      accountId: current.accountId || normalizedSummaries[0]?.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      sourceKey: current.sourceKey || availableSignalSources[0]?.key || "",
      role: current.role || "trigger",
      symbol: current.symbol || "BTCUSDT",
      timeframe: current.timeframe || "1d",
    }));
    setStrategySignalForm((current) => ({
      strategyId: normalizeLoadedStrategyId(current.strategyId, normalizedStrategies[0]?.id || ""),
      sourceKey: current.sourceKey || availableSignalSources[0]?.key || "",
      role: current.role || "trigger",
      symbol: current.symbol || "BTCUSDT",
      timeframe: current.timeframe || "1d",
    }));
    setStrategyCreateForm((current) => ({
      name: current.name || "",
      description: current.description || "",
    }));
    setSignalRuntimeForm((current) => ({
      accountId: current.accountId || normalizedSummaries[0]?.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      strategyId: normalizeLoadedStrategyId(current.strategyId, normalizedStrategies[0]?.id || ""),
    }));
  }

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
      setStrategyCreateForm({ name: "", description: "" });
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
      const boundAccount =
        accounts.find((item) => item.id === liveBindingForm.accountId) ??
        quickLiveAccount ??
        null;
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

  async function stopLiveFlow(accountId: string) {
    setLiveFlowAction(accountId);
    try {
      await fetchJSON(`/api/v1/live/accounts/${accountId}/stop`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to stop live flow");
    } finally {
      setLiveFlowAction(null);
    }
  }

  async function unbindAccountSignalSource(accountId: string, bindingId: string) {
    try {
      await fetchJSON(`/api/v1/accounts/${accountId}/signal-bindings?id=${encodeURIComponent(bindingId)}`, { method: "DELETE" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to unbind account signal source");
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

  async function deleteSignalRuntimeSession(sessionId: string) {
    if (!window.confirm("确定要彻底删除该信号运行时会话吗？(将停止运行中的流)")) return;
    try {
      await fetchJSON(`/api/v1/signal-runtime/sessions/${sessionId}`, { method: "DELETE" });
      if (sessionId === selectedSignalRuntimeId) {
        setSelectedSignalRuntimeId(null);
        setSignalRuntimePlan(null);
      }
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete signal runtime session");
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
    setLiveSessionForm((current) => ({ ...current, strategyId: normalizedStrategyId }));
    setLiveSessionCreateAction(true);
    try {
      const created = await fetchJSON<LiveSession>("/api/v1/live/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: liveSessionForm.accountId,
          strategyId: normalizedStrategyId,
          signalTimeframe: liveSessionForm.signalTimeframe,
          executionDataSource: liveSessionForm.executionDataSource,
          symbol: liveSessionForm.symbol,
          defaultOrderQuantity: Number(liveSessionForm.defaultOrderQuantity) || 0.001,
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
      setLiveSessionForm((current) => ({ ...current, strategyId: normalizedStrategyId }));
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
          signalTimeframe: liveSessionForm.signalTimeframe,
          executionDataSource: liveSessionForm.executionDataSource,
          symbol: liveSessionForm.symbol,
          defaultOrderQuantity: Number(liveSessionForm.defaultOrderQuantity) || 0.001,
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
      if (!created?.id) {
        return;
      }
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

  async function deleteLiveSession(sessionId: string) {
    setLiveSessionDeleteAction(sessionId);
    try {
      await fetchJSON(`/api/v1/live/sessions/${sessionId}`, { method: "DELETE" });
      await loadDashboard();
      if (activeSettingsModal === "live-session") {
        setLiveSessionNotice(`已删除会话：${sessionId}`);
      }
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to delete live session");
    } finally {
      setLiveSessionDeleteAction(null);
    }
  }

  async function launchLiveFlow(account: AccountRecord) {
    const strategyId =
      validLiveSessions.find((item) => item.accountId === account.id)?.strategyId ||
      liveSessionForm.strategyId ||
      strategies[0]?.id ||
      "";
    if (!strategyId) {
      setError("Launch live flow needs a strategy");
      return;
    }

    setLiveFlowAction(account.id);
    setError(null);
    setLiveBindingForm((current) => ({ ...current, accountId: account.id }));
    setLiveSessionForm((current) => ({ ...current, accountId: account.id, strategyId }));
    setSignalRuntimeForm((current) => ({ ...current, accountId: account.id, strategyId }));
    setAccountSignalForm((current) => ({ ...current, accountId: account.id }));
    setStrategySignalForm((current) => ({ ...current, strategyId }));

    try {
      const strategyBindings = strategySignalBindingMap[strategyId] ?? [];
      if ((accountSignalBindingMap[account.id] ?? []).length === 0 && strategyBindings.length === 0) {
        window.location.hash = "signals";
        throw new Error("Launch live flow needs strategy signal bindings before it can continue");
      }
      await fetchJSON(`/api/v1/live/accounts/${account.id}/launch`, {
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
          startRuntime: true,
          startSession: true,
          liveSessionOverrides: {
            signalTimeframe: liveSessionForm.signalTimeframe,
            executionDataSource: liveSessionForm.executionDataSource,
            symbol: liveSessionForm.symbol,
            defaultOrderQuantity: Number(liveSessionForm.defaultOrderQuantity) || 0.001,
            dispatchMode: liveSessionForm.dispatchMode,
            dispatchCooldownSeconds: Number(liveSessionForm.dispatchCooldownSeconds) || 30,
          },
        }),
      });

      await loadDashboard();
      window.location.hash = "live";
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to launch live flow");
    } finally {
      setLiveFlowAction(null);
    }
  }

  async function runLiveSessionAction(sessionId: string, action: "start" | "stop") {
    try {
      setLiveSessionAction(`${sessionId}:${action}`);
      setError(null);
      await fetchJSON(`/api/v1/live/sessions/${sessionId}/${action}`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      const message = err instanceof Error ? err.message : "Failed to execute live session action";
      setError(message);
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
      await fetchJSON(`/api/v1/live/sessions/${sessionId}/dispatch`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to dispatch live session intent");
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

  async function bindAccountSignalSource() {
    if (!accountSignalForm.accountId || !accountSignalForm.sourceKey) {
      setError("Account signal binding needs an account and source");
      return;
    }
    setSignalBindingAction("account");
    try {
      await fetchJSON(`/api/v1/accounts/${accountSignalForm.accountId}/signal-bindings`, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          sourceKey: accountSignalForm.sourceKey,
          role: accountSignalForm.role,
          symbol: accountSignalForm.symbol,
          options: accountSignalForm.role === "signal" ? { timeframe: accountSignalForm.timeframe } : undefined,
        }),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to bind account signal source");
    } finally {
      setSignalBindingAction(null);
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
        paperStartReadinessTimeoutSeconds: String(
          updated.paperStartReadinessTimeoutSeconds ?? payload.paperStartReadinessTimeoutSeconds
        ),
      });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to update runtime policy");
    } finally {
      setRuntimePolicyAction(false);
    }
  }

  async function runSignalRuntimeAction(sessionId: string, action: "start" | "stop") {
    setSignalRuntimeAction(`${sessionId}:${action}`);
    try {
      await fetchJSON(`/api/v1/signal-runtime/sessions/${sessionId}/${action}`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to execute signal runtime action");
    } finally {
      setSignalRuntimeAction(null);
    }
  }

  function runLiveNextAction(account: AccountRecord, action: LiveNextAction, activeRuntime: SignalRuntimeSession | null) {
    switch (action.key) {
      case "bind-live-adapter":
        setLiveBindingForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "live";
        break;
      case "bind-signals":
        setAccountSignalForm((current) => ({ ...current, accountId: account.id }));
        setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "create-runtime":
        setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "start-runtime":
      case "inspect-runtime":
        if (activeRuntime) {
          jumpToSignalRuntimeSession(activeRuntime.id);
        } else {
          setSignalRuntimeForm((current) => ({ ...current, accountId: account.id }));
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
    window.location.hash = "signals";
  }

  function jumpToAlert(alert: PlatformAlert) {
    switch (alert.anchor) {
      case "signals":
        if (alert.runtimeSessionId) {
          jumpToSignalRuntimeSession(alert.runtimeSessionId);
          return;
        }
        window.location.hash = "signals";
        return;
      case "paper":
        window.location.hash = "paper";
        return;
      case "live":
        window.location.hash = "live";
        return;
      default:
        window.location.hash = alert.anchor || "";
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
        }),
      });
      setTelegramConfig(updated);
      setTelegramForm((current) => ({ 
        ...current, 
        enabled: Boolean(updated.enabled),
        chatId: String(updated.chatId ?? ""),
        botToken: "" 
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
      setLoginForm((current) => ({ ...current, password: "" }));
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
    setLiveSessions([]);
    setError(null);
    setLoading(false);
  }

  useEffect(() => {
    let active = true;

    async function load() {
      if (!authSession?.token) {
        setLoading(false);
        return;
      }
      try {
        await loadDashboard();
        if (!active) {
          return;
        }
        setError(null);
      } catch (err) {
        if (!active) {
          return;
        }
        if (typeof err === "object" && err && "status" in err && (err as { status?: number }).status === 401) {
          writeStoredAuthSession(null);
          setAuthSession(null);
          setError("登录已失效，请重新登录");
          return;
        }
        setError(err instanceof Error ? err.message : "Failed to load monitoring data");
      } finally {
        if (active) {
          setLoading(false);
        }
      }
    }

    load();
    const timer = window.setInterval(load, 5000);
    return () => {
      active = false;
      window.clearInterval(timer);
    };
  }, [authSession?.token, timeWindow, chartOverrideRange]);

  return (
    <>
    <WorkbenchLayout
      sidebarTab={sidebarTab}
      onSidebarTabChange={setSidebarTab}
      dockTab={dockTab}
      onDockTabChange={setDockTab}
      headerMetrics={
        <div className="flex space-x-2">
          <MetricCard label="账户" value={monitorMode} />
          <MetricCard label="策略" value={String(highlightedLiveSession?.session?.strategyId ?? "--")} />
          <MetricCard label="实盘状态" value={highlightedLiveSession?.health.status ?? "--"} tone={highlightedLiveSession?.health.status === "ready" ? "accent" : undefined} />
        </div>
      }
      headerConnection={
        <div 
          className={`flex items-center space-x-2 ${error ? 'cursor-pointer hover:bg-white/5 px-2 py-1 rounded transition-colors' : ''}`}
          onClick={() => { if (error) setError(null); }}
          title={error || undefined}
        >
          <span className={!authSession?.token || error ? "w-2 h-2 rounded-full bg-rose-500" : "w-2 h-2 rounded-full bg-emerald-500"} />
          <span className="text-zinc-400 text-xs truncate max-w-[200px]">
            {!authSession?.token ? "需要登录" : error ? `连接异常: ${error}` : "运行正常"}
          </span>
        </div>
      }
      sidePanelContent={
        sidebarTab === 'strategy' ? (
          <StrategySidePanel createBacktestRun={createBacktestRun} />
        ) : sidebarTab === 'account' ? (
          <AccountSidePanel />
        ) : null
      }
      dockContent={
        <div className="h-full">
          <div style={{ display: dockTab === 'orders' ? 'block' : 'none' }} className="h-full p-4">
            <OrdersPanel
              liveOrderForm={liveOrderForm}
              setLiveOrderForm={setLiveOrderForm}
              liveAccounts={liveAccounts}
              strategies={strategies}
              selectedLiveOrderPreflight={selectedLiveOrderPreflight}
              liveOrderAction={liveOrderAction}
              createLiveOrder={createLiveOrder}
              selectedLiveOrderActiveRuntime={selectedLiveOrderActiveRuntime}
              selectedLiveOrderRuntimeState={selectedLiveOrderRuntimeState}
              selectedLiveOrderSignalAction={selectedLiveOrderSignalAction}
              selectedLiveOrderMarket={selectedLiveOrderMarket}
              selectedLiveOrderSignalBarState={selectedLiveOrderSignalBarState}
              selectedLiveOrderRuntimeSummary={selectedLiveOrderRuntimeSummary}
              orders={orders}
            />
          </div>
          <div style={{ display: dockTab === 'positions' ? 'block' : 'none' }} className="h-full p-4 space-y-6">
            <PositionsPanel />
          </div>
          <div style={{ display: dockTab === 'fills' ? 'block' : 'none' }} className="h-full p-4">
            <FillsPanel />
          </div>
          <div style={{ display: dockTab === 'alerts' ? 'block' : 'none' }} className="h-full p-4 space-y-6">
            <AlertsPanel
              alerts={alerts}
              jumpToAlert={jumpToAlert}
              notifications={notifications}
              notificationAction={notificationAction}
              telegramAction={telegramAction}
              acknowledgeNotification={acknowledgeNotification}
              telegramConfig={telegramConfig}
              sendNotificationToTelegram={sendNotificationToTelegram}
            />
          </div>
        </div>
      }
      mainStageContent={
        sidebarTab === 'monitor' ? (
          <MonitorStage />
        ) :
 sidebarTab === 'strategy' ? (
   <StrategyStage
     createStrategy={createStrategy}
     saveStrategyParameters={saveStrategyParameters}
   />
 ) :
         (
          <AccountStage
            logout={logout}
            openLiveAccountModal={openLiveAccountModal}
            openLiveBindingModal={openLiveBindingModal}
            openLiveSessionModal={openLiveSessionModal}
            launchLiveFlow={launchLiveFlow}
            runLiveSessionAction={runLiveSessionAction}
            dispatchLiveSessionIntent={dispatchLiveSessionIntent}
            syncLiveSession={syncLiveSession}
            deleteLiveSession={deleteLiveSession}
            syncLiveAccount={syncLiveAccount}
            syncLiveOrder={syncLiveOrder}
            jumpToSignalRuntimeSession={jumpToSignalRuntimeSession}
            runLiveNextAction={runLiveNextAction}
            selectQuickLiveAccount={selectQuickLiveAccount}
            bindAccountSignalSource={bindAccountSignalSource}
            unbindAccountSignalSource={unbindAccountSignalSource}
            bindStrategySignalSource={bindStrategySignalSource}
            unbindStrategySignalSource={unbindStrategySignalSource}
            updateRuntimePolicy={updateRuntimePolicy}
            createSignalRuntimeSession={createSignalRuntimeSession}
            deleteSignalRuntimeSession={deleteSignalRuntimeSession}
            runSignalRuntimeAction={runSignalRuntimeAction}
          />
        )
      }
    />
        



        <LiveAccountModal
          activeSettingsModal={activeSettingsModal}
          setActiveSettingsModal={setActiveSettingsModal}
          quickLiveAccount={quickLiveAccount}
          liveAccounts={liveAccounts}
          quickLiveAccountId={quickLiveAccountId}
          selectQuickLiveAccount={selectQuickLiveAccount}
          liveAccountError={liveAccountError}
          liveAccountNotice={liveAccountNotice}
          liveAccountForm={liveAccountForm}
          setLiveAccountForm={setLiveAccountForm}
          liveCreateAction={liveCreateAction}
          createLiveAccount={createLiveAccount}
          openLiveBindingModal={openLiveBindingModal}
        />

        <LoginModal
          authSession={authSession}
          error={error}
          loginForm={loginForm}
          loginAction={loginAction}
          setLoginForm={setLoginForm}
          login={login}
        />

        <LiveBindingModal
          activeSettingsModal={activeSettingsModal}
          setActiveSettingsModal={setActiveSettingsModal}
          liveBindingError={liveBindingError}
          liveBindingNotice={liveBindingNotice}
          liveBindingForm={liveBindingForm}
          setLiveBindingForm={setLiveBindingForm}
          liveAccounts={liveAccounts}
          liveAdapters={liveAdapters}
          quickLiveAccount={quickLiveAccount}
          liveBindAction={liveBindAction}
          bindLiveAccount={bindLiveAccount}
        />

        <LiveSessionModal
          activeSettingsModal={activeSettingsModal}
          setActiveSettingsModal={setActiveSettingsModal}
          liveSessionError={liveSessionError}
          liveSessionNotice={liveSessionNotice}
          liveAccounts={liveAccounts}
          liveSessionForm={liveSessionForm}
          setLiveSessionForm={setLiveSessionForm}
          strategies={strategies}
          validLiveSessions={validLiveSessions}
          editingLiveSessionId={editingLiveSessionId}
          strategyOptions={strategyOptions}
          liveSessionCreateAction={liveSessionCreateAction}
          liveSessionLaunchAction={liveSessionLaunchAction}
          liveSessionAction={liveSessionAction}
          saveLiveSession={saveLiveSession}
          createAndStartLiveSession={createAndStartLiveSession}
          setLiveSessionLaunchAction={setLiveSessionLaunchAction}
          setLiveSessionAction={setLiveSessionAction}
          setLiveSessionError={setLiveSessionError}
          loadDashboard={loadDashboard}
          setError={setError}
          fetchJSON={fetchJSON}
        />

        <TelegramModal
          activeSettingsModal={activeSettingsModal}
          setActiveSettingsModal={setActiveSettingsModal}
          telegramConfig={telegramConfig}
          telegramForm={telegramForm}
          setTelegramForm={setTelegramForm}
          telegramAction={telegramAction}
          saveTelegramConfig={saveTelegramConfig}
          sendTelegramTest={sendTelegramTest}
        />

    </>

  );
}


ReactDOM.createRoot(document.getElementById("root")!).render(
  <React.StrictMode>
    <ErrorBoundary>
      <App />
    </ErrorBoundary>
  </React.StrictMode>
);
