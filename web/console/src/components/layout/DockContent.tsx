import React, { useState, useMemo, useEffect } from 'react';
import { Badge } from '../ui/badge';
import { Button } from '../ui/button';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../ui/table';
import { Tooltip, TooltipContent, TooltipTrigger } from '../ui/tooltip';
import { Input } from '../ui/input';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from '../ui/select';
import { formatTime, formatMaybeNumber, shrink, formatSigned } from '../../utils/format';
import { technicalStatusLabel } from '../../utils/derivation';
import { useTradingStore } from '../../store/useTradingStore';
import { useUIStore } from '../../store/useUIStore';
import { useLiveTradePairs } from '../../hooks/useLiveTradePairs';
import { useOrdersPageQuery } from '../../hooks/useOrdersPageQuery';
import { useFillsPageQuery } from '../../hooks/useFillsPageQuery';
import { ShieldCheck, Loader2, ChevronLeft, ChevronRight, Activity, AlertCircle, FileSearch } from 'lucide-react';
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogDescription, 
  DialogFooter,
  DialogClose
} from '../ui/dialog';
import { FillSyncModal } from '../../modals/FillSyncModal';
import { ManualTradeReviewDialog } from '../live/ManualTradeReviewDialog';

import { cn } from '../../lib/utils';
import { fetchJSON } from '../../utils/api';
import type { Order } from '../../types/domain';

interface DockContentProps {
  dockTab: 'pairs' | 'orders' | 'positions' | 'fills' | 'alerts';
  actions: any;
  sessionId?: string | null;
}

type UnifiedLogEvent = {
  id: string;
  type: string;
  title: string;
  message: string;
  eventTime: string;
  recordedAt?: string;
  liveSessionId?: string;
  runtimeSessionId?: string;
  decisionEventId?: string;
  metadata?: Record<string, unknown>;
  payload?: Record<string, unknown>;
};

type UnifiedLogEventPage = {
  items?: UnifiedLogEvent[];
  nextCursor?: string;
};

type DecisionTraceStatus = "idle" | "loading" | "loaded" | "missing" | "error";

type DecisionTrace = {
  event: UnifiedLogEvent;
  payload: Record<string, unknown>;
  decisionMetadata: Record<string, unknown>;
  signalBarDecision: Record<string, unknown>;
  signalBarState: Record<string, unknown> | null;
  signalBarStateKey: string;
  breakoutProof: BreakoutProof | null;
};

type BreakoutProof = {
  barTime?: string;
  close?: number;
  eventAt?: string;
  level?: number;
  price?: number;
  priceSource?: string;
  side?: string;
  signalBarStateKey?: string;
  source?: string;
  timeframe?: string;
};

type LiveSessionBreakoutDetail = {
  state?: {
    breakoutHistory?: unknown[];
  };
};

function tradePairStatusLabel(status: string) {
  return String(status).toLowerCase() === 'open' ? '持仓中' : '已平仓';
}

function tradePairVerdictLabel(verdict: string) {
  switch (String(verdict).toLowerCase()) {
    case 'normal':
      return '正常退出';
    case 'recovery-close':
      return 'Recovery 平仓';
    case 'orphan-exit':
      return '孤儿退出';
    case 'mismatch':
      return '需复核';
    default:
      return '进行中';
  }
}

function tradePairVerdictTone(verdict: string) {
  switch (String(verdict).toLowerCase()) {
    case 'normal':
      return 'text-[var(--bk-status-success)]';
    case 'mismatch':
    case 'orphan-exit':
      return 'text-[var(--bk-status-danger)]';
    case 'recovery-close':
      return 'text-[var(--bk-status-warning)]';
    default:
      return 'text-[var(--bk-text-muted)]';
  }
}

function asRecord(value: unknown): Record<string, unknown> | null {
  if (!value || typeof value !== "object" || Array.isArray(value)) {
    return null;
  }
  return value as Record<string, unknown>;
}

function firstText(...values: unknown[]) {
  for (const value of values) {
    const text = String(value ?? "").trim();
    if (text !== "") {
      return text;
    }
  }
  return "";
}

function nestedRecord(root: Record<string, unknown> | null | undefined, key: string) {
  return asRecord(root?.[key]);
}

function formatDecisionValue(value: unknown, maxDigits = 8) {
  if (typeof value === "boolean") {
    return value ? "true" : "false";
  }
  if (typeof value === "number" && Number.isFinite(value)) {
    return formatMaybeNumber(value, maxDigits);
  }
  const text = String(value ?? "").trim();
  if (text === "") {
    return "--";
  }
  const numeric = Number(text);
  if (Number.isFinite(numeric) && text !== "") {
    return formatMaybeNumber(numeric, maxDigits);
  }
  return text;
}

function finiteNumber(value: unknown) {
  if (typeof value === "number" && Number.isFinite(value)) {
    return value;
  }
  const parsed = Number(String(value ?? "").trim());
  return Number.isFinite(parsed) ? parsed : null;
}

function differenceValue(left: unknown, right: unknown) {
  const leftNumber = finiteNumber(left);
  const rightNumber = finiteNumber(right);
  if (leftNumber == null || rightNumber == null) {
    return null;
  }
  return leftNumber - rightNumber;
}

function samePrice(left: unknown, right: unknown) {
  const leftNumber = finiteNumber(left);
  const rightNumber = finiteNumber(right);
  if (leftNumber == null || rightNumber == null) {
    return false;
  }
  return Math.abs(leftNumber - rightNumber) < 0.0000001;
}

function formatBarTimestamp(value: unknown) {
  const text = firstText(value);
  if (!text) {
    return "--";
  }
  const numeric = Number(text);
  if (Number.isFinite(numeric) && numeric > 0) {
    const millis = numeric > 1_000_000_000_000 ? numeric : numeric * 1000;
    return formatTime(new Date(millis).toISOString());
  }
  const formatted = formatTime(text);
  return formatted === "--" ? text : formatted;
}

function timestampMillis(value: unknown) {
  const text = firstText(value);
  if (!text) {
    return null;
  }
  const numeric = Number(text);
  if (Number.isFinite(numeric) && numeric > 0) {
    return numeric > 1_000_000_000_000 ? numeric : numeric * 1000;
  }
  const parsed = Date.parse(text);
  return Number.isFinite(parsed) ? parsed : null;
}

function orderExecutionProposal(order: Order | null | undefined) {
  return nestedRecord(asRecord(order?.metadata), "executionProposal");
}

function orderDecisionEventId(order: Order | null | undefined) {
  const metadata = asRecord(order?.metadata);
  const proposal = orderExecutionProposal(order);
  const proposalMetadata = nestedRecord(proposal, "metadata");
  const bindings = asRecord(order?.bindings);
  return firstText(
    metadata?.decisionEventId,
    proposal?.decisionEventId,
    proposalMetadata?.decisionEventId,
    bindings?.decisionEventId
  );
}

function orderSignalSummary(order: Order | null | undefined) {
  const metadata = asRecord(order?.metadata);
  const proposal = orderExecutionProposal(order);
  const proposalMetadata = nestedRecord(proposal, "metadata");
  return {
    role: firstText(proposal?.role, proposalMetadata?.nextPlannedRole),
    reason: firstText(proposal?.reason, metadata?.reason, metadata?.signalKind),
    signalKind: firstText(proposal?.signalKind, metadata?.signalKind),
  };
}

function normalizeBreakoutProof(value: unknown): BreakoutProof | null {
  const item = asRecord(value);
  if (!item) {
    return null;
  }
  return {
    barTime: firstText(item.barTime),
    close: finiteNumber(item.close) ?? undefined,
    eventAt: firstText(item.eventAt),
    level: finiteNumber(item.level) ?? undefined,
    price: finiteNumber(item.price) ?? undefined,
    priceSource: firstText(item.priceSource),
    side: firstText(item.side),
    signalBarStateKey: firstText(item.signalBarStateKey),
    source: firstText(item.source),
    timeframe: firstText(item.timeframe),
  };
}

function findBreakoutProof(
  event: UnifiedLogEvent,
  trace: Omit<DecisionTrace, "breakoutProof">,
  history: unknown[] | undefined
) {
  if (!history?.length) {
    return null;
  }
  const proposal = nestedRecord(trace.payload, "executionProposal");
  const current = nestedRecord(trace.signalBarDecision, "current");
  const eventMillis = timestampMillis(event.eventTime) ?? Number.POSITIVE_INFINITY;
  const side = firstText(proposal?.side, trace.decisionMetadata.nextPlannedSide).toUpperCase();
  const level = trace.signalBarDecision.longBreakoutLevel;
  const barStartMillis = timestampMillis(current?.barStart);
  const signalBarKey = trace.signalBarStateKey;

  const candidates = history
    .map(normalizeBreakoutProof)
    .filter((proof): proof is BreakoutProof => {
      if (!proof) {
        return false;
      }
      const proofMillis = timestampMillis(proof.eventAt);
      if (proofMillis != null && proofMillis > eventMillis + 1000) {
        return false;
      }
      if (side && firstText(proof.side).toUpperCase() !== side) {
        return false;
      }
      if (signalBarKey && proof.signalBarStateKey && proof.signalBarStateKey !== signalBarKey) {
        return false;
      }
      if (finiteNumber(level) != null && !samePrice(proof.level, level)) {
        return false;
      }
      const proofBarMillis = timestampMillis(proof.barTime);
      if (barStartMillis != null && proofBarMillis != null && proofBarMillis !== barStartMillis) {
        return false;
      }
      return true;
    })
    .sort((left, right) => (timestampMillis(right.eventAt) ?? 0) - (timestampMillis(left.eventAt) ?? 0));

  return candidates[0] ?? null;
}

function resolveSignalBarState(
  payload: Record<string, unknown>,
  decisionMetadata: Record<string, unknown>
) {
  const directState = asRecord(decisionMetadata.signalBarState);
  const proposal = nestedRecord(payload, "executionProposal");
  const proposalMetadata = nestedRecord(proposal, "metadata");
  const signalBarStateKey = firstText(
    decisionMetadata.signalBarStateKey,
    proposal?.signalBarStateKey,
    proposalMetadata?.signalBarStateKey
  );

  if (directState) {
    return { state: directState, key: signalBarStateKey };
  }

  const signalBarStates = nestedRecord(payload, "signalBarStates");
  if (!signalBarStates) {
    return { state: null, key: signalBarStateKey };
  }

  if (signalBarStateKey) {
    const keyedState = asRecord(signalBarStates[signalBarStateKey]);
    if (keyedState) {
      return { state: keyedState, key: signalBarStateKey };
    }
  }

  const evaluationContext = nestedRecord(payload, "evaluationContext");
  const symbol = firstText(payload.symbol, decisionMetadata.symbol).toUpperCase();
  const timeframe = firstText(evaluationContext?.signalTimeframe, proposal?.signalTimeframe).toLowerCase();
  for (const [key, value] of Object.entries(signalBarStates)) {
    const state = asRecord(value);
    if (!state) {
      continue;
    }
    const stateSymbol = firstText(state.symbol).toUpperCase();
    const stateTimeframe = firstText(state.timeframe).toLowerCase();
    if ((!symbol || stateSymbol === symbol) && (!timeframe || stateTimeframe === timeframe)) {
      return { state, key };
    }
  }

  const [firstKey, firstValue] = Object.entries(signalBarStates)[0] ?? [];
  return { state: asRecord(firstValue), key: firstText(firstKey) };
}

function buildDecisionTrace(event: UnifiedLogEvent, breakoutProof: BreakoutProof | null = null): DecisionTrace {
  const payload = asRecord(event.payload) ?? {};
  const decisionMetadata = nestedRecord(payload, "decisionMetadata") ?? {};
  const signalBarDecision = nestedRecord(decisionMetadata, "signalBarDecision") ?? {};
  const { state, key } = resolveSignalBarState(payload, decisionMetadata);
  return {
    event,
    payload,
    decisionMetadata,
    signalBarDecision,
    signalBarState: state,
    signalBarStateKey: key,
    breakoutProof,
  };
}

function TruncatedValue({ value, display, noShrink }: { value: string; display?: string; noShrink?: boolean }) {
  const fullValue = String(value ?? "").trim() || "--";
  const shownValue = display ?? (noShrink ? fullValue : shrink(fullValue));

  return (
    <Tooltip>
      <TooltipTrigger className="block max-w-full overflow-hidden text-ellipsis whitespace-nowrap text-left hover:text-[var(--bk-text-primary)] transition-colors">
        {shownValue}
      </TooltipTrigger>
      <TooltipContent className="max-w-sm rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] px-3 py-2 text-[11px] text-[var(--bk-text-primary)] shadow-xl">
        {fullValue}
      </TooltipContent>
    </Tooltip>
  );
}

function DockBadge({
  tone,
  children,
}: {
  tone: "ready" | "watch" | "blocked" | "neutral";
  children: React.ReactNode;
}) {
  if (tone === "ready") {
    return <Badge variant="success">{children}</Badge>;
  }

  if (tone === "blocked") {
    return (
      <Badge className="border-[var(--bk-status-danger)] bg-[color:color-mix(in_srgb,var(--bk-status-danger)_12%,transparent)] text-[var(--bk-status-danger)]">
        {children}
      </Badge>
    );
  }

  if (tone === "watch") {
    return (
      <Badge className="border-[var(--bk-status-warning)]/35 bg-[color:color-mix(in_srgb,var(--bk-status-warning)_12%,transparent)] text-[var(--bk-status-warning)]">
        {children}
      </Badge>
    );
  }

  return <Badge variant="neutral">{children}</Badge>;
}

function DockActionButton({
  label,
  variant = "ghost",
  disabled,
  onClick,
}: {
  label: string;
  variant?: "ghost" | "danger";
  disabled?: boolean;
  onClick: () => void;
}) {
  return (
    <Button
      type="button"
      size="sm"
      variant={variant === "danger" ? "bento-danger" : "bento-outline"}
      disabled={disabled}
      className="h-8 rounded-xl px-3 text-[11px] font-black"
      onClick={onClick}
    >
      {label}
    </Button>
  );
}

function DecisionMetricGrid({ items }: { items: Array<[string, unknown, number?]> }) {
  return (
    <div className="grid grid-cols-2 gap-2 md:grid-cols-4">
      {items.map(([label, value, digits]) => (
        <div
          key={label}
          className="min-w-0 rounded-xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)]/60 px-3 py-2"
        >
          <div className="truncate text-[9px] font-black uppercase tracking-wide text-[var(--bk-text-muted)] opacity-70">
            {label}
          </div>
          <div className="mt-1 truncate font-mono text-[11px] font-black text-[var(--bk-text-primary)]">
            {formatDecisionValue(value, digits)}
          </div>
        </div>
      ))}
    </div>
  );
}

function DecisionBarTable({ signalBarState }: { signalBarState: Record<string, unknown> | null }) {
  const rows: Array<[string, Record<string, unknown> | null]> = [
    ["T-3", nestedRecord(signalBarState, "prevBar3")],
    ["T-2", nestedRecord(signalBarState, "prevBar2")],
    ["T-1", nestedRecord(signalBarState, "prevBar1")],
    ["T", nestedRecord(signalBarState, "current")],
  ];

  return (
    <div className="overflow-hidden rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-strong)]">
      <Table tone="bento">
        <TableHeader className="bg-[var(--bk-surface-muted)]/40">
          <TableRow className="border-[var(--bk-border-soft)] hover:bg-transparent">
            {["Bar", "Start", "Open", "High", "Low", "Close", "Closed"].map((column) => (
              <TableHead
                key={column}
                className="h-8 px-3 text-[9px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]"
              >
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map(([label, bar]) => (
            <TableRow key={label} className="border-[var(--bk-border-soft)]">
              <TableCell className="px-3 py-2 font-mono text-[11px] font-black text-[var(--bk-text-primary)]">
                {label}
              </TableCell>
              <TableCell className="px-3 py-2 font-mono text-[10px] text-[var(--bk-text-secondary)]">
                {formatBarTimestamp(bar?.barStart)}
              </TableCell>
              <TableCell className="px-3 py-2 font-mono text-[11px]">{formatDecisionValue(bar?.open)}</TableCell>
              <TableCell className="px-3 py-2 font-mono text-[11px]">{formatDecisionValue(bar?.high)}</TableCell>
              <TableCell className="px-3 py-2 font-mono text-[11px]">{formatDecisionValue(bar?.low)}</TableCell>
              <TableCell className="px-3 py-2 font-mono text-[11px]">{formatDecisionValue(bar?.close)}</TableCell>
              <TableCell className="px-3 py-2 font-mono text-[11px]">{formatDecisionValue(bar?.isClosed)}</TableCell>
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

function DecisionTraceDialog({
  order,
  status,
  trace,
  error,
  onClose,
}: {
  order: Order | null;
  status: DecisionTraceStatus;
  trace: DecisionTrace | null;
  error: string;
  onClose: () => void;
}) {
  const decisionEventId = orderDecisionEventId(order);
  const metadata = trace?.decisionMetadata ?? {};
  const signalBarDecision = trace?.signalBarDecision ?? {};
  const signalBarState = trace?.signalBarState ?? null;
  const breakoutProof = trace?.breakoutProof ?? null;
  const proposal = nestedRecord(trace?.payload, "executionProposal");
  const role = firstText(proposal?.role, metadata.nextPlannedRole).toLowerCase();
  const proposalReason = firstText(proposal?.reason, metadata.nextPlannedReason, trace?.event.message);
  const signalKind = firstText(trace?.payload.signalKind, trace?.event.metadata?.signalKind);
  const isExit = role === "exit" || signalKind.toLowerCase().includes("exit");
  const prevBar1 = nestedRecord(signalBarState, "prevBar1");
  const prevBar2 = nestedRecord(signalBarState, "prevBar2");
  const breakoutPrice = signalBarDecision.breakoutPrice ?? metadata.breakoutPrice;
  const breakoutLevel = signalBarDecision.longBreakoutLevel;

  return (
    <Dialog open={!!order} onOpenChange={(open) => !open && onClose()}>
      <DialogContent tone="bento" className="max-h-[88vh] max-w-5xl overflow-hidden rounded-[28px] border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-0 shadow-2xl">
        <div className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)]/30 px-6 py-5">
          <DialogHeader>
            <div className="flex items-center gap-3">
              <div className="flex size-8 items-center justify-center rounded-xl bg-[color:color-mix(in_srgb,var(--bk-status-success)_12%,transparent)] text-[var(--bk-status-success)]">
                <FileSearch size={17} />
              </div>
              <div className="min-w-0">
                <DialogTitle className="truncate text-lg font-black text-[var(--bk-text-primary)]">订单决策快照</DialogTitle>
                <DialogDescription className="mt-1 truncate font-mono text-[11px] text-[var(--bk-text-muted)]">
                  {order?.id ?? "--"} · {decisionEventId || "no-decision-event"}
                </DialogDescription>
              </div>
              {role && (
                <DockBadge tone={isExit ? "watch" : "ready"}>{role.toUpperCase()}</DockBadge>
              )}
            </div>
          </DialogHeader>
        </div>

        <div className="max-h-[calc(88vh-142px)] space-y-4 overflow-y-auto p-6">
          {status === "loading" && (
            <div className="flex items-center justify-center gap-3 py-16 text-[var(--bk-text-muted)]">
              <Loader2 className="size-4 animate-spin" />
              <span className="text-[11px] font-black uppercase tracking-widest">Loading decision trace</span>
            </div>
          )}

          {status === "missing" && (
            <div className="rounded-2xl border border-dashed border-[var(--bk-border)] bg-[var(--bk-surface-faint)] p-6 text-center text-[12px] font-bold text-[var(--bk-text-muted)]">
              未找到该订单关联的 decision event
            </div>
          )}

          {status === "error" && (
            <div className="rounded-2xl border border-[var(--bk-status-danger)]/25 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)] p-4 text-[12px] font-bold text-[var(--bk-status-danger)]">
              {error || "加载决策快照失败"}
            </div>
          )}

          {status === "loaded" && trace && (
            <>
              <DecisionMetricGrid
                items={[
                  ["Action", trace.payload.action ?? trace.event.title],
                  ["Reason", trace.payload.reason ?? trace.event.message],
                  ["Role", role],
                  ["Proposal", proposalReason],
                  ["Signal", trace.payload.signalKind ?? trace.event.metadata?.signalKind],
                  ["State", trace.payload.decisionState ?? trace.event.metadata?.decisionState],
                  ["Event Time", trace.event.eventTime ? formatTime(trace.event.eventTime) : "--"],
                  ["Trigger", trace.payload.triggerType ?? trace.event.metadata?.triggerType],
                  ["Symbol", trace.payload.symbol ?? trace.event.metadata?.symbol],
                  ["Runtime", trace.payload.runtimeSessionId],
                ]}
              />

              {breakoutProof && (
                <div className="space-y-2">
                  <div className="flex items-center justify-between gap-3">
                    <h3 className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">
                      Window Proof
                    </h3>
                    <span className="truncate font-mono text-[10px] text-[var(--bk-text-muted)]">
                      {breakoutProof.source || "--"}
                    </span>
                  </div>
                  <DecisionMetricGrid
                    items={[
                      ["Proof At", breakoutProof.eventAt ? formatTime(breakoutProof.eventAt) : "--"],
                      ["Side", breakoutProof.side],
                      ["Level", breakoutProof.level],
                      ["Price", breakoutProof.price],
                      ["Proof Δ", differenceValue(breakoutProof.price, breakoutProof.level)],
                      ["Price Src", breakoutProof.priceSource],
                      ["Bar", breakoutProof.barTime ? formatTime(breakoutProof.barTime) : "--"],
                      ["Close", breakoutProof.close],
                    ]}
                  />
                </div>
              )}

              <div className="space-y-2">
                <div className="flex items-center justify-between gap-3">
                  <h3 className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">
                    {isExit ? "Exit Signal Snapshot" : "Breakout / Reentry"}
                  </h3>
                  <span className="truncate font-mono text-[10px] text-[var(--bk-text-muted)]">
                    {trace.signalBarStateKey || "--"}
                  </span>
                </div>
                {isExit && (
                  <div className="rounded-2xl border border-[var(--bk-status-warning)]/25 bg-[color:color-mix(in_srgb,var(--bk-status-warning)_8%,transparent)] px-4 py-3 text-[11px] font-bold text-[var(--bk-status-warning)]">
                    EXIT / {proposalReason || signalKind || "--"} · breakout fields are diagnostic
                  </div>
                )}
                <DecisionMetricGrid
                  items={[
                    ["Long Shape", signalBarDecision.longBreakoutShapeName],
                    ["Long Level", signalBarDecision.longBreakoutLevel],
                    ["T2 High Δ", differenceValue(prevBar2?.high, prevBar1?.high)],
                    ["Breakout Δ", differenceValue(breakoutPrice, breakoutLevel)],
                    ["Pattern", signalBarDecision.longBreakoutPatternReady],
                    ["Price Ready", signalBarDecision.longBreakoutPriceReady],
                    ["Quality", signalBarDecision.longBreakoutQualityReady],
                    ["Structure", signalBarDecision.longStructureReady],
                    ["Long Ready", signalBarDecision.longReady],
                    ["Breakout Px", signalBarDecision.breakoutPrice ?? metadata.breakoutPrice],
                    ["Breakout Src", signalBarDecision.breakoutPriceSource ?? metadata.breakoutPriceSource],
                    ["Reentry Window", metadata.reentryWindowOpen],
                    ["Reentry Trigger", metadata.reentryTriggerReady],
                    ["Reentry Price", metadata.reentryTriggerPrice],
                  ]}
                />
              </div>

              <div className="space-y-2">
                <h3 className="text-[11px] font-black uppercase tracking-widest text-[var(--bk-text-muted)]">
                  Runtime Signal Bars
                </h3>
                <DecisionMetricGrid
                  items={[
                    ["Timeframe", signalBarState?.timeframe],
                    ["SMA5", signalBarState?.sma5],
                    ["MA20", signalBarState?.ma20],
                    ["ATR14", signalBarState?.atr14],
                  ]}
                />
                <DecisionBarTable signalBarState={signalBarState} />
              </div>
            </>
          )}
        </div>

        <DialogFooter className="flex items-center justify-end gap-3 border-t border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)]/30 px-6 py-4">
          <DialogClose
            render={
              <Button variant="bento-outline" className="h-9 rounded-xl px-4 text-[12px] font-black" />
            }
          >
            关闭
          </DialogClose>
        </DialogFooter>
      </DialogContent>
    </Dialog>
  );
}

function Pagination({ 
  currentPage, 
  totalItems, 
  pageSize, 
  onPageChange, 
  onPageSizeChange 
}: { 
  currentPage: number; 
  totalItems: number; 
  pageSize: number; 
  onPageChange: (page: number) => void;
  onPageSizeChange: (size: number) => void;
}) {
  const totalPages = Math.ceil(totalItems / pageSize) || 1;
  const [jumpPage, setJumpPage] = useState("");

  const handleJump = () => {
    const p = parseInt(jumpPage);
    if (!isNaN(p) && p >= 1 && p <= totalPages) {
      onPageChange(p);
      setJumpPage("");
    }
  };

  return (
    <div className="flex items-center justify-between px-6 py-3 border-t border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)]/50">
      <div className="flex items-center gap-4">
        <div className="flex items-center gap-2">
          <span className="text-[10px] font-black uppercase text-[var(--bk-text-muted)] opacity-60">每页</span>
          <Select value={String(pageSize)} onValueChange={(val) => val && onPageSizeChange(parseInt(val))}>
            <SelectTrigger tone="bento" size="sm" className="h-7 w-16 text-[10px] font-black">
              <SelectValue />
            </SelectTrigger>
            <SelectContent tone="bento" className="min-w-[80px]">
              {[5, 10, 20, 50, 100].map(size => (
                <SelectItem key={size} value={String(size)}>{size}</SelectItem>
              ))}
            </SelectContent>
          </Select>
        </div>
        <span className="text-[10px] font-bold text-[var(--bk-text-muted)] opacity-60">
          共 {totalItems} 条 · {totalPages} 页
        </span>
      </div>

      <div className="flex items-center gap-3">
        <div className="flex items-center gap-1.5 mr-2">
          <Input 
            className="h-7 w-12 rounded-lg border-[var(--bk-border)] bg-[var(--bk-surface)] px-1 text-center text-[10px] font-black"
            placeholder="页码"
            value={jumpPage}
            onChange={(e) => setJumpPage(e.target.value)}
            onKeyDown={(e) => e.key === 'Enter' && handleJump()}
          />
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 rounded-lg px-2 text-[10px] font-black"
            onClick={handleJump}
          >
            跳转
          </Button>
        </div>

        <div className="flex items-center gap-1">
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 w-7 rounded-lg p-0" 
            disabled={currentPage <= 1}
            onClick={() => onPageChange(currentPage - 1)}
          >
            <ChevronLeft className="size-3.5" />
          </Button>
          <div className="flex h-7 min-w-[28px] items-center justify-center rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-2 text-[10px] font-black text-[var(--bk-text-primary)] shadow-inner">
            {currentPage}
          </div>
          <Button 
            variant="bento-outline" 
            size="sm" 
            className="h-7 w-7 rounded-lg p-0" 
            disabled={currentPage >= totalPages}
            onClick={() => onPageChange(currentPage + 1)}
          >
            <ChevronRight className="size-3.5" />
          </Button>
        </div>
      </div>
    </div>
  );
}

function DockTable({
  columns,
  rows,
  emptyMessage,
}: {
  columns: string[];
  rows: React.ReactNode[][];
  emptyMessage: string;
}) {
  if (rows.length === 0) {
    return (
      <div className="rounded-[24px] border border-dashed border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-4 py-8 text-center text-sm italic text-[var(--bk-text-secondary)]">
        {emptyMessage}
      </div>
    );
  }

  return (
    <div className="overflow-hidden rounded-[24px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] shadow-inner">
      <Table tone="bento">
        <TableHeader className="bg-[var(--bk-surface-muted)]/40">
          <TableRow className="border-[var(--bk-border-soft)] hover:bg-transparent">
            {columns.map((column) => (
              <TableHead
                key={column}
                className={cn(
                  "h-9 px-4 text-[10px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]",
                  column === "ID" && "min-w-[150px]",
                  column === "策略版本" && "min-w-[280px]",
                  column === "创建时间" && "min-w-[160px]",
                  column === "操作" && "text-right"
                )}
              >
                {column}
              </TableHead>
            ))}
          </TableRow>
        </TableHeader>
        <TableBody>
          {rows.map((row, index) => (
            <TableRow key={`row-${index}`} className="border-[var(--bk-border-soft)]">
              {row.map((cell, cellIndex) => {
                const columnName = columns[cellIndex];
                return (
                  <TableCell
                    key={`cell-${index}-${cellIndex}`}
                    className={cn(
                      "px-4 py-3 text-[12px] text-[var(--bk-text-primary)]",
                      columnName === "ID" && "min-w-[150px] font-mono",
                      columnName === "策略版本" && "min-w-[280px] font-mono",
                      columnName === "创建时间" && "min-w-[160px] font-mono",
                      columnName === "操作" && "text-right"
                    )}
                  >
                    {cell}
                  </TableCell>
                );
              })}
            </TableRow>
          ))}
        </TableBody>
      </Table>
    </div>
  );
}

export function DockContent({ dockTab, actions, sessionId }: DockContentProps) {
  const orders = useTradingStore(s => s.orders);
  const fills = useTradingStore(s => s.fills);
  const positions = useTradingStore(s => s.positions);
  const alerts = useTradingStore(s => s.alerts);
  const positionCloseAction = useUIStore(s => s.positionCloseAction);
  const liveSyncAction = useUIStore(s => s.liveSyncAction);
  const openConfirmDialog = useUIStore(s => s.openConfirmDialog);
  const { pairs, loading: pairsLoading, error: pairsError, refetch: refetchPairs } = useLiveTradePairs(
    dockTab === 'pairs' ? (sessionId ?? null) : null, 
    200
  );

  const [selectedPairForReview, setSelectedPairForReview] = useState<any | null>(null);
  const [selectedDecisionOrder, setSelectedDecisionOrder] = useState<Order | null>(null);
  const [decisionTraceStatus, setDecisionTraceStatus] = useState<DecisionTraceStatus>("idle");
  const [decisionTrace, setDecisionTrace] = useState<DecisionTrace | null>(null);
  const [decisionTraceError, setDecisionTraceError] = useState("");
  
  const [fillSyncOrder, setFillSyncOrder] = useState<Order | null>(null);
  const [fillSyncMode, setFillSyncMode] = useState<'view' | 'sync'>('view');

  // Pagination & Sorting State
  const [pages, setPages] = useState({ pairs: 1, positions: 1, alerts: 1 });
  const [pageSize, setPageSize] = useState(5);

  const ordersPageQuery = useOrdersPageQuery(pageSize, dockTab === 'orders');
  const fillsPageQuery = useFillsPageQuery(pageSize, dockTab === 'fills');

  // Reset page when tab or pageSize changes
  const handlePageChange = (page: number) => {
    if (dockTab === 'orders') {
      ordersPageQuery.setCurrentPage(page);
    } else if (dockTab === 'fills') {
      fillsPageQuery.setCurrentPage(page);
    } else {
      setPages(prev => ({ ...prev, [dockTab]: page }));
    }
  };

  const handlePageSizeChange = (size: number) => {
    setPageSize(size);
    setPages({ pairs: 1, positions: 1, alerts: 1 });
    if (dockTab === 'orders') {
      ordersPageQuery.setCurrentPage(1);
    } else if (dockTab === 'fills') {
      fillsPageQuery.setCurrentPage(1);
    }
  };

  const sortedPositions = useMemo(() => [...positions].sort((a, b) => Date.parse(b.updatedAt) - Date.parse(a.updatedAt)), [positions]);
  const sortedAlerts = useMemo(() => [...alerts].sort((a, b) => Date.parse(b.eventTime ?? "") - Date.parse(a.eventTime ?? "")), [alerts]);
  const sortedPairs = useMemo(() => [...pairs].sort((a, b) => Date.parse(b.entryAt) - Date.parse(a.entryAt)), [pairs]);

  useEffect(() => {
    if (!selectedDecisionOrder) {
      setDecisionTraceStatus("idle");
      setDecisionTrace(null);
      setDecisionTraceError("");
      return;
    }

    const decisionEventId = orderDecisionEventId(selectedDecisionOrder);
    if (!decisionEventId) {
      setDecisionTraceStatus("missing");
      setDecisionTrace(null);
      setDecisionTraceError("");
      return;
    }

    let active = true;
    const params = new URLSearchParams({
      type: "strategy-decision",
      decisionEventId,
      limit: "1",
    });
    setDecisionTraceStatus("loading");
    setDecisionTrace(null);
    setDecisionTraceError("");

    fetchJSON<UnifiedLogEventPage>(`/api/v1/logs/events?${params.toString()}`)
      .then((page) => {
        if (!active) {
          return;
        }
        const event = page.items?.[0];
        if (!event) {
          setDecisionTraceStatus("missing");
          return;
        }
        const baseTrace = buildDecisionTrace(event);
        const liveSessionId = firstText(event.liveSessionId, baseTrace.payload.liveSessionId);
        if (!liveSessionId) {
          setDecisionTrace(baseTrace);
          setDecisionTraceStatus("loaded");
          return;
        }
        fetchJSON<LiveSessionBreakoutDetail>(
          `/api/v1/live/sessions/${encodeURIComponent(liveSessionId)}/detail?fields=breakoutHistory`
        )
          .then((detail) => {
            if (!active) {
              return;
            }
            const breakoutProof = findBreakoutProof(event, baseTrace, detail.state?.breakoutHistory);
            setDecisionTrace(buildDecisionTrace(event, breakoutProof));
            setDecisionTraceStatus("loaded");
          })
          .catch((error) => {
            if (!active) {
              return;
            }
            console.warn("Failed to load breakout proof", error);
            setDecisionTrace(baseTrace);
            setDecisionTraceStatus("loaded");
          });
      })
      .catch((error) => {
        if (!active) {
          return;
        }
        console.warn("Failed to load order decision trace", error);
        setDecisionTraceError(error instanceof Error ? error.message : "加载决策快照失败");
        setDecisionTraceStatus("error");
      });

    return () => {
      active = false;
    };
  }, [selectedDecisionOrder]);

  const pagedOrders = ordersPageQuery.orders;
  const pagedFills = fillsPageQuery.fills;
  const pagedPositions = useMemo(() => sortedPositions.slice((pages.positions - 1) * pageSize, pages.positions * pageSize), [sortedPositions, pages.positions, pageSize]);
  const pagedAlerts = useMemo(() => sortedAlerts.slice((pages.alerts - 1) * pageSize, pages.alerts * pageSize), [sortedAlerts, pages.alerts, pageSize]);
  const pagedPairs = useMemo(() => sortedPairs.slice((pages.pairs - 1) * pageSize, pages.pairs * pageSize), [sortedPairs, pages.pairs, pageSize]);

  const orderById = new Map(orders.map((order) => [order.id, order] as const));
  const duplicateFallbackFillCounts = fills
    .filter((fill) => !(fill.exchangeTradeId ?? "").trim())
    .reduce((counts, fill) => {
      const key = [
        fill.orderId,
        String(fill.price),
        String(fill.quantity),
        String(fill.fee),
        fill.exchangeTradeTime ?? "",
      ].join("|");
      counts.set(key, (counts.get(key) ?? 0) + 1);
      return counts;
    }, new Map<string, number>());

  return (
    <div className="h-full relative flex flex-col overflow-hidden">
      <div className="flex-1 overflow-y-auto">
        {dockTab === 'pairs' && (
          <div className="p-0">
            {pairsLoading ? (
              <div className="flex items-center justify-center gap-3 py-20 text-[var(--bk-text-muted)]">
                <Activity size={16} className="animate-spin" />
                <span className="text-[11px] font-black uppercase tracking-widest">聚合追溯中...</span>
              </div>
            ) : pairsError ? (
              <div className="p-8 text-center text-rose-500 text-[11px] font-black">{pairsError}</div>
            ) : (
              <DockTable
                columns={["状态", "方向/Symbol", "开仓细节", "平仓细节", "数量/成交", "PNL统计"]}
                rows={pagedPairs.map((pair) => {
                  const netPositive = Number(pair.netPnl ?? 0) >= 0;
                  const quantity = String(pair.status).toLowerCase() === 'open' ? pair.openQuantity : pair.entryQuantity;
                  return [
                    <div key={`${pair.id}-status`} className="space-y-1">
                      <Badge variant={String(pair.status).toLowerCase() === 'open' ? 'neutral' : 'success'}>
                        {tradePairStatusLabel(pair.status)}
                      </Badge>
                      <div 
                        className={cn(
                          'text-[10px] font-black', 
                          tradePairVerdictTone(pair.exitVerdict),
                          (String(pair.exitVerdict).toLowerCase() === 'mismatch' || String(pair.exitVerdict).toLowerCase() === 'orphan-exit') && "cursor-pointer hover:underline flex items-center gap-1"
                        )}
                        onClick={() => {
                          const v = String(pair.exitVerdict).toLowerCase();
                          if (v === 'mismatch' || v === 'orphan-exit') {
                            setSelectedPairForReview(pair);
                          }
                        }}
                      >
                        {tradePairVerdictLabel(pair.exitVerdict)}
                        {(String(pair.exitVerdict).toLowerCase() === 'mismatch' || String(pair.exitVerdict).toLowerCase() === 'orphan-exit') && <AlertCircle size={10} />}
                      </div>
                    </div>,
                    <div key={`${pair.id}-side`} className="space-y-1">
                      <div className="text-[12px] font-black text-[var(--bk-text-primary)]">{pair.side}</div>
                      <div className="text-[10px] text-[var(--bk-text-muted)] font-mono">{pair.symbol}</div>
                    </div>,
                    <div key={`${pair.id}-entry`} className="space-y-0.5">
                      <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                        {formatMaybeNumber(pair.entryAvgPrice)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] opacity-60">{formatTime(pair.entryAt)}</div>
                      <div className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] truncate max-w-[120px]">
                        {pair.entryReason || '--'}
                      </div>
                    </div>,
                    String(pair.status).toLowerCase() === 'closed' ? (
                      <div key={`${pair.id}-exit`} className="space-y-0.5">
                        <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                          {formatMaybeNumber(pair.exitAvgPrice)}
                        </div>
                        <div className="text-[9px] text-[var(--bk-text-muted)] opacity-60">
                          {pair.exitAt ? formatTime(pair.exitAt) : '--'}
                        </div>
                        <div className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] truncate max-w-[120px]">
                          {pair.exitReason || '--'}
                        </div>
                      </div>
                    ) : (
                      <div key={`${pair.id}-exit-pending`} className="space-y-1">
                        <div className="text-[11px] font-black text-[var(--bk-status-success)]">运行中</div>
                        <div className="text-[9px] text-[var(--bk-text-muted)] font-mono">
                          未实现 {formatSigned(pair.unrealizedPnl)}
                        </div>
                      </div>
                    ),
                    <div key={`${pair.id}-qty`} className="space-y-0.5">
                      <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                        {formatMaybeNumber(quantity)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] font-black">
                        {pair.entryFillCount} IN / {pair.exitFillCount} OUT
                      </div>
                    </div>,
                    <div key={`${pair.id}-pnl`} className="space-y-0.5">
                      <div className={cn('font-mono text-[13px] font-black', netPositive ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]')}>
                        {formatSigned(pair.netPnl)}
                      </div>
                      <div className="text-[9px] text-[var(--bk-text-muted)] flex items-center gap-2">
                        <span>费 {formatMaybeNumber(pair.fees, 5)}</span>
                      </div>
                    </div>,
                  ];
                })}
                emptyMessage="当前焦点会话无追溯记录"
              />
            )}
          </div>
        )}
        {dockTab === 'orders' && (
          <DockTable
            columns={["ID", "策略版本", "Symbol", "Side", "Type", "信号", "数量", "价格", "交易所订单ID", "状态", "创建时间", "操作"]}
            rows={pagedOrders.map((order) => {
              const exchangeId = String(order.metadata?.exchangeOrderId ?? "--");
              const isReconciled = !!(order.metadata?.orderLifecycle as any)?.synced;
              const isOrphan = order.status === "ACCEPTED" && (order.metadata?.orderLifecycle as any)?.reconciliationState === "orphaned";
              const decisionEventId = orderDecisionEventId(order);
              const signalSummary = orderSignalSummary(order);

              return [
                <TruncatedValue key={`${order.id}-id`} value={order.id} display={order.id.replace('order-', '')} />,
                <TruncatedValue key={`${order.id}-strategy`} value={String(order.metadata?.strategyVersionId ?? order.metadata?.source ?? "--")} noShrink />,
                order.symbol,
                <DockBadge key={`${order.id}-side`} tone={order.side === "buy" ? "ready" : "neutral"}>{order.side}</DockBadge>,
                order.type,
                <div key={`${order.id}-signal`} className="max-w-[150px] space-y-0.5">
                  <div className="truncate text-[11px] font-black text-[var(--bk-text-primary)]">
                    {signalSummary.reason || "--"}
                  </div>
                  <div className="truncate font-mono text-[9px] font-bold uppercase text-[var(--bk-text-muted)]">
                    {signalSummary.role || signalSummary.signalKind || "--"}
                  </div>
                </div>,
                formatMaybeNumber(order.quantity),
                formatMaybeNumber(order.price),
                <div key={`${order.id}-exid`} className="flex items-center gap-1.5 min-w-[120px]">
                  <TruncatedValue value={exchangeId} />
                  {isReconciled && (
                     <div className="flex size-3.5 items-center justify-center rounded-full bg-[var(--bk-status-success-soft)] text-[var(--bk-status-success)]">
                        <ShieldCheck className="size-2.5" />
                     </div>
                  )}
                </div>,
                <div key={`${order.id}-status`} className="flex items-center gap-2">
                  <DockBadge tone={isOrphan ? "blocked" : (isReconciled ? "ready" : "watch")}>
                    {technicalStatusLabel(order.status)}
                  </DockBadge>
                </div>,
                formatTime(order.createdAt),
                <div key={`${order.id}-actions`} className="inline-flex items-center justify-end gap-1.5 relative">
                  <Tooltip>
                    <TooltipTrigger
                      className={cn(
                        "inline-flex h-8 w-8 items-center justify-center rounded-xl border border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] text-[var(--bk-text-muted)] transition-colors hover:bg-[var(--bk-surface-strong)] hover:text-[var(--bk-text-primary)]",
                        !decisionEventId && "opacity-45"
                      )}
                      aria-label="查看决策快照"
                      aria-disabled={!decisionEventId}
                      onClick={() => {
                        if (decisionEventId) {
                          setSelectedDecisionOrder(order);
                        }
                      }}
                    >
                      <FileSearch className="size-3.5" />
                    </TooltipTrigger>
                    <TooltipContent className="rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] px-3 py-2 text-[11px] text-[var(--bk-text-primary)] shadow-xl">
                      {decisionEventId ? "决策快照" : "无决策事件"}
                    </TooltipContent>
                  </Tooltip>
                  <DockActionButton 
                    label={liveSyncAction === order.id ? "Syncing..." : "Sync"} 
                    disabled={liveSyncAction !== null}
                    variant="ghost" 
                    onClick={() => actions.syncLiveOrder(order.id)} 
                  />
                  {liveSyncAction === order.id && (
                    <Loader2 className="absolute -right-2 top-1/2 size-3.5 -translate-y-1/2 animate-spin text-[var(--bk-text-muted)]" />
                  )}
                </div>,
              ];
            })}
            emptyMessage="暂无订单"
          />
        )}
        {dockTab === 'positions' && (
          <DockTable
            columns={["ID", "账户", "Symbol", "Side", "仓位大小", "开仓价", "标记价", "更新时间", "操作"]}
            rows={pagedPositions.map((pos) => [
              <TruncatedValue key={`${pos.id}-id`} value={pos.id} display={pos.id.replace('position-', 'pos-')} />,
              <TruncatedValue key={`${pos.id}-account`} value={pos.accountId} display={pos.accountId.replace('account-', 'acc-')} />,
              pos.symbol,
              <DockBadge key={`${pos.id}-side`} tone={pos.side === "long" ? "ready" : "neutral"}>{pos.side}</DockBadge>,
              formatMaybeNumber(pos.quantity),
              formatMaybeNumber(pos.entryPrice),
              formatMaybeNumber(pos.markPrice),
              formatTime(pos.updatedAt),
              <div key={`${pos.id}-actions`} className="inline-actions">
                <DockActionButton 
                  label={positionCloseAction === pos.id ? "平仓中..." : "强平"} 
                  variant="danger" 
                  disabled={positionCloseAction !== null}
                  onClick={() => {
                    openConfirmDialog(
                      "强制市价平仓风险确认",
                      "您即将放弃策略托管，使用系统市价单直接平仓。注意：此接管动作可能产生额外滑点，是否确认强平？",
                      () => actions.closePosition(pos.id)
                    );
                  }} 
                />
              </div>,
            ])}
            emptyMessage="暂无持仓"
          />
        )}
        {dockTab === 'fills' && (
          <DockTable
            columns={["ID", "来源", "策略版本", "Symbol", "侧向", "价格", "数量", "手续费", "交易所成交ID", "交易所成交时间", "状态/操作"]}
            rows={pagedFills.map((fill) => {
              const order = orderById.get(fill.orderId);
              const duplicateKey = [
                fill.orderId,
                String(fill.price),
                String(fill.quantity),
                String(fill.fee),
                fill.exchangeTradeTime ?? "",
              ].join("|");
              const suspiciousDuplicate = !(fill.exchangeTradeId ?? "").trim() && (duplicateFallbackFillCounts.get(duplicateKey) ?? 0) > 1;
              const sourceLabel = fill.source === "synthetic" ? "Synthetic" : fill.source === "real" ? "Real" : fill.source === "remainder" ? "Remainder" : (fill.source || "--");
              const isSynthetic = fill.source === "synthetic" || !fill.exchangeTradeId || fill.fee === 0;

              return [
                <TruncatedValue key={`${fill.id}-id`} value={fill.id} display={fill.id.replace('fill-', '')} />,
                <DockBadge key={`${fill.id}-source`} tone={fill.source === "real" ? "ready" : fill.source === "synthetic" ? "watch" : "neutral"}>{sourceLabel}</DockBadge>,
                String(order?.metadata?.strategyVersionId ?? fill.strategyVersion ?? "--"),
                order?.symbol ?? fill.symbol ?? "--",
                order?.side ?? fill.side ?? "--",
                formatMaybeNumber(fill.price),
                formatMaybeNumber(fill.quantity),
                formatMaybeNumber(fill.fee),
                <TruncatedValue key={`${fill.id}-exid`} value={fill.exchangeTradeId ?? "--"} />,
                formatTime(fill.exchangeTradeTime ?? ""),
                <div key={`${fill.id}-actions`} className="flex items-center justify-end gap-2">
                  {suspiciousDuplicate ? (
                    <DockBadge tone="watch">疑似重复</DockBadge>
                  ) : fill.exchangeTradeId ? (
                    <DockBadge tone="ready">已同步</DockBadge>
                  ) : (
                    <span className="text-[11px] text-[var(--bk-text-muted)]">等待同步</span>
                  )}
                  {order && (
                    <div className="flex gap-1 ml-2">
                      <Button
                        variant="bento-outline"
                        size="sm"
                        className="h-6 px-2 text-[10px]"
                        onClick={() => {
                          setFillSyncOrder(order);
                          setFillSyncMode('view');
                        }}
                      >
                        详情
                      </Button>
                      {isSynthetic && (
                        <Button
                          variant="bento-outline"
                          size="sm"
                          className="h-6 px-2 text-[10px] text-orange-500 border-orange-500/30 hover:bg-orange-500/10"
                          onClick={() => {
                            setFillSyncOrder(order);
                            setFillSyncMode('sync');
                          }}
                        >
                          同步
                        </Button>
                      )}
                    </div>
                  )}
                </div>,
              ];
            })}
            emptyMessage="暂无成交记录"
          />
        )}
        {dockTab === 'alerts' && (
          <DockTable
            columns={["时间", "级别", "模块", "消息"]}
            rows={pagedAlerts.map((alert) => [
              formatTime(alert.eventTime ?? ""),
              <DockBadge key={`${alert.id}-level`} tone={alert.level === "critical" ? "blocked" : alert.level === "warning" ? "watch" : "neutral"}>
                {alert.level}
              </DockBadge>,
              alert.title,
              alert.detail,
            ])}
            emptyMessage="暂无告警信息"
          />
        )}
      </div>

      <Pagination 
        currentPage={dockTab === 'orders' ? ordersPageQuery.currentPage : dockTab === 'fills' ? fillsPageQuery.currentPage : pages[dockTab]}
        totalItems={dockTab === 'orders' ? ordersPageQuery.totalCount : dockTab === 'fills' ? fillsPageQuery.totalCount : { pairs: pairs.length, positions: positions.length, alerts: alerts.length }[dockTab]}
        pageSize={pageSize}
        onPageChange={handlePageChange}
        onPageSizeChange={handlePageSizeChange}
      />

      <DecisionTraceDialog
        order={selectedDecisionOrder}
        status={decisionTraceStatus}
        trace={decisionTrace}
        error={decisionTraceError}
        onClose={() => setSelectedDecisionOrder(null)}
      />

      <ManualTradeReviewDialog 
        pair={selectedPairForReview}
        sessionId={sessionId}
        onClose={() => setSelectedPairForReview(null)}
        onSuccess={() => refetchPairs?.()}
      />

      <FillSyncModal
        isOpen={fillSyncOrder !== null}
        onClose={() => setFillSyncOrder(null)}
        order={fillSyncOrder}
        initialMode={fillSyncMode}
        onSuccess={() => {
          fillsPageQuery.refetch?.();
          ordersPageQuery.refetch?.();
        }}
      />
    </div>
  );
}
