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


  const primaryAccount = summaries[0] ?? null;
  const primarySession = paperSessions[0] ?? null;
  const primaryLiveSession = liveSessions[0] ?? null;
  const primarySessionSourceStates = getRecord(primarySession?.state?.lastStrategyEvaluationSourceStates);
  const primarySessionTriggerSource = getRecord(primarySession?.state?.lastStrategyEvaluationTriggerSource);
  const primarySessionSourceGate = getRecord(primarySession?.state?.lastStrategyEvaluationSourceGate);
  const primarySessionDecision = getRecord(primarySession?.state?.lastStrategyDecision);
  const primarySessionDecisionMeta = getRecord(primarySessionDecision.metadata);
  const primarySessionCurrentPosition = getRecord(primarySessionDecisionMeta.currentPosition);
  const primarySessionSignalBarState = getRecord(primarySessionDecisionMeta.signalBarState);
  const primarySessionSignalBarDecision = getRecord(primarySessionDecisionMeta.signalBarDecision);
  const primarySessionTimeline = getList(primarySession?.state?.timeline);
  const paperAccounts = summaries.filter((item) => item.mode === "PAPER");
  const liveAccounts = accounts.filter((item) => item.mode === "LIVE");
  const quickLiveAccountId = liveSessionForm.accountId || liveBindingForm.accountId || liveAccounts[0]?.id || "";
  const quickLiveAccount = liveAccounts.find((item) => item.id === quickLiveAccountId) ?? null;
  const primaryLiveSessionDecision = getRecord(primaryLiveSession?.state?.lastStrategyDecision);
  const primaryLiveSessionDecisionMeta = getRecord(primaryLiveSessionDecision.metadata);
  const primaryLiveSessionSignalBarDecision = getRecord(primaryLiveSessionDecisionMeta.signalBarDecision);
  const primaryLiveSessionIntent = getRecord(primaryLiveSession?.state?.lastStrategyIntent);
  const primaryLiveSessionSourceGate = getRecord(primaryLiveSession?.state?.lastStrategyEvaluationSourceGate);
  const primaryLiveSessionTimeline = getList(primaryLiveSession?.state?.timeline);
  const primaryLiveSessionRuntime =
    signalRuntimeSessions.find((item) => item.id === String(primaryLiveSession?.state?.signalRuntimeSessionId ?? "")) ??
    signalRuntimeSessions.find((item) => item.accountId === primaryLiveSession?.accountId && item.strategyId === primaryLiveSession?.strategyId) ??
    null;
  const primaryLiveSessionRuntimeState = getRecord(primaryLiveSessionRuntime?.state);
  const primaryLiveSessionRuntimeSummary = getRecord(primaryLiveSessionRuntimeState.lastEventSummary);
  const primaryLiveSessionMarket = deriveRuntimeMarketSnapshot(
    getRecord(primaryLiveSessionRuntimeState.sourceStates),
    primaryLiveSessionRuntimeSummary
  );
  const primaryLiveSessionSourceSummary = deriveRuntimeSourceSummary(
    getRecord(primaryLiveSessionRuntimeState.sourceStates),
    runtimePolicy
  );
  const primaryLiveSessionRuntimeReadiness = deriveRuntimeReadiness(
    primaryLiveSessionRuntimeState,
    primaryLiveSessionSourceSummary,
    {
      requireTick: true,
      requireOrderBook: strategySignalBindingMap[primaryLiveSession?.strategyId ?? ""]?.some((item) => item.streamType === "order_book") ?? false,
    }
  );
  const primaryLiveAccount =
    (primaryLiveSession ? liveAccounts.find((item) => item.id === primaryLiveSession.accountId) : null) ?? null;
  const primaryLiveBindings = primaryLiveSession ? accountSignalBindingMap[primaryLiveSession.accountId] ?? [] : [];
  const primaryLiveRuntimeSessions = primaryLiveSession
    ? signalRuntimeSessions.filter((item) => item.accountId === primaryLiveSession.accountId)
    : [];
  const primaryLiveDispatchPreview = deriveLiveDispatchPreview(
    primaryLiveSession,
    primaryLiveAccount,
    primaryLiveBindings,
    primaryLiveRuntimeSessions,
    primaryLiveSessionRuntime,
    primaryLiveSessionRuntimeReadiness,
    primaryLiveSessionIntent
  );
  const primaryLiveExecutionSummary = deriveLiveSessionExecutionSummary(
    primaryLiveSession,
    orders,
    fills,
    positions
  );
  const highlightedLiveSession = useMemo(
    () => deriveHighlightedLiveSession(liveSessions, orders, fills, positions),
    [liveSessions, orders, fills, positions]
  );
  const highlightedLiveRuntime =
    highlightedLiveSession?.session
      ? signalRuntimeSessions.find((item) => item.id === String(highlightedLiveSession.session.state?.signalRuntimeSessionId ?? "")) ??
        signalRuntimeSessions.find(
          (item) =>
            item.accountId === highlightedLiveSession.session.accountId &&
            item.strategyId === highlightedLiveSession.session.strategyId
        ) ??
        null
      : null;
  const highlightedLiveRuntimeState = getRecord(highlightedLiveRuntime?.state);
  const highlightedLiveSessionFlow = useMemo(
    () =>
      highlightedLiveSession
        ? deriveLiveSessionFlow(highlightedLiveSession.session, highlightedLiveSession.execution)
        : [],
    [highlightedLiveSession]
  );
  const primaryPaperAccountBindings = primarySession ? accountSignalBindingMap[primarySession.accountId] ?? [] : [];
  const primaryPaperStrategyBindings = primarySession ? strategySignalBindingMap[primarySession.strategyId] ?? [] : [];
  const primaryLinkedSignalRuntime =
    signalRuntimeSessions.find((item) => item.id === String(primarySession?.state?.signalRuntimeSessionId ?? "")) ??
    signalRuntimeSessions.find((item) => item.accountId === primarySession?.accountId && item.strategyId === primarySession?.strategyId) ??
    null;
  const primaryLinkedSignalRuntimeState = getRecord(primaryLinkedSignalRuntime?.state);
  const primaryLinkedSignalRuntimeSummary = getRecord(primaryLinkedSignalRuntimeState.lastEventSummary);
  const primaryLinkedSignalRuntimeMarket = deriveRuntimeMarketSnapshot(
    getRecord(primaryLinkedSignalRuntimeState.sourceStates),
    primaryLinkedSignalRuntimeSummary
  );
  const primaryLinkedSignalRuntimeSourceSummary = deriveRuntimeSourceSummary(
    getRecord(primaryLinkedSignalRuntimeState.sourceStates),
    runtimePolicy
  );
  const primaryPaperRuntimeReadiness = deriveRuntimeReadiness(
    primaryLinkedSignalRuntimeState,
    primaryLinkedSignalRuntimeSourceSummary,
    {
      requireTick: String(primarySession?.state?.executionDataSource ?? "") === "tick",
      requireOrderBook: primaryPaperStrategyBindings.some((item) => item.streamType === "order_book"),
    }
  );
  const primaryPaperAlerts = derivePaperAlerts(
    primarySession,
    primaryLinkedSignalRuntimeState,
    primaryLinkedSignalRuntimeSourceSummary,
    primaryPaperRuntimeReadiness,
    primarySessionDecision,
    primarySessionDecisionMeta,
    primarySessionSignalBarDecision,
    runtimePolicy
  );
  const monitorSession = highlightedLiveSession?.session ?? null;
  const monitorMode = highlightedLiveSession?.session ? "LIVE" : "--";
  const monitorExecutionSummary = highlightedLiveSession?.execution ?? derivePaperSessionExecutionSummary(null, orders, fills, positions);
  const monitorRuntimeState = highlightedLiveSession?.session ? highlightedLiveRuntimeState : {};
  const monitorSessionState = getRecord(monitorSession?.state);
  const monitorBars = mapChartCandlesToSignalBarCandles(
    monitorCandles,
    String(monitorSessionState.signalTimeframe ?? "1d")
  );
  const monitorSignalState = derivePrimarySignalBarState(
    getRecord(monitorRuntimeState.signalBarStates),
    getRecord(monitorSessionState.lastStrategyEvaluationSignalBarStates)
  );
  const monitorMarket = deriveRuntimeMarketSnapshot(
    getRecord(monitorRuntimeState.sourceStates),
    getRecord(monitorRuntimeState.lastEventSummary)
  );
  const monitorSummary =
    monitorSession ? summaries.find((item) => item.accountId === monitorSession.accountId) ?? null : null;
  const monitorMarkers = deriveSessionMarkers(monitorSession, orders, fills);
  const topPositions = positions.slice(0, 8);
  const topFills = fills.slice().reverse().slice(0, 8);

  function selectQuickLiveAccount(accountId: string) {
    setLiveBindingForm((current: any) => ({ ...current, accountId }));
    setLiveSessionForm((current: any) => ({ ...current, accountId }));
    setLiveOrderForm((current: any) => ({ ...current, accountId }));
    setAccountSignalForm((current: any) => ({ ...current, accountId }));
    setSignalRuntimeForm((current: any) => ({ ...current, accountId }));
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
    setLiveAccountForm((current: any) => ({
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
    setLiveSessionForm((current: any) => ({
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
  function normalizeStrategyId(candidate: string, fallback = "") {
    return strategyIds.has(candidate) ? candidate : fallback;
  }
  const monitorSessionSource: SourceFilter = highlightedLiveSession?.session ? "live" : "all";
  const monitorSessionID = monitorSession?.id;
  const selectedSignalAccount = accountSignalForm.accountId || liveAccounts[0]?.id || "";
  const selectedSignalStrategy = normalizeStrategyId(strategySignalForm.strategyId, strategies[0]?.id || "");
  const selectedRuntimeAccount = signalRuntimeForm.accountId || selectedSignalAccount;
  const selectedRuntimeStrategy = normalizeStrategyId(signalRuntimeForm.strategyId, selectedSignalStrategy);
  const selectedStrategy =
    strategies.find((item) => item.id === (selectedStrategyId || strategyEditorForm.strategyId)) ?? strategies[0] ?? null;
  const selectedStrategyVersion = selectedStrategy?.currentVersion ?? null;
  const selectedStrategyParameters = getRecord(selectedStrategyVersion?.parameters);
  const selectedSignalRuntime =
    signalRuntimeSessions.find((item) => item.id === selectedSignalRuntimeId) ?? signalRuntimeSessions[0] ?? null;
  const selectedSignalRuntimeState = getRecord(selectedSignalRuntime?.state);
  const selectedSignalRuntimePlan = getRecord(selectedSignalRuntimeState.plan);
  const selectedSignalRuntimeLastSummary = getRecord(selectedSignalRuntimeState.lastEventSummary);
  const selectedSignalRuntimeSourceStates = getRecord(selectedSignalRuntimeState.sourceStates);
  const selectedSignalBarStates = getRecord(selectedSignalRuntimeState.signalBarStates);
  const selectedSignalRuntimeTimeline = getList(selectedSignalRuntimeState.timeline);
  const selectedSignalRuntimeSignalBars = deriveSignalBarCandles(selectedSignalRuntimeSourceStates);
  const selectedSignalRuntimeSubscriptions = Array.isArray(selectedSignalRuntimeState.subscriptions)
    ? (selectedSignalRuntimeState.subscriptions as Array<Record<string, unknown>>)
    : [];
  const syncableLiveOrders = orders.filter((item) => item.metadata?.executionMode === "live" && item.status === "ACCEPTED");
  const selectedLiveOrderAccount =
    liveAccounts.find((item) => item.id === liveOrderForm.accountId) ??
    liveAccounts[0] ??
    null;
  const selectedLiveOrderBindings = selectedLiveOrderAccount ? accountSignalBindingMap[selectedLiveOrderAccount.id] ?? [] : [];
  const selectedLiveOrderRuntimeSessions = selectedLiveOrderAccount
    ? signalRuntimeSessions.filter((item) => item.accountId === selectedLiveOrderAccount.id)
    : [];
  const selectedLiveOrderActiveRuntime =
    selectedLiveOrderRuntimeSessions.find((item) => item.status === "RUNNING") ?? selectedLiveOrderRuntimeSessions[0] ?? null;
  const selectedLiveOrderRuntimeState = getRecord(selectedLiveOrderActiveRuntime?.state);
  const selectedLiveOrderSourceSummary = deriveRuntimeSourceSummary(
    getRecord(selectedLiveOrderRuntimeState.sourceStates),
    runtimePolicy
  );
  const selectedLiveOrderReadiness = deriveRuntimeReadiness(selectedLiveOrderRuntimeState, selectedLiveOrderSourceSummary, {
    requireTick: selectedLiveOrderBindings.some((item) => item.streamType === "trade_tick"),
    requireOrderBook: selectedLiveOrderBindings.some((item) => item.streamType === "order_book"),
  });
  const selectedLiveOrderRuntimeSummary = getRecord(selectedLiveOrderRuntimeState.lastEventSummary);
  const selectedLiveOrderMarket = deriveRuntimeMarketSnapshot(
    getRecord(selectedLiveOrderRuntimeState.sourceStates),
    selectedLiveOrderRuntimeSummary
  );
  const selectedLiveOrderSignalBarState = derivePrimarySignalBarState(getRecord(selectedLiveOrderRuntimeState.signalBarStates));
  const selectedLiveOrderSignalAction = deriveSignalActionSummary(selectedLiveOrderSignalBarState);
  const selectedLiveOrderPreflight = selectedLiveOrderAccount
    ? deriveLivePreflightSummary(
        selectedLiveOrderAccount,
        selectedLiveOrderBindings,
        selectedLiveOrderRuntimeSessions,
        selectedLiveOrderActiveRuntime,
        selectedLiveOrderReadiness
      )
    : {
        status: "blocked" as const,
        reason: "no-live-account",
        detail: "create or select a live account first",
      };
  const selectedExecutionAvailability = backtestOptions?.availability?.[backtestForm.executionDataSource] ?? "unknown";
  const selectedExecutionDatasets = backtestOptions?.datasets?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSymbols = backtestOptions?.supportedSymbols?.[backtestForm.executionDataSource] ?? [];
  const selectedExecutionSchema = backtestOptions?.schema?.[backtestForm.executionDataSource];
  const selectedSymbolAvailable =
    selectedExecutionSymbols.length === 0 || selectedExecutionSymbols.includes(backtestForm.symbol.trim().toUpperCase());
  const backtestItems = backtests.slice().reverse().slice(0, 8);
  const selectedBacktest =
    backtests.find((item) => item.id === selectedBacktestId) ??
    (backtests.length > 0 ? backtests[backtests.length - 1] : null);
  const latestBacktestSummary = (selectedBacktest?.resultSummary ?? {}) as Record<string, unknown>;
  const latestExecutionSource = String(latestBacktestSummary.executionDataSource ?? selectedBacktest?.parameters?.executionDataSource ?? "");
  const previewCountLabel = latestExecutionSource === "tick" ? "Preview Ticks" : "Preview Bars";
  const processedCountLabel = latestExecutionSource === "tick" ? "Processed Ticks" : "Processed Bars";
  const processedCountValue =
    latestExecutionSource === "tick"
      ? String(latestBacktestSummary.processedTicks ?? "--")
      : String(latestBacktestSummary.processedBars ?? "--");
  const latestReplayByReason = (latestBacktestSummary.replayLedgerByReason ?? {}) as ReplayReasonStats;
  const latestExecutionTrades = Array.isArray(latestBacktestSummary.executionTrades)
    ? (latestBacktestSummary.executionTrades as ExecutionTrade[])
    : [];
  const latestReplaySkippedSamples = Array.isArray(latestBacktestSummary.replayLedgerSkippedSamples)
    ? (latestBacktestSummary.replayLedgerSkippedSamples as ReplaySample[])
    : [];
  const latestReplayCompletedSamples = Array.isArray(latestBacktestSummary.replayLedgerCompletedSamples)
    ? (latestBacktestSummary.replayLedgerCompletedSamples as ReplaySample[])
    : [];
  const selectableSamples = useMemo<SelectableSample[]>(
    () => [
      ...latestReplayCompletedSamples.map((sample, index) => ({
        key: buildSampleKey("completed", index, sample),
        sample,
        group: "completed" as const,
      })),
      ...latestReplaySkippedSamples.map((sample, index) => ({
        key: buildSampleKey("skipped", index, sample),
        sample,
        group: "skipped" as const,
      })),
    ],
    [latestReplayCompletedSamples, latestReplaySkippedSamples]
  );

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
    setSelectedStrategyId((current: any) => {
      if (current && normalizedStrategies.some((item) => item.id === current)) {
        return current;
      }
      return normalizedStrategies[0]?.id ?? null;
    });
    setBacktests(normalizedBacktests);
    setSelectedBacktestId((current: any) => {
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
    setTelegramForm({
      enabled: Boolean(telegramConfigData.enabled),
      botToken: "",
      chatId: String(telegramConfigData.chatId ?? ""),
      sendLevels: (telegramConfigData.sendLevels ?? []).join(",") || "critical,warning",
    });
    setRuntimePolicyForm({
      tradeTickFreshnessSeconds: String(runtimePolicyData.tradeTickFreshnessSeconds ?? 15),
      orderBookFreshnessSeconds: String(runtimePolicyData.orderBookFreshnessSeconds ?? 10),
      signalBarFreshnessSeconds: String(runtimePolicyData.signalBarFreshnessSeconds ?? 30),
      runtimeQuietSeconds: String(runtimePolicyData.runtimeQuietSeconds ?? 30),
      paperStartReadinessTimeoutSeconds: String(runtimePolicyData.paperStartReadinessTimeoutSeconds ?? 5),
    });
    setAccountSignalBindingMap(Object.fromEntries(accountBindingEntries));
    setStrategySignalBindingMap(Object.fromEntries(strategyBindingEntries));
    setSelectedSignalRuntimeId((current: any) => {
      if (current && normalizedSignalRuntimeSessions.some((item) => item.id === current)) {
        return current;
      }
      return normalizedSignalRuntimeSessions[0]?.id ?? null;
    });
    setCandles(normalizedCandles);
    setAnnotations(normalizedAnnotations);
    setBacktestForm((current: any) => ({
      strategyVersionId: current.strategyVersionId || normalizedStrategies[0]?.currentVersion?.id || "",
      signalTimeframe: current.signalTimeframe || normalizedBacktestOptions.defaultSignalTimeframe,
      executionDataSource: current.executionDataSource || normalizedBacktestOptions.defaultExecutionDataSource,
      symbol: current.symbol || "BTCUSDT",
      from: current.from || "",
      to: current.to || "",
    }));
    setPaperForm((current: any) => current);
    const strategyIDSet = new Set(normalizedStrategies.map((item) => item.id));
    const normalizeLoadedStrategyId = (candidate: string, fallback = "") =>
      candidate && strategyIDSet.has(candidate) ? candidate : fallback;
    setLiveBindingForm((current: any) => ({
      accountId: current.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      adapterKey: current.adapterKey || normalizedLiveAdapters[0]?.key || "binance-futures",
      positionMode: current.positionMode || "ONE_WAY",
      marginMode: current.marginMode || "CROSSED",
      sandbox: current.sandbox,
      apiKeyRef: current.apiKeyRef,
      apiSecretRef: current.apiSecretRef,
    }));
    setLiveOrderForm((current: any) => ({
      accountId: current.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      strategyVersionId: current.strategyVersionId || normalizedStrategies[0]?.currentVersion?.id || "",
      symbol: current.symbol || "BTCUSDT",
      side: current.side || "BUY",
      type: current.type || "LIMIT",
      quantity: current.quantity || "0.001",
      price: current.price || "",
    }));
    setLiveSessionForm((current: any) => ({
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
    setAccountSignalForm((current: any) => ({
      accountId: current.accountId || normalizedSummaries[0]?.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      sourceKey: current.sourceKey || availableSignalSources[0]?.key || "",
      role: current.role || "trigger",
      symbol: current.symbol || "BTCUSDT",
      timeframe: current.timeframe || "1d",
    }));
    setStrategySignalForm((current: any) => ({
      strategyId: normalizeLoadedStrategyId(current.strategyId, normalizedStrategies[0]?.id || ""),
      sourceKey: current.sourceKey || availableSignalSources[0]?.key || "",
      role: current.role || "trigger",
      symbol: current.symbol || "BTCUSDT",
      timeframe: current.timeframe || "1d",
    }));
    setStrategyCreateForm((current: any) => ({
      name: current.name || "",
      description: current.description || "",
    }));
    setSignalRuntimeForm((current: any) => ({
      accountId: current.accountId || normalizedSummaries[0]?.accountId || normalizedAccounts.find((item) => item.mode === "LIVE")?.id || "",
      strategyId: normalizeLoadedStrategyId(current.strategyId, normalizedStrategies[0]?.id || ""),
    }));
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

  useEffect(() => {
    setSelectedSample(null);
  }, [selectedBacktest?.id]);

  useEffect(() => {
    if (!selectedStrategy) {
      return;
    }
    const currentParameters = getRecord(selectedStrategy.currentVersion?.parameters);
    setStrategyEditorForm({
      strategyId: selectedStrategy.id,
      strategyEngine: String(currentParameters.strategyEngine ?? "bk-default"),
      signalTimeframe: String(
        currentParameters.signalTimeframe ??
          selectedStrategy.currentVersion?.signalTimeframe ??
          "1d"
      ),
      executionDataSource: String(
        currentParameters.executionDataSource ??
          currentParameters.executionTimeframe ??
          selectedStrategy.currentVersion?.executionTimeframe ??
          "tick"
      ),
      parametersJson: JSON.stringify(currentParameters, null, 2),
    });
  }, [selectedStrategy]);

  useEffect(() => {
    if (strategySignalForm.strategyId && !strategyIds.has(strategySignalForm.strategyId)) {
      setStrategySignalForm((current: any) => ({ ...current, strategyId: strategies[0]?.id || "" }));
    }
    if (signalRuntimeForm.strategyId && !strategyIds.has(signalRuntimeForm.strategyId)) {
      setSignalRuntimeForm((current: any) => ({ ...current, strategyId: strategies[0]?.id || "" }));
    }
    if (liveSessionForm.strategyId && !strategyIds.has(liveSessionForm.strategyId)) {
      setLiveSessionForm((current: any) => ({ ...current, strategyId: strategies[0]?.id || "" }));
    }
  }, [liveSessionForm.strategyId, signalRuntimeForm.strategyId, strategies, strategyIds, strategySignalForm.strategyId]);

  useEffect(() => {
    async function loadSignalDetails() {
      try {
        const tasks: Promise<unknown>[] = [];
        if (selectedSignalAccount) {
          tasks.push(
            fetchJSON<SignalBinding[]>(`/api/v1/accounts/${selectedSignalAccount}/signal-bindings`).then(setAccountSignalBindings)
          );
        } else {
          setAccountSignalBindings([]);
        }
        if (selectedSignalStrategy && strategyIds.has(selectedSignalStrategy)) {
          tasks.push(
            fetchJSON<SignalBinding[]>(`/api/v1/strategies/${selectedSignalStrategy}/signal-bindings`).then(setStrategySignalBindings)
          );
        } else {
          setStrategySignalBindings([]);
        }
        if (selectedRuntimeAccount && selectedRuntimeStrategy && strategyIds.has(selectedRuntimeStrategy)) {
          tasks.push(
            fetchJSON<Record<string, unknown>>(
              `/api/v1/signal-runtime/plan?accountId=${encodeURIComponent(selectedRuntimeAccount)}&strategyId=${encodeURIComponent(selectedRuntimeStrategy)}`
            ).then(setSignalRuntimePlan)
          );
        } else {
          setSignalRuntimePlan(null);
        }
        await Promise.all(tasks);
        setError(null);
      } catch (err) {
        setError(err instanceof Error ? err.message : "Failed to load signal runtime details");
      }
    }

    void loadSignalDetails();
  }, [selectedSignalAccount, selectedSignalStrategy, selectedRuntimeAccount, selectedRuntimeStrategy, strategyIds]);

  const chartPath = useMemo(() => buildLinePath(snapshots.map((item) => item.netEquity), 560, 180), [snapshots]);
  const chartRange = useMemo(() => summarizeRange(snapshots.map((item) => item.netEquity)), [snapshots]);
  const candleRange = useMemo(() => summarizeTimeRange(candles.map((item) => item.time)), [candles]);
  const chartAnnotations = useMemo(
    () => filterChartAnnotations(annotations, candles, monitorSessionID, monitorSessionSource, sourceFilter, eventFilter),
    [annotations, candles, monitorSessionID, monitorSessionSource, sourceFilter, eventFilter]
  );
  const selectedAnnotationIds = useMemo(() => {
    if (!selectedSample) {
      return [];
    }
    return chartAnnotations.filter((item) => annotationMatchesSample(item, selectedSample.sample)).map((item) => item.id);
  }, [chartAnnotations, selectedSample]);
  const selectedAnnotationFocusTime = useMemo(() => {
    if (selectedAnnotationIds.length === 0) {
      return undefined;
    }
    return chartAnnotations.find((item) => item.id === selectedAnnotationIds[0])?.time;
  }, [chartAnnotations, selectedAnnotationIds]);
  const selectedMarkerDetail = useMemo<MarkerDetail | null>(() => {
    if (selectedAnnotationIds.length === 0) {
      return null;
    }
    const item = chartAnnotations.find((annotation) => annotation.id === selectedAnnotationIds[0]);
    return item ? toMarkerDetail(item) : null;
  }, [chartAnnotations, selectedAnnotationIds]);
  const latestVisibleAnnotationTime = useMemo(
    () => (chartAnnotations.length > 0 ? chartAnnotations[chartAnnotations.length - 1].time : undefined),
    [chartAnnotations]
  );
  const markerDetail = useMemo<MarkerDetail | null>(() => {
    if (hoveredMarker) {
      return hoveredMarker;
    }
    if (selectedMarkerDetail) {
      return selectedMarkerDetail;
    }
    const latest = chartAnnotations[chartAnnotations.length - 1];
    return latest ? toMarkerDetail(latest) : null;
  }, [chartAnnotations, hoveredMarker, selectedMarkerDetail]);
  const markerLegend = useMemo<MarkerLegendItem[]>(
    () => [
      { label: "Initial", color: "#7a8791" },
      { label: "PT-Reentry", color: "#0e6d60" },
      { label: "SL-Reentry", color: "#1f8f7d" },
      { label: "PT Exit", color: "#c58b2d" },
      { label: "SL Exit", color: "#b04a37" },
      { label: "Paper Fill", color: "#284d86" },
    ],
    []
  );

  async function runSessionAction(sessionId: string, action: "start" | "stop" | "tick") {
    try {
      setSessionAction(`${sessionId}:${action}`);
      setError(null);
      await fetchJSON(`/api/v1/paper/sessions/${sessionId}/${action}`, { method: "POST" });
      await loadDashboard();
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to execute paper session action");
    } finally {
      setSessionAction(null);
    }
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

  async function createPaperSession() {
    if (!paperForm.accountId || !paperForm.strategyId) {
      setError("Paper session needs an account and strategy");
      return null;
    }
    setPaperCreateAction(true);
    try {
      const created = await fetchJSON<PaperSession>("/api/v1/paper/sessions", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          accountId: paperForm.accountId,
          strategyId: paperForm.strategyId,
          startEquity: Number(paperForm.startEquity) || 100000,
          signalTimeframe: paperForm.signalTimeframe,
          executionDataSource: paperForm.executionDataSource,
          symbol: paperForm.symbol,
          tradingFeeBps: Number(paperForm.tradingFeeBps) || 0,
          fundingRateBps: Number(paperForm.fundingRateBps) || 0,
          fundingIntervalHours: Number(paperForm.fundingIntervalHours) || 8,
        }),
      });
      await loadDashboard();
      setError(null);
      return created;
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create paper session");
      return null;
    } finally {
      setPaperCreateAction(false);
    }
  }

  async function createAndStartPaperSession() {
    setPaperLaunchAction(true);
    try {
      const created = await createPaperSession();
      if (!created?.id) {
        return;
      }
      setSessionAction(`${created.id}:start`);
      await fetchJSON(`/api/v1/paper/sessions/${created.id}/start`, { method: "POST" });
      await loadDashboard();
      setError(null);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to create and start paper session");
    } finally {
      setSessionAction(null);
      setPaperLaunchAction(false);
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
    const normalizedStrategyId = strategyIds.has(liveSessionForm.strategyId) ? liveSessionForm.strategyId : strategies[0]?.id || "";
    if (!liveSessionForm.accountId || !normalizedStrategyId) {
      setLiveSessionError("Live session needs an account and strategy");
      return null;
    }
    setLiveSessionError(null);
    setLiveSessionForm((current: any) => ({ ...current, strategyId: normalizedStrategyId }));
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
      setLiveSessionForm((current: any) => ({ ...current, strategyId: normalizedStrategyId }));
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
    const normalizedStrategyId = strategyIds.has(liveSessionForm.strategyId) ? liveSessionForm.strategyId : strategies[0]?.id || "";
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
    setLiveBindingForm((current: any) => ({ ...current, accountId: account.id }));
    setLiveSessionForm((current: any) => ({ ...current, accountId: account.id, strategyId }));
    setSignalRuntimeForm((current: any) => ({ ...current, accountId: account.id, strategyId }));
    setAccountSignalForm((current: any) => ({ ...current, accountId: account.id }));
    setStrategySignalForm((current: any) => ({ ...current, strategyId }));

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
        setLiveBindingForm((current: any) => ({ ...current, accountId: account.id }));
        window.location.hash = "live";
        break;
      case "bind-signals":
        setAccountSignalForm((current: any) => ({ ...current, accountId: account.id }));
        setSignalRuntimeForm((current: any) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "create-runtime":
        setSignalRuntimeForm((current: any) => ({ ...current, accountId: account.id }));
        window.location.hash = "signals";
        break;
      case "start-runtime":
      case "inspect-runtime":
        if (activeRuntime) {
          jumpToSignalRuntimeSession(activeRuntime.id);
        } else {
          setSignalRuntimeForm((current: any) => ({ ...current, accountId: account.id }));
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
      setTelegramForm((current: any) => ({ ...current, botToken: "" }));
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
      setLoginForm((current: any) => ({ ...current, password: "" }));
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

  return (
    <>
    <WorkbenchLayout
      sidebarTab={sidebarTab}
      onSidebarTabChange={setSidebarTab}
      dockTab={dockTab}
      onDockTabChange={setDockTab}
      headerMetrics={
        <div className="flex space-x-6">
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
          <div className="p-4 space-y-6">
            <section id="backtests" className="panel panel-backtests">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Backtests</p>
              <h3>回测配置与运行记录</h3>
            </div>
            <div className="range-box">
              <span>{backtests.length} runs</span>
              <span>{strategies.length} strategies</span>
            </div>
          </div>
          <div className="backtest-layout">
            <div className="backtest-form">
              <div className="form-grid">
                <label className="form-field">
                  <span>Strategy Version</span>
                  <select
                    value={backtestForm.strategyVersionId}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, strategyVersionId: event.target.value }))}
                  >
                    {strategies.map((strategy) => (
                      <option key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                        {strategyLabel(strategy)}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Signal Timeframe</span>
                  <select
                    value={backtestForm.signalTimeframe}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, signalTimeframe: event.target.value }))}
                  >
                    {(backtestOptions?.signalTimeframes ?? ["4h", "1d"]).map((item) => (
                      <option key={item} value={item}>
                        {item}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Execution Source</span>
                  <select
                    value={backtestForm.executionDataSource}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, executionDataSource: event.target.value }))}
                  >
                    {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                      <option key={item} value={item}>
                        {item}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Symbol</span>
                  <input
                    value={backtestForm.symbol}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
                    placeholder="BTCUSDT"
                  />
                </label>
                <label className="form-field">
                  <span>From (RFC3339)</span>
                  <input
                    value={backtestForm.from}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, from: event.target.value }))}
                    placeholder="2020-01-01T00:00:00Z"
                  />
                </label>
                <label className="form-field">
                  <span>To (RFC3339)</span>
                  <input
                    value={backtestForm.to}
                    onChange={(event) => setBacktestForm((current: any) => ({ ...current, to: event.target.value }))}
                    placeholder="2020-01-31T23:59:59Z"
                  />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton
                  label={backtestAction ? "Submitting..." : "Create Backtest"}
                  disabled={
                    backtestAction ||
                    backtestForm.strategyVersionId.trim() === "" ||
                    backtestForm.symbol.trim() === "" ||
                    selectedExecutionAvailability === "missing" ||
                    !selectedSymbolAvailable
                  }
                  onClick={createBacktestRun}
                />
              </div>
              {backtestOptions ? (
                <div className="backtest-notes">
                  <div className="note-item">
                    tick: {String(backtestOptions.availability?.tick ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.tick ?? "--")}
                  </div>
                  <div className="note-item">
                    1min: {String(backtestOptions.availability?.["1min"] ?? "unknown")} · dir: {String(backtestOptions.dataDirectories?.["1min"] ?? "--")}
                  </div>
                  <div className="note-item">
                    selected source: {backtestForm.executionDataSource} · {selectedExecutionDatasets.length} dataset file(s)
                  </div>
                  <div className="note-item">
                    symbols: {selectedExecutionSymbols.length > 0 ? selectedExecutionSymbols.join(", ") : "--"}
                  </div>
                  <div className="note-item">
                    required columns: {selectedExecutionSchema?.requiredColumns?.join(", ") ?? "--"}
                  </div>
                  <div className="note-item">
                    file examples: {selectedExecutionSchema?.filenameExamples?.join(", ") ?? "--"}
                  </div>
                  {!selectedSymbolAvailable ? (
                    <div className="note-item">
                      symbol {backtestForm.symbol.trim().toUpperCase()} is not available for {backtestForm.executionDataSource}
                    </div>
                  ) : null}
                  {selectedExecutionDatasets.slice(0, 3).map((dataset) => (
                    <div key={dataset.path} className="note-item">
                      {dataset.name} · {dataset.symbol}
                      {dataset.format ? ` · ${dataset.format}` : ""}
                      {dataset.fileCount ? ` · files ${dataset.fileCount}` : ""}
                    </div>
                  ))}
                  {(backtestOptions.notes ?? []).map((note) => (
                    <div key={note} className="note-item">
                      {note}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
            <div className="backtest-list">
              {backtestItems.length > 0 ? (
                <div className="table-wrap">
                  <table>
                    <thead>
                      <tr>
                        <th>Time</th>
                        <th>Mode</th>
                        <th>Symbol</th>
                        <th>Status</th>
                        <th>Return</th>
                        <th>DD</th>
                      </tr>
                    </thead>
                    <tbody>
                      {backtestItems.map((item) => (
                        <tr
                          key={item.id}
                          className={item.id === selectedBacktest?.id ? "table-row-active" : ""}
                          onClick={() => setSelectedBacktestId(item.id)}
                        >
                          <td>{formatTime(item.createdAt)}</td>
                          <td>{String(item.parameters?.backtestMode ?? "--")}</td>
                          <td>{String(item.parameters?.symbol ?? "--")}</td>
                          <td>{item.status}</td>
                          <td>{formatPercent(item.resultSummary?.return)}</td>
                          <td>{formatPercent(item.resultSummary?.maxDrawdown)}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table>
                </div>
              ) : (
                <div className="empty-state">No backtests yet</div>
              )}
              <div className="backtest-detail-card">
                <div className="panel-header">
                  <div>
                    <p className="panel-kicker">Strategy Replay</p>
                    <h3>选中回测详情</h3>
                  </div>
                  <div className="range-box range-box-wrap">
                    <span>{selectedBacktest?.status ?? "NO RUN"}</span>
                    <span>{String(selectedBacktest?.parameters?.backtestMode ?? "--")}</span>
                    <button
                      type="button"
                      className="filter-chip"
                      disabled={!selectedBacktest?.parameters?.from || !selectedBacktest?.parameters?.to}
                      onClick={() => {
                        const from = Date.parse(String(selectedBacktest?.parameters?.from ?? ""));
                        const to = Date.parse(String(selectedBacktest?.parameters?.to ?? ""));
                        if (!Number.isFinite(from) || !Number.isFinite(to)) {
                          return;
                        }
                        setChartOverrideRange({
                          from: Math.floor(from / 1000),
                          to: Math.floor(to / 1000),
                          label: "Backtest Window",
                        });
                        setFocusNonce((value) => value + 1);
                      }}
                    >
                      Use Backtest Window
                    </button>
                    <button
                      type="button"
                      className="filter-chip"
                      disabled={!chartOverrideRange}
                      onClick={() => setChartOverrideRange(null)}
                    >
                      Back To Live Window
                    </button>
                    <a
                      className={`filter-chip ${selectedBacktest ? "" : "filter-chip-disabled"}`}
                      href={selectedBacktest ? `${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv` : undefined}
                    >
                      Export Trades CSV
                    </a>
                  </div>
                </div>
                {selectedBacktest ? (
                  <>
                    <div className="detail-grid">
                      <div className="detail-item">
                        <span>Execution Source</span>
                        <strong>{latestExecutionSource || "--"}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Matched Files</span>
                        <strong>{String(latestBacktestSummary.matchedArchiveFiles ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>{previewCountLabel}</span>
                        <strong>{String(latestBacktestSummary.streamPreviewTicks ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>{processedCountLabel}</span>
                        <strong>{processedCountValue}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Trade Count</span>
                        <strong>{String(latestBacktestSummary.executionTradeCount ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Closed Trades</span>
                        <strong>{String(latestBacktestSummary.executionClosedCount ?? "--")}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Win Rate</span>
                        <strong>{formatPercent(latestBacktestSummary.executionWinRate)}</strong>
                      </div>
                      <div className="detail-item">
                        <span>Total PnL</span>
                        <strong>{formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL))}</strong>
                      </div>
                    </div>

                    <div className="backtest-breakdown">
                      <h4>Execution Trades</h4>
                      {latestExecutionTrades.length > 0 ? (
                        <SimpleTable
                          columns={["Status", "Source", "Side", "Qty", "Entry", "Exit", "Exit Type", "PnL"]}
                          rows={latestExecutionTrades.map((trade) => [
                            String(trade.status ?? "--"),
                            String(trade.source ?? "--"),
                            String(trade.side ?? "--"),
                            formatMaybeNumber(trade.quantity),
                            `${formatMaybeNumber(trade.entryPrice)} @ ${formatTime(String(trade.entryTime ?? ""))}`,
                            `${formatMaybeNumber(trade.exitPrice)} @ ${formatTime(String(trade.exitTime ?? ""))}`,
                            String(trade.exitType ?? "--"),
                            formatSigned(getNumber(trade.realizedPnL)),
                          ])}
                          emptyMessage="No execution trades"
                        />
                      ) : (
                        <div className="empty-state empty-state-compact">No execution trades yet</div>
                      )}
                    </div>

                    {Boolean(latestBacktestSummary.replayLedgerTrades) ? (
                      <>
                        <div className="backtest-breakdown">
                          <h4>Optional Ledger Audit</h4>
                          {Object.keys(latestReplayByReason).length > 0 ? (
                            <SimpleTable
                              columns={["Reason", "Trades", "Completed", "Skipped", "Entry", "Exit"]}
                              rows={Object.entries(latestReplayByReason).map(([reason, stats]) => [
                                reason,
                                String(stats.trades ?? 0),
                                String(stats.completed ?? 0),
                                String(stats.skipped ?? 0),
                                String(stats.skippedEntry ?? 0),
                                String(stats.skippedExit ?? 0),
                              ])}
                              emptyMessage="No grouped replay stats"
                            />
                          ) : (
                            <div className="empty-state empty-state-compact">No optional ledger audit data</div>
                          )}
                        </div>

                        <div className="backtest-samples-grid">
                          <div className="backtest-sample-panel">
                            <h4>Completed Samples</h4>
                            {latestReplayCompletedSamples.length > 0 ? (
                              latestReplayCompletedSamples.map((sample, index) => (
                                <SampleCard
                                  key={`completed-${index}`}
                                  sample={sample}
                                  selected={selectedSample?.key === buildSampleKey("completed", index, sample)}
                                  onSelect={() => {
                                    const range = buildSampleRange(sample);
                                    if (!range) {
                                      return;
                                    }
                                    setSelectedSample({ key: buildSampleKey("completed", index, sample), sample });
                                    setChartOverrideRange(range);
                                    setSourceFilter("backtest");
                                    setEventFilter("all");
                                    setFocusNonce((value) => value + 1);
                                  }}
                                />
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No completed samples</div>
                            )}
                          </div>
                          <div className="backtest-sample-panel">
                            <h4>Skipped Samples</h4>
                            {latestReplaySkippedSamples.length > 0 ? (
                              latestReplaySkippedSamples.map((sample, index) => (
                                <SampleCard
                                  key={`skipped-${index}`}
                                  sample={sample}
                                  selected={selectedSample?.key === buildSampleKey("skipped", index, sample)}
                                  onSelect={() => {
                                    const range = buildSampleRange(sample);
                                    if (!range) {
                                      return;
                                    }
                                    setSelectedSample({ key: buildSampleKey("skipped", index, sample), sample });
                                    setChartOverrideRange(range);
                                    setSourceFilter("backtest");
                                    setEventFilter("all");
                                    setFocusNonce((value) => value + 1);
                                  }}
                                />
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No skipped samples</div>
                            )}
                          </div>
                        </div>
                      </>
                    ) : null}
                  </>
                ) : (
                  <div className="empty-state empty-state-compact">No backtest detail yet</div>
                )}
              </div>
            </div>
          </div>
        </section>
          </div>
        ) : sidebarTab === 'account' ? (
          <div className="p-4 space-y-6">
            <section id="signals" className="panel panel-session">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Signal Runtime</p>
              <h3>信号源绑定与市场数据运行时</h3>
            </div>
            <div className="range-box">
              <span>{signalCatalog?.sources?.length ?? 0} sources</span>
              <span>{signalRuntimeSessions.length} sessions</span>
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-form session-form">
              <h4>Bind Account Signal Source</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Account</span>
                  <select value={accountSignalForm.accountId} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, accountId: event.target.value }))}>
                    {liveAccounts.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name} ({item.mode})
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Source</span>
                  <select value={accountSignalForm.sourceKey} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, sourceKey: event.target.value }))}>
                    {(signalCatalog?.sources ?? []).map((source) => (
                      <option key={source.key} value={source.key}>
                        {source.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Role</span>
                  <select value={accountSignalForm.role} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, role: event.target.value }))}>
                    <option value="signal">signal</option>
                    <option value="trigger">trigger</option>
                    <option value="feature">feature</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>Timeframe</span>
                  <select value={accountSignalForm.timeframe} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, timeframe: event.target.value }))}>
                    <option value="4h">4h</option>
                    <option value="1d">1d</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>Symbol</span>
                  <input value={accountSignalForm.symbol} onChange={(event) => setAccountSignalForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton label={signalBindingAction === "account" ? "Binding..." : "Bind Account Source"} disabled={signalBindingAction !== null || !accountSignalForm.accountId} onClick={bindAccountSignalSource} />
              </div>
            </div>

            <div className="backtest-form session-form">
              <h4>Bind Strategy Signal Source</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Strategy</span>
                  <select value={strategySignalForm.strategyId} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, strategyId: event.target.value }))}>
                    {strategyOptions.map((strategy) => (
                      <option key={strategy.value} value={strategy.value}>
                        {strategy.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Source</span>
                  <select value={strategySignalForm.sourceKey} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, sourceKey: event.target.value }))}>
                    {(signalCatalog?.sources ?? []).map((source) => (
                      <option key={source.key} value={source.key}>
                        {source.name}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Role</span>
                  <select value={strategySignalForm.role} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, role: event.target.value }))}>
                    <option value="signal">signal</option>
                    <option value="trigger">trigger</option>
                    <option value="feature">feature</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>Timeframe</span>
                  <select value={strategySignalForm.timeframe} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, timeframe: event.target.value }))}>
                    <option value="4h">4h</option>
                    <option value="1d">1d</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>Symbol</span>
                  <input value={strategySignalForm.symbol} onChange={(event) => setStrategySignalForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))} />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton label={signalBindingAction === "strategy" ? "Binding..." : "Bind Strategy Source"} disabled={signalBindingAction !== null || !strategySignalForm.strategyId} onClick={bindStrategySignalSource} />
              </div>
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-list">
              <h4>Signal Source Catalog</h4>
              {signalCatalog?.sources?.length ? (
                <SimpleTable
                  columns={["Source", "Exchange", "Type", "Roles", "Env", "Transport"]}
                  rows={signalCatalog.sources.map((source) => [
                    source.name,
                    source.exchange,
                    source.streamType,
                    source.roles.join(", "),
                    source.environments.join(", "),
                    source.transport,
                  ])}
                  emptyMessage="No signal sources"
                />
              ) : (
                <div className="empty-state empty-state-compact">No signal source catalog</div>
              )}
              <div className="backtest-notes">
                {(signalCatalog?.notes ?? []).map((note) => (
                  <div key={note} className="note-item">
                    {note}
                  </div>
                ))}
                {(signalSourceTypes ?? []).map((item) => (
                  <div key={item.streamType} className="note-item">
                    {item.streamType}: {item.description}
                  </div>
                ))}
              </div>
            </div>

            <div className="backtest-list">
              <h4>Current Bindings</h4>
              <div className="backtest-breakdown">
                <h5>Account</h5>
                <SimpleTable
                  columns={["Source", "Role", "Symbol", "Exchange", "Status"]}
                  rows={accountSignalBindings.map((item) => [item.sourceName, item.role, item.symbol || "--", item.exchange, item.status])}
                  emptyMessage="No account bindings"
                />
              </div>
              <div className="backtest-breakdown">
                <h5>Strategy</h5>
                <SimpleTable
                  columns={["Source", "Role", "Symbol", "Exchange", "Status"]}
                  rows={strategySignalBindings.map((item) => [item.sourceName, item.role, item.symbol || "--", item.exchange, item.status])}
                  emptyMessage="No strategy bindings"
                />
              </div>
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-form session-form">
              <h4>Runtime Policy</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Trade Tick Freshness (s)</span>
                  <input
                    value={runtimePolicyForm.tradeTickFreshnessSeconds}
                    onChange={(event) =>
                      setRuntimePolicyForm((current: any) => ({ ...current, tradeTickFreshnessSeconds: event.target.value }))
                    }
                  />
                </label>
                <label className="form-field">
                  <span>Order Book Freshness (s)</span>
                  <input
                    value={runtimePolicyForm.orderBookFreshnessSeconds}
                    onChange={(event) =>
                      setRuntimePolicyForm((current: any) => ({ ...current, orderBookFreshnessSeconds: event.target.value }))
                    }
                  />
                </label>
                <label className="form-field">
                  <span>Signal Bar Freshness (s)</span>
                  <input
                    value={runtimePolicyForm.signalBarFreshnessSeconds}
                    onChange={(event) =>
                      setRuntimePolicyForm((current: any) => ({ ...current, signalBarFreshnessSeconds: event.target.value }))
                    }
                  />
                </label>
                <label className="form-field">
                  <span>Runtime Quiet (s)</span>
                  <input
                    value={runtimePolicyForm.runtimeQuietSeconds}
                    onChange={(event) =>
                      setRuntimePolicyForm((current: any) => ({ ...current, runtimeQuietSeconds: event.target.value }))
                    }
                  />
                </label>
                <label className="form-field">
                  <span>Paper Start Timeout (s)</span>
                  <input
                    value={runtimePolicyForm.paperStartReadinessTimeoutSeconds}
                    onChange={(event) =>
                      setRuntimePolicyForm((current: any) => ({
                        ...current,
                        paperStartReadinessTimeoutSeconds: event.target.value,
                      }))
                    }
                  />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton
                  label={runtimePolicyAction ? "Saving..." : "Save Runtime Policy"}
                  disabled={runtimePolicyAction}
                  onClick={updateRuntimePolicy}
                />
              </div>
              <div className="backtest-notes">
                <div className="note-item">
                  active policy: tick {runtimePolicy?.tradeTickFreshnessSeconds ?? "--"}s · book {runtimePolicy?.orderBookFreshnessSeconds ?? "--"}s ·
                  bar {runtimePolicy?.signalBarFreshnessSeconds ?? "--"}s
                </div>
                <div className="note-item">
                  quiet {runtimePolicy?.runtimeQuietSeconds ?? "--"}s · paper preflight {runtimePolicy?.paperStartReadinessTimeoutSeconds ?? "--"}s
                </div>
              </div>
            </div>

            <div className="backtest-form session-form">
              <h4>Create Runtime Session</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>Account</span>
                  <select value={signalRuntimeForm.accountId} onChange={(event) => setSignalRuntimeForm((current: any) => ({ ...current, accountId: event.target.value }))}>
                    {liveAccounts.map((item) => (
                      <option key={item.id} value={item.id}>
                        {item.name} ({item.mode})
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>Strategy</span>
                  <select value={signalRuntimeForm.strategyId} onChange={(event) => setSignalRuntimeForm((current: any) => ({ ...current, strategyId: event.target.value }))}>
                    {strategyOptions.map((strategy) => (
                      <option key={strategy.value} value={strategy.value}>
                        {strategy.label}
                      </option>
                    ))}
                  </select>
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton label={signalRuntimeAction === "create" ? "Creating..." : "Create Runtime Session"} disabled={signalRuntimeAction !== null || !signalRuntimeForm.accountId || !signalRuntimeForm.strategyId} onClick={createSignalRuntimeSession} />
              </div>
              <div className="detail-grid">
                <div className="detail-item">
                  <span>Plan Ready</span>
                  <strong>{boolLabel(signalRuntimePlan?.ready)}</strong>
                </div>
                <div className="detail-item">
                  <span>Required</span>
                  <strong>{String((signalRuntimePlan?.requiredBindings as unknown[] | undefined)?.length ?? 0)}</strong>
                </div>
                <div className="detail-item">
                  <span>Matched</span>
                  <strong>{String((signalRuntimePlan?.matchedBindings as unknown[] | undefined)?.length ?? 0)}</strong>
                </div>
                <div className="detail-item">
                  <span>Missing</span>
                  <strong>{String((signalRuntimePlan?.missingBindings as unknown[] | undefined)?.length ?? 0)}</strong>
                </div>
              </div>
              <div className="backtest-notes">
                <div className="note-item">runtime adapters: {signalRuntimeAdapters.map((item) => item.key).join(", ") || "--"}</div>
                {((signalRuntimePlan?.missingBindings as unknown[] | undefined) ?? []).slice(0, 4).map((item, index) => (
                  <div key={index} className="note-item">
                    missing: {JSON.stringify(item)}
                  </div>
                ))}
              </div>
            </div>

            <div className="backtest-list">
              <h4>Runtime Sessions</h4>
              {signalRuntimeSessions.length > 0 ? (
                <>
                  <div className="table-wrap">
                    <table>
                      <thead>
                        <tr>
                          <th>Session</th>
                          <th>Status</th>
                          <th>Adapter</th>
                          <th>Subs</th>
                          <th>Heartbeat</th>
                          <th>Action</th>
                        </tr>
                      </thead>
                      <tbody>
                        {signalRuntimeSessions.map((session) => (
                          <tr
                            key={session.id}
                            className={session.id === selectedSignalRuntime?.id ? "table-row-active" : ""}
                            onClick={() => setSelectedSignalRuntimeId(session.id)}
                          >
                            <td>{shrink(session.id)}</td>
                            <td>{session.status}</td>
                            <td>{session.runtimeAdapter || "--"}</td>
                            <td>{String(session.subscriptionCount)}</td>
                            <td>{formatTime(String(session.state?.lastHeartbeatAt ?? ""))}</td>
                            <td>
                              <div className="inline-actions">
                                <ActionButton
                                  label={signalRuntimeAction === `${session.id}:start` ? "Starting..." : "Start"}
                                  disabled={signalRuntimeAction !== null || session.status === "RUNNING"}
                                  onClick={() => runSignalRuntimeAction(session.id, "start")}
                                />
                                <ActionButton
                                  label={signalRuntimeAction === `${session.id}:stop` ? "Stopping..." : "Stop"}
                                  variant="ghost"
                                  disabled={signalRuntimeAction !== null || session.status === "STOPPED"}
                                  onClick={() => runSignalRuntimeAction(session.id, "stop")}
                                />
                              </div>
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>

                  <div className="backtest-detail-card">
                    <div className="panel-header">
                      <div>
                        <p className="panel-kicker">Signal Session</p>
                        <h3>选中 Runtime Session 详情</h3>
                      </div>
                      <div className="range-box">
                        <span>{selectedSignalRuntime?.status ?? "NO SESSION"}</span>
                        <span>{selectedSignalRuntime?.runtimeAdapter ?? "--"}</span>
                      </div>
                    </div>
                    {selectedSignalRuntime ? (
                      <>
                        <div className="detail-grid">
                          <div className="detail-item">
                            <span>Session ID</span>
                            <strong>{shrink(selectedSignalRuntime.id)}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Account</span>
                            <strong>{shrink(selectedSignalRuntime.accountId)}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Strategy</span>
                            <strong>{shrink(selectedSignalRuntime.strategyId)}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Transport</span>
                            <strong>{selectedSignalRuntime.transport || "--"}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Health</span>
                            <strong>{String(selectedSignalRuntimeState.health ?? "--")}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Signal Events</span>
                            <strong>{String(Math.trunc(getNumber(selectedSignalRuntimeState.signalEventCount) ?? 0))}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Heartbeat</span>
                            <strong>{formatTime(String(selectedSignalRuntimeState.lastHeartbeatAt ?? ""))}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Last Event</span>
                            <strong>{formatTime(String(selectedSignalRuntimeState.lastEventAt ?? ""))}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Source States</span>
                            <strong>{String(Object.keys(selectedSignalRuntimeSourceStates).length)}</strong>
                          </div>
                          <div className="detail-item">
                            <span>Plan Ready</span>
                            <strong>{boolLabel(selectedSignalRuntimePlan.ready)}</strong>
                          </div>
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Subscriptions</h4>
                          <SimpleTable
                            columns={["Source", "Role", "Symbol", "Channel", "Adapter"]}
                            rows={selectedSignalRuntimeSubscriptions.map((item) => [
                              String(item.sourceKey ?? "--"),
                              String(item.role ?? "--"),
                              String(item.symbol ?? "--"),
                              String(item.channel ?? "--"),
                              String(item.adapterKey ?? "--"),
                            ])}
                            emptyMessage="No subscriptions"
                          />
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Signal Bars</h4>
                          {selectedSignalRuntimeSignalBars.length > 0 ? (
                            <div className="chart-shell">
                              <SignalBarChart candles={selectedSignalRuntimeSignalBars} />
                            </div>
                          ) : (
                            <div className="empty-state empty-state-compact">No 4h/1d signal bars cached yet</div>
                          )}
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Signal States</h4>
                          <div className="backtest-notes">
                            {Object.entries(selectedSignalBarStates).length > 0 ? (
                              Object.entries(selectedSignalBarStates).map(([key, value]) => {
                                const state = getRecord(value);
                                const current = getRecord(state.current);
                                const prevBar1 = getRecord(state.prevBar1);
                                const prevBar2 = getRecord(state.prevBar2);
                                return (
                                  <div key={key} className="note-item">
                                    {[
                                      key,
                                      `tf=${String(state.timeframe ?? "--")}`,
                                      `bars=${String(state.barCount ?? "--")}`,
                                      `ma20=${formatMaybeNumber(state.ma20)}`,
                                      `atr14=${formatMaybeNumber(state.atr14)}`,
                                      `t-1=${formatMaybeNumber(prevBar1.open)}/${formatMaybeNumber(prevBar1.high)}/${formatMaybeNumber(prevBar1.low)}/${formatMaybeNumber(prevBar1.close)}`,
                                      `t-2=${formatMaybeNumber(prevBar2.open)}/${formatMaybeNumber(prevBar2.high)}/${formatMaybeNumber(prevBar2.low)}/${formatMaybeNumber(prevBar2.close)}`,
                                      `current=${formatMaybeNumber(current.open)}/${formatMaybeNumber(current.high)}/${formatMaybeNumber(current.low)}/${formatMaybeNumber(current.close)}`,
                                    ].join(" · ")}
                                  </div>
                                );
                              })
                            ) : (
                              <div className="empty-state empty-state-compact">No signal states yet</div>
                            )}
                          </div>
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Runtime Timeline</h4>
                          <div className="backtest-notes">
                            {buildTimelineNotes(selectedSignalRuntimeTimeline).map((line) => (
                              <div key={line} className="note-item">
                                {line}
                              </div>
                            ))}
                          </div>
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Last Event Summary</h4>
                          <div className="backtest-notes">
                            {Object.entries(selectedSignalRuntimeLastSummary).length > 0 ? (
                              Object.entries(selectedSignalRuntimeLastSummary).map(([key, value]) => (
                                <div key={key} className="note-item">
                                  {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                                </div>
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No event summary yet</div>
                            )}
                          </div>
                        </div>

                        <div className="backtest-breakdown">
                          <h4>Source States</h4>
                          <div className="backtest-notes">
                            {Object.entries(selectedSignalRuntimeSourceStates).length > 0 ? (
                              Object.entries(selectedSignalRuntimeSourceStates).slice(0, 8).map(([key, value]) => (
                                <div key={key} className="note-item">
                                  {key}: {typeof value === "object" ? JSON.stringify(value) : String(value)}
                                </div>
                              ))
                            ) : (
                              <div className="empty-state empty-state-compact">No source states yet</div>
                            )}
                          </div>
                        </div>
                      </>
                    ) : (
                      <div className="empty-state empty-state-compact">No runtime session selected</div>
                    )}
                  </div>
                </>
              ) : (
                <div className="empty-state empty-state-compact">No runtime sessions</div>
              )}
            </div>
          </div>
        </section>
          </div>
        ) : null
      }
      dockContent={
        <div className="h-full">
          <div style={{ display: dockTab === 'orders' ? 'block' : 'none' }} className="h-full p-4"><article id="orders" className="panel">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">Orders</p>
                <h3>最新订单</h3>
              </div>
            </div>
            <div className="grid grid-cols-2 gap-8 items-start">
              <div className="backtest-form session-form">
                <h4>Create Live Order</h4>
                <div className="form-grid">
                  <label className="form-field">
                    <span>Live Account</span>
                    <select
                      value={liveOrderForm.accountId}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, accountId: event.target.value }))}
                    >
                      {liveAccounts.map((account) => (
                        <option key={account.id} value={account.id}>
                          {account.name} ({account.status})
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="form-field">
                    <span>Strategy Version</span>
                    <select
                      value={liveOrderForm.strategyVersionId}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, strategyVersionId: event.target.value }))}
                    >
                      <option value="">Auto</option>
                      {strategies.map((strategy) => (
                        <option key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                          {strategy.name} · {strategy.currentVersion?.version ?? "no-version"}
                        </option>
                      ))}
                    </select>
                  </label>
                  <label className="form-field">
                    <span>Symbol</span>
                    <input
                      value={liveOrderForm.symbol}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, symbol: event.target.value.toUpperCase() }))}
                    />
                  </label>
                  <label className="form-field">
                    <span>Side</span>
                    <select
                      value={liveOrderForm.side}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, side: event.target.value }))}
                    >
                      <option value="BUY">BUY</option>
                      <option value="SELL">SELL</option>
                    </select>
                  </label>
                  <label className="form-field">
                    <span>Type</span>
                    <select
                      value={liveOrderForm.type}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, type: event.target.value }))}
                    >
                      <option value="LIMIT">LIMIT</option>
                      <option value="MARKET">MARKET</option>
                    </select>
                  </label>
                  <label className="form-field">
                    <span>Quantity</span>
                    <input
                      value={liveOrderForm.quantity}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, quantity: event.target.value }))}
                    />
                  </label>
                  <label className="form-field">
                    <span>Price</span>
                    <input
                      value={liveOrderForm.price}
                      onChange={(event) => setLiveOrderForm((current: any) => ({ ...current, price: event.target.value }))}
                      placeholder={liveOrderForm.type === "MARKET" ? "optional" : "required for limit"}
                    />
                  </label>
                </div>
                <div className="live-account-meta">
                  <span>
                    <StatusPill tone={runtimeReadinessTone(selectedLiveOrderPreflight.status)}>
                      {selectedLiveOrderPreflight.status}
                    </StatusPill>
                  </span>
                  <span>{selectedLiveOrderPreflight.reason}</span>
                  <span>{selectedLiveOrderPreflight.detail}</span>
                </div>
                <div className="backtest-actions">
                  <ActionButton
                    label={liveOrderAction ? "Submitting..." : "Submit Live Order"}
                    disabled={
                      liveOrderAction ||
                      selectedLiveOrderPreflight.status === "blocked" ||
                      !liveOrderForm.accountId ||
                      !liveOrderForm.symbol.trim() ||
                      !(Number(liveOrderForm.quantity) > 0) ||
                      (liveOrderForm.type === "LIMIT" && !(Number(liveOrderForm.price) > 0))
                    }
                    onClick={createLiveOrder}
                  />
                </div>
              </div>

              <div className="backtest-list session-form">
                <h4>Live Execution Context</h4>
                <div className="detail-grid">
                  <div className="detail-item">
                    <span>Runtime</span>
                    <strong>{selectedLiveOrderActiveRuntime ? `${selectedLiveOrderActiveRuntime.status} · ${selectedLiveOrderActiveRuntime.runtimeAdapter}` : "--"}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Health</span>
                    <strong>{String(selectedLiveOrderRuntimeState.health ?? "--")}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Signal Bias</span>
                    <strong>{selectedLiveOrderSignalAction.bias}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Signal State</span>
                    <strong>{selectedLiveOrderSignalAction.state}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Trade</span>
                    <strong>{formatMaybeNumber(selectedLiveOrderMarket.tradePrice)}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Bid / Ask</span>
                    <strong>{formatMaybeNumber(selectedLiveOrderMarket.bestBid)} / {formatMaybeNumber(selectedLiveOrderMarket.bestAsk)}</strong>
                  </div>
                  <div className="detail-item">
                    <span>Spread</span>
                    <strong>{formatMaybeNumber(selectedLiveOrderMarket.spreadBps)} bps</strong>
                  </div>
                  <div className="detail-item">
                    <span>Signal TF</span>
                    <strong>{String(selectedLiveOrderSignalBarState.timeframe ?? "--")}</strong>
                  </div>
                  <div className="detail-item">
                    <span>MA20 / ATR14</span>
                    <strong>{formatMaybeNumber(selectedLiveOrderSignalBarState.ma20)} / {formatMaybeNumber(selectedLiveOrderSignalBarState.atr14)}</strong>
                  </div>
                </div>
                <div className="backtest-notes">
                  {buildSignalActionNotes(selectedLiveOrderSignalAction).map((line) => (
                    <div key={line} className="note-item">
                      {line}
                    </div>
                  ))}
                  {buildSignalBarStateNotes(selectedLiveOrderSignalBarState).slice(0, 2).map((line) => (
                    <div key={line} className="note-item">
                      {line}
                    </div>
                  ))}
                  {buildRuntimeEventNotes(selectedLiveOrderRuntimeSummary).map((line) => (
                    <div key={line} className="note-item">
                      {line}
                    </div>
                  ))}
                </div>
              </div>
            </div>
            <SimpleTable
              columns={["Time", "Symbol", "Side", "Qty", "Price", "Status", "Mode", "Runtime", "Preflight"]}
              rows={orders
                .slice()
                .reverse()
                .slice(0, 8)
                .map((order) => [
                  formatTime(String(order.metadata?.eventTime ?? order.createdAt)),
                  order.symbol,
                  order.side,
                  formatNumber(order.quantity, 4),
                  formatMoney(order.price),
                  order.status,
                  String(order.metadata?.executionMode ?? "--"),
                  String(order.metadata?.runtimeSessionId ?? "--"),
                  summarizeOrderPreflight(order.metadata?.runtimePreflight),
                ])}
              emptyMessage="No orders"
            />
          </article></div>
          <div style={{ display: dockTab === 'positions' ? 'block' : 'none' }} className="h-full p-4 space-y-6">{/* missing equity */}{/* missing positions */}</div>
          <div style={{ display: dockTab === 'fills' ? 'block' : 'none' }} className="h-full p-4">{/* missing fills */}</div>
          <div style={{ display: dockTab === 'alerts' ? 'block' : 'none' }} className="h-full p-4 space-y-6"><section id="alerts" className="panel panel-alerts">
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
        </section><section id="notifications" className="panel panel-alerts">
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
            <div className="empty-state empty-state-compact">No notifications yet</div>
          )}
        </section></div>
        </div>
      }
      mainStageContent={
        sidebarTab === 'monitor' ? (
          <div className="flex flex-col p-4 bg-zinc-950/20">
            <section id="monitor" className="panel panel-market panel-compact monitor-panel-main w-full">
            <div className="panel-header">
              <div>
                <p className="panel-kicker">主监控</p>
                <h3>运行中会话的大周期 K 线与执行状态</h3>
              </div>
              <div className="range-box">
                <span>{monitorMode}</span>
                <span>{monitorBars.length} 根 K 线</span>
                <span>{monitorMarkers.length} 个标记</span>
                <span>{String(monitorSignalState.timeframe ?? "--")}</span>
              </div>
            </div>
            <div className="chart-shell chart-shell-market h-[320px] min-h-[260px]">
              {monitorBars.length > 0 ? (
                <SignalMonitorChart candles={monitorBars} markers={monitorMarkers} />
              ) : (
                <div className="empty-state">当前运行会话还没有交易所大周期 K 线缓存</div>
              )}
            </div>
            <div className="detail-grid detail-grid-compact">
              <div className="detail-item">
                <span>会话模式</span>
                <strong>{monitorMode}</strong>
              </div>
              <div className="detail-item">
                <span>账户净值</span>
                <strong>{formatMoney(monitorSummary?.netEquity)}</strong>
              </div>
              <div className="detail-item">
                <span>未实现盈亏</span>
                <strong>{formatSigned(monitorSummary?.unrealizedPnl)}</strong>
              </div>
              <div className="detail-item">
                <span>持仓方向</span>
                <strong>{String(monitorExecutionSummary.position?.side ?? "FLAT")}</strong>
              </div>
              <div className="detail-item">
                <span>持仓数量</span>
                <strong>{formatMaybeNumber(monitorExecutionSummary.position?.quantity)}</strong>
              </div>
              <div className="detail-item">
                <span>标记价格</span>
                <strong>{formatMaybeNumber(monitorExecutionSummary.position?.markPrice)}</strong>
              </div>
              <div className="detail-item">
                <span>盘口</span>
                <strong>{formatMaybeNumber(monitorMarket.bestBid)} / {formatMaybeNumber(monitorMarket.bestAsk)}</strong>
              </div>
              <div className="detail-item">
                <span>SMA5 / ATR14</span>
                <strong>{formatMaybeNumber(monitorSignalState.sma5)} / {formatMaybeNumber(monitorSignalState.atr14)}</strong>
              </div>
            </div>
            <div className="backtest-notes notes-compact">
              <div className="note-item">
                当前会话：{monitorSession ? shrink(monitorSession.id) : "--"} · 订单 {monitorExecutionSummary.orderCount} · 成交 {monitorExecutionSummary.fillCount}
              </div>
              <div className="note-item">
                最新订单：{String(monitorExecutionSummary.latestOrder?.side ?? "--")} · {String(monitorExecutionSummary.latestOrder?.status ?? "--")} · {formatTime(String(monitorExecutionSummary.latestOrder?.createdAt ?? ""))}
              </div>
              <div className="note-item">
                最新成交：{formatMaybeNumber(monitorExecutionSummary.latestFill?.price)} · 手续费 {formatMaybeNumber(monitorExecutionSummary.latestFill?.fee)} · {formatTime(String(monitorExecutionSummary.latestFill?.createdAt ?? ""))}
              </div>
            </div>
          </section>
          </div>
        ) : sidebarTab === 'strategy' ? (
          <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
            <section id="strategies" className="panel panel-backtests">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Strategies</p>
              <h3>策略管理</h3>
            </div>
            <div className="range-box">
              <span>{strategies.length} strategies</span>
              <span>{signalRuntimeAdapters.length} engines</span>
            </div>
          </div>
          <div className="live-grid">
            <div className="backtest-form session-form">
              <h4>创建策略</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>策略名称</span>
                  <input
                    value={strategyCreateForm.name}
                    onChange={(event) => setStrategyCreateForm((current: any) => ({ ...current, name: event.target.value }))}
                    placeholder="例如：BK 4H Runner"
                  />
                </label>
                <label className="form-field form-field-wide">
                  <span>策略说明</span>
                  <input
                    value={strategyCreateForm.description}
                    onChange={(event) =>
                      setStrategyCreateForm((current: any) => ({ ...current, description: event.target.value }))
                    }
                    placeholder="记录这条策略的用途、市场和执行方式"
                  />
                </label>
              </div>
              <div className="backtest-actions">
                <ActionButton
                  label={strategyCreateAction ? "创建中..." : "创建策略"}
                  disabled={strategyCreateAction || !strategyCreateForm.name.trim()}
                  onClick={createStrategy}
                />
              </div>
              <div className="backtest-notes">
                <div className="note-item">第一版先直接创建当前版本，默认引擎是 bk-default。</div>
                <div className="note-item">版本历史和回滚下一步再补，这一版先保证你能直接改参数。</div>
              </div>
            </div>

            <div className="backtest-form session-form">
              <h4>策略参数编辑</h4>
              <div className="form-grid">
                <label className="form-field">
                  <span>选择策略</span>
                  <select
                    value={strategyEditorForm.strategyId}
                    onChange={(event) => {
                      setSelectedStrategyId(event.target.value);
                      setStrategyEditorForm((current: any) => ({ ...current, strategyId: event.target.value }));
                    }}
                  >
                    {strategyOptions.map((strategy) => (
                      <option key={strategy.value} value={strategy.value}>
                        {strategy.label}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>策略引擎</span>
                  <select
                    value={strategyEditorForm.strategyEngine}
                    onChange={(event) =>
                      setStrategyEditorForm((current: any) => ({ ...current, strategyEngine: event.target.value }))
                    }
                  >
                    {[...new Set(["bk-default", ...strategies.map((item) => String(getRecord(item.currentVersion?.parameters).strategyEngine || "bk-default"))])].map((engineKey) => (
                      <option key={engineKey} value={engineKey}>
                        {engineKey}
                      </option>
                    ))}
                  </select>
                </label>
                <label className="form-field">
                  <span>信号周期</span>
                  <select
                    value={strategyEditorForm.signalTimeframe}
                    onChange={(event) =>
                      setStrategyEditorForm((current: any) => ({ ...current, signalTimeframe: event.target.value }))
                    }
                  >
                    <option value="4h">4h</option>
                    <option value="1d">1d</option>
                  </select>
                </label>
                <label className="form-field">
                  <span>执行数据源</span>
                  <select
                    value={strategyEditorForm.executionDataSource}
                    onChange={(event) =>
                      setStrategyEditorForm((current: any) => ({ ...current, executionDataSource: event.target.value }))
                    }
                  >
                    <option value="tick">tick</option>
                    <option value="1min">1min</option>
                  </select>
                </label>
                <label className="form-field form-field-wide">
                  <span>参数 JSON</span>
                  <textarea
                    rows={14}
                    value={strategyEditorForm.parametersJson}
                    onChange={(event) =>
                      setStrategyEditorForm((current: any) => ({ ...current, parametersJson: event.target.value }))
                    }
                    placeholder='{"stop_loss_atr":0.05,"profit_protect_atr":1.0}'
                  />
                </label>
              </div>
              <div className="backtest-actions inline-actions">
                <ActionButton
                  label={strategySaveAction ? "保存中..." : "保存策略参数"}
                  disabled={strategySaveAction || !strategyEditorForm.strategyId}
                  onClick={saveStrategyParameters}
                />
                <button
                  type="button"
                  className="filter-chip"
                  onClick={() => {
                    if (!selectedStrategy) {
                      return;
                    }
                    const currentParameters = getRecord(selectedStrategy.currentVersion?.parameters);
                    setStrategyEditorForm({
                      strategyId: selectedStrategy.id,
                      strategyEngine: String(currentParameters.strategyEngine ?? "bk-default"),
                      signalTimeframe: String(
                        currentParameters.signalTimeframe ??
                          selectedStrategy.currentVersion?.signalTimeframe ??
                          "1d"
                      ),
                      executionDataSource: String(
                        currentParameters.executionDataSource ??
                          currentParameters.executionTimeframe ??
                          selectedStrategy.currentVersion?.executionTimeframe ??
                          "tick"
                      ),
                      parametersJson: JSON.stringify(currentParameters, null, 2),
                    });
                  }}
                >
                  还原当前版本
                </button>
              </div>
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-list">
              <h4>策略列表</h4>
              <SimpleTable
                columns={["策略", "版本", "信号周期", "执行源", "引擎"]}
                rows={strategies.map((strategy) => {
                  const parameters = getRecord(strategy.currentVersion?.parameters);
                  return [
                    strategy.name,
                    strategy.currentVersion?.version ?? "--",
                    String(parameters.signalTimeframe ?? strategy.currentVersion?.signalTimeframe ?? "--"),
                    String(parameters.executionDataSource ?? parameters.executionTimeframe ?? strategy.currentVersion?.executionTimeframe ?? "--"),
                    String(parameters.strategyEngine ?? "bk-default"),
                  ];
                })}
                emptyMessage="暂无策略"
              />
            </div>
            <div className="backtest-list">
              <h4>当前版本摘要</h4>
              {selectedStrategy ? (
                <div className="backtest-notes">
                  <div className="note-item">
                    <strong>策略</strong> {selectedStrategy.name}
                  </div>
                  <div className="note-item">
                    <strong>说明</strong> {selectedStrategy.description || "--"}
                  </div>
                  <div className="note-item">
                    <strong>当前版本</strong> {selectedStrategyVersion?.version ?? "--"}
                  </div>
                  <div className="note-item">
                    <strong>创建时间</strong> {formatTime(selectedStrategy.createdAt)}
                  </div>
                  <div className="note-item">
                    <strong>引擎</strong> {String(selectedStrategyParameters.strategyEngine ?? "bk-default")}
                  </div>
                  <div className="note-item">
                    <strong>信号周期</strong> {String(selectedStrategyParameters.signalTimeframe ?? selectedStrategyVersion?.signalTimeframe ?? "--")}
                  </div>
                  <div className="note-item">
                    <strong>执行源</strong> {String(selectedStrategyParameters.executionDataSource ?? selectedStrategyVersion?.executionTimeframe ?? "--")}
                  </div>
                  <div className="note-item">
                    <strong>说明</strong> 这一版是直接编辑当前版本参数，不会新建版本。
                  </div>
                </div>
              ) : (
                <div className="empty-state empty-state-compact">暂无可编辑策略</div>
              )}
            </div>
          </div>
        </section>
          </div>
        ) : (
          <div className="absolute inset-0 overflow-y-auto p-6 space-y-6 bg-zinc-950/50">
            <section id="overview" className="hero">
          <div>
            <p className="eyebrow">交易主控</p>
            <h2>实盘 / 模拟统一监控与执行运行台</h2>
            <p className="hero-copy">
              当前页面直接消费平台 API，主监控优先展示正在运行的实盘会话；如果当前没有实盘会话，则回退展示模拟盘。大周期 K 线直接来自交易所信号源，执行侧信息则展示实时 tick、盘口、持仓、订单和盈亏。
            </p>
          </div>
          <div className="hero-side">
            <div className="hero-pill">{loading ? "加载中..." : `${accounts.length} 个账户`}</div>
            <div className="hero-pill hero-pill-accent">{monitorMode}</div>
            <div className="hero-user-card">
              <div>
                <strong>{authSession?.username ?? "未登录"}</strong>
                <p>{authSession?.expiresAt ? `有效至 ${formatTime(authSession.expiresAt)}` : "登录后即可加载账户与监控"}</p>
              </div>
              {authSession ? (
                <button type="button" className="hero-menu-button hero-logout" onClick={logout}>
                  退出
                </button>
              ) : null}
            </div>
            <div className="hero-menu">
              <button
                type="button"
                className="hero-menu-button"
                onClick={() => setSettingsMenuOpen((current: any) => !current)}
              >
                设置
              </button>
              {settingsMenuOpen ? (
                <div className="hero-menu-popover">
                  <button
                    type="button"
                    className="hero-menu-item"
                    onClick={() => {
                      openLiveAccountModal();
                      setSettingsMenuOpen(false);
                    }}
                  >
                    新建账户
                  </button>
                  <button
                    type="button"
                    className="hero-menu-item"
                    onClick={() => {
                      if (quickLiveAccountId) {
                        selectQuickLiveAccount(quickLiveAccountId);
                      }
                      openLiveBindingModal();
                      setSettingsMenuOpen(false);
                    }}
                  >
                    绑定账户
                  </button>
                  <button
                    type="button"
                    className="hero-menu-item"
                    onClick={() => {
                      setActiveSettingsModal("telegram");
                      setSettingsMenuOpen(false);
                    }}
                  >
                    Telegram 通知
                  </button>
                </div>
              ) : null}
            </div>
          </div>
        </section>
            <section id="live" className="panel panel-session">
          <div className="panel-header">
            <div>
              <p className="panel-kicker">Live Trading</p>
              <h3>实盘账户与订单同步</h3>
            </div>
          </div>
          {highlightedLiveSession ? (
            <div className="live-grid">
              <div className="session-card session-card-primary">
                <div className="session-card-header">
                  <div>
                    <p className="panel-kicker">Live Overview</p>
                    <h4>当前优先处理会话</h4>
                  </div>
                  <StatusPill tone={liveSessionHealthTone(highlightedLiveSession.health.status)}>
                    {highlightedLiveSession.health.status}
                  </StatusPill>
                </div>
                <div className="live-account-meta">
                  <span>{shrink(highlightedLiveSession.session.id)}</span>
                  <span>{highlightedLiveSession.session.accountId}</span>
                  <span>{highlightedLiveSession.session.strategyId}</span>
                  <span>{String(highlightedLiveSession.session.state?.signalTimeframe ?? "--")}</span>
                </div>
                <div className="backtest-notes">
                  <div className="note-item">health: {highlightedLiveSession.health.detail}</div>
                  <div className="note-item">
                    recovery: {String(highlightedLiveSession.session.state?.positionRecoveryStatus ?? "--")} · protection{" "}
                    {String(highlightedLiveSession.session.state?.protectionRecoveryStatus ?? "--")} · orders{" "}
                    {String(highlightedLiveSession.session.state?.recoveredProtectionCount ?? "--")}
                  </div>
                  <div className="note-item">
                    execution: orders {highlightedLiveSession.execution.orderCount} · fills {highlightedLiveSession.execution.fillCount}
                  </div>
                  <div className="note-item">
                    latest-order: {String(highlightedLiveSession.execution.latestOrder?.status ?? "--")} · {String(highlightedLiveSession.execution.latestOrder?.side ?? "--")} · {formatMaybeNumber(highlightedLiveSession.execution.latestOrder?.price)}
                  </div>
                  <div className="note-item">
                    position: {String(highlightedLiveSession.execution.position?.side ?? "FLAT")} · {formatMaybeNumber(highlightedLiveSession.execution.position?.quantity)} @ {formatMaybeNumber(highlightedLiveSession.execution.position?.entryPrice)}
                  </div>
                </div>
                <div className="flow-row">
                  {highlightedLiveSessionFlow.map((step) => (
                    <div key={step.key} className="flow-step">
                      <StatusPill tone={step.status}>{step.label}</StatusPill>
                      <span>{step.detail}</span>
                    </div>
                  ))}
                </div>
              </div>
            </div>
          ) : null}
          <div className="live-grid">
            <div className="backtest-form session-form panel-compact">
              <h4>配置入口已收进弹窗</h4>
              <div className="backtest-notes notes-compact">
                <div className="note-item">首屏顶部提供账户下拉、创建账户、绑定适配器、创建会话和一键拉起。</div>
                <div className="note-item">当前选中账户：{quickLiveAccount?.name ?? "--"} · {quickLiveAccount?.status ?? "--"} · {quickLiveAccount?.exchange ?? "--"}</div>
                <div className="note-item">sandbox=true 时默认从 `.env` 读取 `BINANCE_TESTNET_API_KEY` / `BINANCE_TESTNET_API_SECRET`。</div>
              </div>
              <div className="backtest-actions inline-actions">
                <ActionButton label="新建账户" variant="ghost" onClick={openLiveAccountModal} />
                <ActionButton
                  label="绑定适配器"
                  variant="ghost"
                  disabled={!quickLiveAccountId}
                  onClick={() => {
                    if (quickLiveAccountId) {
                      selectQuickLiveAccount(quickLiveAccountId);
                    }
                    openLiveBindingModal();
                  }}
                />
                <ActionButton
                  label="创建会话"
                  variant="ghost"
                  disabled={!quickLiveAccountId}
                  onClick={() => {
                    if (quickLiveAccountId) {
                      selectQuickLiveAccount(quickLiveAccountId);
                    }
                    openLiveSessionModal();
                  }}
                />
              </div>
            </div>

            <div className="backtest-list">
              <h4>Live Strategy Sessions</h4>
              <div className="backtest-notes notes-compact">
                <div className="note-item">有效会话：{validLiveSessions.length}</div>
              </div>
              {validLiveSessions.length > 0 ? (
                <div className="live-card-list">
                  {validLiveSessions.map((session) => {
                    const intent = getRecord(session.state?.lastStrategyIntent);
                    const executionSummary = deriveLiveSessionExecutionSummary(session, orders, fills, positions);
                    const sessionHealth = deriveLiveSessionHealth(session, executionSummary);
                    const sessionAccount = liveAccounts.find((account) => account.id === session.accountId) ?? null;
                    const sessionBinding = getRecord(sessionAccount?.metadata?.liveBinding);
                    const sessionAccountReady =
                      sessionAccount?.status === "CONFIGURED" ||
                      sessionAccount?.status === "READY" ||
                      String(sessionBinding.connectionMode ?? "") !== "" && String(sessionBinding.connectionMode ?? "") !== "disabled";
                    return (
                      <div key={session.id} className="session-row">
                        <div className="session-row-main">
                          <div className="session-row-title">
                            <strong>{shrink(session.id)}</strong>
                            <StatusPill tone={liveSessionHealthTone(sessionHealth.status)}>{sessionHealth.status}</StatusPill>
                            <StatusPill tone={session.status === "RUNNING" ? "ready" : session.status === "STOPPED" ? "watch" : "neutral"}>
                              {session.status}
                            </StatusPill>
                          </div>
                          <div className="live-account-meta session-row-meta">
                            <span>{session.accountId}</span>
                            <span>{strategyLabel(strategies.find((strategy) => strategy.id === session.strategyId))}</span>
                            <span>{String(session.state?.signalTimeframe ?? "--")}</span>
                            <span>{sessionAccount?.status ?? "--"}</span>
                            <span>{String(intent.action ?? "no-intent")}</span>
                            <span>{String(executionSummary.position?.side ?? "FLAT")}</span>
                            <span>{formatMaybeNumber(executionSummary.position?.quantity)}</span>
                            <span>{executionSummary.orderCount}/{executionSummary.fillCount}</span>
                            {!sessionAccountReady ? <span>先绑定适配器</span> : null}
                          </div>
                        </div>
                        <div className="session-row-actions inline-actions">
                          <ActionButton
                            label="Edit"
                            variant="ghost"
                            disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                            onClick={() => openLiveSessionModal(session)}
                          />
                          {String(session.state?.signalRuntimeSessionId ?? "") ? (
                            <ActionButton
                              label="Open Runtime"
                              variant="ghost"
                              onClick={() => jumpToSignalRuntimeSession(String(session.state?.signalRuntimeSessionId ?? ""))}
                            />
                          ) : null}
                          <ActionButton
                            label={liveSessionAction === `${session.id}:start` ? "Starting..." : "Start"}
                            disabled={liveSessionAction !== null || session.status === "RUNNING" || !sessionAccountReady}
                            onClick={() => runLiveSessionAction(session.id, "start")}
                          />
                          {!sessionAccountReady ? (
                            <ActionButton
                              label="Bind Adapter"
                              variant="ghost"
                              disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                              onClick={() => {
                                selectQuickLiveAccount(session.accountId);
                                openLiveBindingModal();
                              }}
                            />
                          ) : null}
                          <ActionButton
                            label={liveSessionAction === `${session.id}:dispatch` ? "Dispatching..." : "Dispatch Intent"}
                            disabled={
                              liveSessionAction !== null ||
                              !getRecord(session.state?.lastStrategyIntent).action ||
                              String(session.state?.dispatchMode ?? "") !== "manual-review" ||
                              (primaryLiveSession?.id === session.id && primaryLiveDispatchPreview.status === "blocked")
                            }
                            onClick={() => dispatchLiveSessionIntent(session.id)}
                          />
                          <ActionButton
                            label={liveSessionAction === `${session.id}:sync` ? "Syncing..." : "Sync Latest Order"}
                            variant="ghost"
                            disabled={liveSessionAction !== null || !String(session.state?.lastDispatchedOrderId ?? "")}
                            onClick={() => syncLiveSession(session.id)}
                          />
                          <ActionButton
                            label={liveSessionAction === `${session.id}:stop` ? "Stopping..." : "Stop"}
                            variant="ghost"
                            disabled={liveSessionAction !== null || session.status === "STOPPED"}
                            onClick={() => runLiveSessionAction(session.id, "stop")}
                          />
                          <ActionButton
                            label={liveSessionDeleteAction === session.id ? "Deleting..." : "Delete"}
                            variant="ghost"
                            disabled={liveSessionAction !== null || liveSessionDeleteAction !== null}
                            onClick={() => void deleteLiveSession(session.id)}
                          />
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="empty-state empty-state-compact">No valid live sessions yet</div>
              )}
              {primaryLiveSession ? (
                <div className="backtest-notes">
                  <div className="note-item">
                    runtime: {String(primaryLiveSession.state?.signalRuntimeStatus ?? "--")} · {formatTime(String(primaryLiveSession.state?.lastSignalRuntimeEventAt ?? ""))}
                  </div>
                  <div className="note-item">
                    market: {formatMaybeNumber(primaryLiveSessionMarket.tradePrice)} · {formatMaybeNumber(primaryLiveSessionMarket.bestBid)} / {formatMaybeNumber(primaryLiveSessionMarket.bestAsk)}
                  </div>
                  <div className="note-item">
                    readiness: {primaryLiveSessionRuntimeReadiness.status} · {primaryLiveSessionRuntimeReadiness.reason}
                  </div>
                  <div className="note-item">
                    intent: {String(primaryLiveSessionIntent.action ?? "none")} · {String(primaryLiveSessionIntent.side ?? "--")} · {formatMaybeNumber(primaryLiveSessionIntent.priceHint)}
                  </div>
                  <div className="note-item">
                    intent-preview: qty {formatMaybeNumber(primaryLiveSessionIntent.quantity)} · src {String(primaryLiveSessionIntent.priceSource ?? "--")} · kind {String(primaryLiveSessionIntent.signalKind ?? "--")}
                  </div>
                  <div className="note-item">
                    intent-context: spread {formatMaybeNumber(primaryLiveSessionIntent.spreadBps)} bps · bias {String(primaryLiveSessionIntent.liquidityBias ?? "--")} · ma20 {formatMaybeNumber(primaryLiveSessionIntent.ma20)} · atr14 {formatMaybeNumber(primaryLiveSessionIntent.atr14)}
                  </div>
                  <div className="note-item">
                    signal-filter: tf {String(primaryLiveSessionSignalBarDecision.timeframe ?? "--")} · sma5 {formatMaybeNumber(primaryLiveSessionSignalBarDecision.sma5)} · early-long{" "}
                    {boolLabel(primaryLiveSessionSignalBarDecision.longEarlyReversalReady)} · early-short {boolLabel(primaryLiveSessionSignalBarDecision.shortEarlyReversalReady)} ·{" "}
                    {String(primaryLiveSessionSignalBarDecision.reason ?? "--")}
                  </div>
                  <div className="note-item">
                    dispatch: {String(primaryLiveSession?.state?.dispatchMode ?? "--")} · cooldown {String(primaryLiveSession?.state?.dispatchCooldownSeconds ?? "--")}s · last-order {String(primaryLiveSession?.state?.lastDispatchedOrderId ?? "--")}
                  </div>
                  <div className="note-item">
                    execution-profile: {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).executionProfile ?? "--")} ·{" "}
                    {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).orderType ?? "--")} · tif{" "}
                    {String(getRecord(primaryLiveSession?.state?.lastExecutionProfile).timeInForce ?? "--")} · postOnly{" "}
                    {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionProfile).postOnly)} · reduceOnly{" "}
                    {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionProfile).reduceOnly)}
                  </div>
                  <div className="note-item">
                    execution-telemetry: {String(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).decision ?? "--")} · spread{" "}
                    {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).spreadBps)} bps · imbalance{" "}
                    {formatMaybeNumber(getRecord(getRecord(primaryLiveSession?.state?.lastExecutionTelemetry).book).bookImbalance)}
                  </div>
                  <div className="note-item">
                    execution-dispatch: {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).status ?? "--")} ·{" "}
                    {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).executionMode ?? "--")} · fallback{" "}
                    {boolLabel(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).fallback)} ·{" "}
                    {String(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).fallbackOrderType ?? "--")}
                  </div>
                  <div className="note-item">
                    execution-fill: expected {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).expectedPrice)} · drift{" "}
                    {formatMaybeNumber(getRecord(primaryLiveSession?.state?.lastExecutionDispatch).priceDriftBps)} bps
                  </div>
                  <div className="note-item">
                    execution-event-stats: proposals {String(getRecord(primaryLiveSession?.state?.executionEventStats).proposalCount ?? "--")} · maker{" "}
                    {String(getRecord(primaryLiveSession?.state?.executionEventStats).makerRestingDecisionCount ?? "--")} · fallback{" "}
                    {String(getRecord(primaryLiveSession?.state?.executionEventStats).fallbackDispatchCount ?? "--")} · avg drift{" "}
                    {formatMaybeNumber(getRecord(primaryLiveSession?.state?.executionEventStats).avgPriceDriftBps)} bps
                  </div>
                  <div className="note-item">
                    auto-dispatch: last-at {formatTime(String(primaryLiveSession?.state?.lastDispatchedAt ?? ""))} · last-error {String(primaryLiveSession?.state?.lastAutoDispatchError ?? "--")}
                  </div>
                  <div className="note-item">
                    sync: {String(primaryLiveSession?.state?.lastSyncedOrderStatus ?? "--")} · {formatTime(String(primaryLiveSession?.state?.lastSyncedAt ?? ""))} · error {String(primaryLiveSession?.state?.lastSyncError ?? "--")}
                  </div>
                  <div className="note-item">
                    recovery: {String(primaryLiveSession?.state?.lastRecoveryStatus ?? "--")} · position {String(primaryLiveSession?.state?.positionRecoveryStatus ?? "--")} · protection{" "}
                    {String(primaryLiveSession?.state?.protectionRecoveryStatus ?? "--")}
                  </div>
                  <div className="note-item">
                    recovery-detail: last-at {formatTime(String(primaryLiveSession?.state?.lastRecoveryAttemptAt ?? primaryLiveSession?.state?.lastProtectionRecoveryAt ?? ""))} · protection-orders{" "}
                    {String(primaryLiveSession?.state?.recoveredProtectionCount ?? "--")} · stop {String(primaryLiveSession?.state?.recoveredStopOrderCount ?? "--")} · take-profit{" "}
                    {String(primaryLiveSession?.state?.recoveredTakeProfitOrderCount ?? "--")}
                  </div>
                  <div className="note-item">
                    execution: orders {primaryLiveExecutionSummary.orderCount} · fills {primaryLiveExecutionSummary.fillCount} · latest-order {String(primaryLiveExecutionSummary.latestOrder?.status ?? "--")}
                  </div>
                  <div className="note-item">
                    position: {String(primaryLiveExecutionSummary.position?.side ?? "FLAT")} · {formatMaybeNumber(primaryLiveExecutionSummary.position?.quantity)} @ {formatMaybeNumber(primaryLiveExecutionSummary.position?.entryPrice)} · mark {formatMaybeNumber(primaryLiveExecutionSummary.position?.markPrice)}
                  </div>
                  <div className="note-item">
                    recovered-position: {String(getRecord(primaryLiveSession?.state?.recoveredPosition).side ?? "FLAT")} ·{" "}
                    {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).quantity)} @{" "}
                    {formatMaybeNumber(getRecord(primaryLiveSession?.state?.recoveredPosition).entryPrice)}
                  </div>
                  <div className="note-item">
                    dispatch-preview: {primaryLiveDispatchPreview.reason} · {primaryLiveDispatchPreview.detail}
                  </div>
                  <div className="note-item">
                    final-order: {String(primaryLiveDispatchPreview.payload.side ?? "--")} {formatMaybeNumber(primaryLiveDispatchPreview.payload.quantity)} {String(primaryLiveDispatchPreview.payload.symbol ?? "--")} · {String(primaryLiveDispatchPreview.payload.type ?? "--")} @ {formatMaybeNumber(primaryLiveDispatchPreview.payload.price)}
                  </div>
                  {buildTimelineNotes(primaryLiveSessionTimeline).slice(0, 4).map((line) => (
                    <div key={line} className="note-item">
                      {line}
                    </div>
                  ))}
                </div>
              ) : null}
            </div>
          </div>

          <div className="live-grid">
            <div className="backtest-list">
              <h4>Live Accounts</h4>
              {liveAccounts.length > 0 ? (
                <div className="live-card-list">
                  {liveAccounts.map((account) => {
                    const binding = (account.metadata?.liveBinding as Record<string, unknown> | undefined) ?? {};
                    const syncSnapshot = getRecord(getRecord(account.metadata).liveSyncSnapshot);
                    const bindings = accountSignalBindingMap[account.id] ?? [];
                    const runtimeSessionsForAccount = signalRuntimeSessions.filter((item) => item.accountId === account.id);
                    const activeRuntime = runtimeSessionsForAccount.find((item) => item.status === "RUNNING") ?? runtimeSessionsForAccount[0] ?? null;
                    const activeRuntimeState = getRecord(activeRuntime?.state);
                    const activeRuntimeSummary = getRecord(activeRuntimeState.lastEventSummary);
                    const activeRuntimeMarket = deriveRuntimeMarketSnapshot(getRecord(activeRuntimeState.sourceStates), activeRuntimeSummary);
                    const activeRuntimeSourceSummary = deriveRuntimeSourceSummary(
                      getRecord(activeRuntimeState.sourceStates),
                      runtimePolicy
                    );
                    const activeSignalBarState = derivePrimarySignalBarState(getRecord(activeRuntimeState.signalBarStates));
                    const activeSignalAction = deriveSignalActionSummary(activeSignalBarState);
                    const activeRuntimeTimeline = getList(activeRuntimeState.timeline);
                    const activeRuntimeReadiness = deriveRuntimeReadiness(activeRuntimeState, activeRuntimeSourceSummary, {
                      requireTick: bindings.some((item) => item.streamType === "trade_tick"),
                      requireOrderBook: bindings.some((item) => item.streamType === "order_book"),
                    });
                    const livePreflight = deriveLivePreflightSummary(
                      account,
                      bindings,
                      runtimeSessionsForAccount,
                      activeRuntime,
                      activeRuntimeReadiness
                    );
                    const liveNextAction = deriveLiveNextAction(livePreflight);
                    const liveAlerts = deriveLiveAlerts(
                      account,
                      activeRuntimeState,
                      activeRuntimeSourceSummary,
                      activeRuntimeReadiness,
                      activeSignalAction,
                      runtimePolicy
                    );
                    return (
                      <div key={account.id} className="session-stat">
                        <span>{account.name}</span>
                        <strong>{account.status}</strong>
                        <div className="live-account-meta">
                          <span>{account.exchange}</span>
                          <span>{String(binding.adapterKey ?? "--")}</span>
                          <span>{String(binding.positionMode ?? "--")} / {String(binding.marginMode ?? "--")}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>sync {String(syncSnapshot.syncStatus ?? "UNSYNCED")}</span>
                          <span>{formatTime(String(getRecord(account.metadata).lastLiveSyncAt ?? ""))}</span>
                          <span>{String(syncSnapshot.source ?? "--")}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>{bindings.length} signal bindings</span>
                          <span>{runtimeSessionsForAccount.length} runtime sessions</span>
                          <span>{activeRuntime ? `${activeRuntime.status} · ${String(activeRuntimeState.health ?? "--")}` : "no runtime"}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>{String(activeRuntimeSummary.event ?? "--")}</span>
                          <span>{formatTime(String(activeRuntimeState.lastHeartbeatAt ?? ""))}</span>
                          <span>{formatTime(String(activeRuntimeState.lastEventAt ?? ""))}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>trade {formatMaybeNumber(activeRuntimeMarket.tradePrice)}</span>
                          <span>bid/ask {formatMaybeNumber(activeRuntimeMarket.bestBid)} / {formatMaybeNumber(activeRuntimeMarket.bestAsk)}</span>
                          <span>spread {formatMaybeNumber(activeRuntimeMarket.spreadBps)} bps</span>
                        </div>
                        <div className="live-account-meta">
                          <span>tick {activeRuntimeSourceSummary.tradeTickCount}</span>
                          <span>book {activeRuntimeSourceSummary.orderBookCount}</span>
                          <span>stale {activeRuntimeSourceSummary.staleCount}</span>
                          <span>{formatTime(String(activeRuntimeSourceSummary.latestEventAt ?? ""))}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>
                            <StatusPill tone={runtimeReadinessTone(activeRuntimeReadiness.status)}>
                              {activeRuntimeReadiness.status}
                            </StatusPill>
                          </span>
                          <span>{activeRuntimeReadiness.reason}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>
                            <StatusPill tone={runtimeReadinessTone(livePreflight.status)}>
                              {livePreflight.status}
                            </StatusPill>
                          </span>
                          <span>{livePreflight.reason}</span>
                          <span>{livePreflight.detail}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>next action</span>
                          <span>{liveNextAction.label}</span>
                          <span>{liveNextAction.detail}</span>
                          <button
                            type="button"
                            className="filter-chip"
                            disabled={
                              liveFlowAction !== null ||
                              liveBindAction ||
                              signalBindingAction !== null ||
                              signalRuntimeAction !== null ||
                              liveSessionAction !== null ||
                              liveSessionCreateAction ||
                              liveSessionLaunchAction
                            }
                            onClick={() => launchLiveFlow(account)}
                          >
                            {liveFlowAction === account.id ? "Launching..." : "Launch Live Flow"}
                          </button>
                          <button
                            type="button"
                            className="filter-chip"
                            onClick={() => runLiveNextAction(account, liveNextAction, activeRuntime)}
                          >
                            Open
                          </button>
                        </div>
                        <div className="live-account-meta">
                          <span>{String(activeSignalBarState.timeframe ?? "--")}</span>
                          <span>ma20 {formatMaybeNumber(activeSignalBarState.ma20)}</span>
                          <span>atr14 {formatMaybeNumber(activeSignalBarState.atr14)}</span>
                        </div>
                        <div className="live-account-meta">
                          <span>
                            <StatusPill tone={signalActionTone(activeSignalAction.bias, activeSignalAction.state)}>
                              {activeSignalAction.bias}
                            </StatusPill>
                          </span>
                          <span>
                            <StatusPill tone={decisionStateTone(activeSignalAction.state)}>
                              {activeSignalAction.state}
                            </StatusPill>
                          </span>
                          <span>{activeSignalAction.reason}</span>
                        </div>
                        <div className="backtest-notes">
                          {buildAlertNotes(liveAlerts).map((item) => (
                            <div key={`${account.id}-${item.title}-${item.detail}`} className={`note-item note-item-alert note-item-alert-${item.level}`}>
                              <strong>{item.title}</strong> {item.detail}
                            </div>
                          ))}
                          {buildSignalActionNotes(activeSignalAction).map((line) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                          <div className="note-item">
                            live-preflight: {livePreflight.reason} · {livePreflight.detail}
                          </div>
                          <div className="note-item">
                            next-action: {liveNextAction.label} · {liveNextAction.detail}
                          </div>
                          <div className="note-item">
                            account-sync: orders {String(syncSnapshot.orderCount ?? "--")} · fills {String(syncSnapshot.fillCount ?? "--")} · positions {String(syncSnapshot.positionCount ?? "--")}
                          </div>
                          {buildSignalBarStateNotes(activeSignalBarState).map((line) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                          {buildRuntimeEventNotes(activeRuntimeSummary).map((line) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                          {buildSourceStateNotes(getRecord(activeRuntimeState.sourceStates)).map((line) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                          {buildTimelineNotes(activeRuntimeTimeline).slice(0, 3).map((line) => (
                            <div key={line} className="note-item">
                              {line}
                            </div>
                          ))}
                        </div>
                        <div className="inline-actions">
                          <ActionButton
                            label={liveAccountSyncAction === account.id ? "Syncing..." : "Sync Account"}
                            variant="ghost"
                            disabled={liveAccountSyncAction !== null}
                            onClick={() => syncLiveAccount(account.id)}
                          />
                          {activeRuntime ? (
                            <ActionButton
                              label="Open Runtime"
                              variant="ghost"
                              onClick={() => jumpToSignalRuntimeSession(activeRuntime.id)}
                            />
                          ) : null}
                        </div>
                      </div>
                    );
                  })}
                </div>
              ) : (
                <div className="empty-state empty-state-compact">No live accounts yet</div>
              )}
            </div>

            <div className="backtest-list">
              <h4>Accepted Live Orders</h4>
              {syncableLiveOrders.length > 0 ? (
                <SimpleTable
                  columns={["Order", "Account", "Symbol", "Side", "Qty", "Status", "Action"]}
                  rows={syncableLiveOrders.map((order) => [
                    shrink(order.id),
                    order.accountId,
                    order.symbol,
                    order.side,
                    formatMaybeNumber(order.quantity),
                    order.status,
                    <ActionButton
                      key={order.id}
                      label={liveSyncAction === order.id ? "Syncing..." : "Sync"}
                      disabled={liveSyncAction !== null}
                      onClick={() => syncLiveOrder(order.id)}
                    />,
                  ])}
                  emptyMessage="No accepted live orders"
                />
              ) : (
                <div className="empty-state empty-state-compact">No accepted live orders</div>
              )}
            </div>
          </div>
        </section>
          </div>
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
