import React, { useCallback, useEffect, useMemo, useState } from 'react';
import {
  AlertTriangle,
  CheckCircle2,
  Clock3,
  RefreshCw,
  RotateCw,
  ServerCog,
  ShieldAlert,
  Signal,
  SlidersHorizontal,
  XCircle,
} from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from '../components/ui/dialog';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../components/ui/table';
import { Separator } from '../components/ui/separator';
import { Textarea } from '../components/ui/textarea';
import { cn } from '../lib/utils';
import { fetchJSON } from '../utils/api';
import { formatTime, shrink } from '../utils/format';
import {
  RuntimeSupervisorControlAction,
  RuntimeSupervisorRuntimeStatus,
  RuntimeSupervisorSnapshot,
  RuntimeSupervisorTargetSnapshot,
} from '../types/domain';

type LoadState = 'idle' | 'loading' | 'loaded' | 'error';
type BadgeVariant = NonNullable<React.ComponentProps<typeof Badge>['variant']>;
type RuntimeRow = RuntimeSupervisorRuntimeStatus & { targetName: string };
type RuntimeControlActionKind = 'restart' | 'suppress-auto-restart' | 'resume-auto-restart';
type RuntimeControlDialogState = {
  runtime: RuntimeRow;
  action: RuntimeControlActionKind;
};
type RuntimeAutoRestartAudit = {
  label: string;
  variant: BadgeVariant;
  at?: string;
  reason?: string;
  source?: string;
};

const REFRESH_INTERVAL_MS = 15_000;

function formatOptionalTime(value?: string) {
  return value ? formatTime(value) : '--';
}

function isProbeOK(probe?: RuntimeSupervisorTargetSnapshot['healthz']) {
  if (!probe?.reachable || probe.error) {
    return false;
  }
  return probe.statusCode == null || (probe.statusCode >= 200 && probe.statusCode < 300);
}

function probeText(probe?: RuntimeSupervisorTargetSnapshot['healthz']) {
  if (!probe) {
    return 'missing';
  }
  if (!probe.reachable) {
    return 'unreachable';
  }
  if (probe.statusCode != null) {
    return `HTTP ${probe.statusCode}`;
  }
  return probe.error ? 'error' : 'reachable';
}

function executorKindLabel(value?: string) {
  const normalized = String(value || '').trim();
  return normalized || 'none';
}

function statusVariant(value?: string): BadgeVariant {
  const normalized = String(value || '').trim().toLowerCase();
  if (['ok', 'ready', 'running', 'healthy'].includes(normalized)) {
    return 'success';
  }
  if (['error', 'fatal', 'blocked', 'unreachable', 'suppressed'].includes(normalized)) {
    return 'destructive';
  }
  if (['recovering', 'starting', 'stale', 'warning', 'degraded'].includes(normalized)) {
    return 'secondary';
  }
  return 'neutral';
}

function ProbeBadge({ probe }: { probe: RuntimeSupervisorTargetSnapshot['healthz'] }) {
  const ok = isProbeOK(probe);
  return (
    <div className="flex max-w-[220px] flex-col gap-1">
      <Badge variant={ok ? 'success' : 'destructive'}>{probeText(probe)}</Badge>
      {probe.error && (
        <span className="truncate text-xs text-[var(--bk-text-muted)]" title={probe.error}>
          {probe.error}
        </span>
      )}
    </div>
  );
}

function StatusBadge({ value, fallback = '--' }: { value?: string; fallback?: string }) {
  const label = String(value || '').trim() || fallback;
  return <Badge variant={statusVariant(label)}>{label}</Badge>;
}

function MetricCard({
  icon: Icon,
  label,
  value,
  detail,
  tone = 'neutral',
}: {
  icon: React.ComponentType<{ className?: string }>;
  label: string;
  value: string | number;
  detail: string;
  tone?: 'neutral' | 'success' | 'warning' | 'danger';
}) {
  return (
    <Card tone="bento" className="rounded-lg">
      <CardContent className="flex items-center justify-between gap-3">
        <div className="flex min-w-0 flex-col gap-1">
          <span className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">{label}</span>
          <span className="text-2xl font-semibold tabular-nums text-[var(--bk-text-primary)]">{value}</span>
          <span className="truncate text-xs text-[var(--bk-text-muted)]">{detail}</span>
        </div>
        <div
          className={cn(
            'flex size-10 shrink-0 items-center justify-center rounded-lg border',
            tone === 'success' && 'border-[var(--bk-status-success)] bg-[var(--bk-status-success-soft)] text-[var(--bk-status-success)]',
            tone === 'warning' && 'border-[var(--bk-status-warning)] bg-[var(--bk-status-warning-soft)] text-[var(--bk-status-warning)]',
            tone === 'danger' && 'border-[var(--bk-status-danger)] bg-[var(--bk-status-danger-soft)] text-[var(--bk-status-danger)]',
            tone === 'neutral' && 'border-[var(--bk-border)] bg-[var(--bk-surface-muted)] text-[var(--bk-text-muted)]'
          )}
        >
          <Icon className="size-5" />
        </div>
      </CardContent>
    </Card>
  );
}

function runtimeNeedsAttention(runtime: RuntimeSupervisorRuntimeStatus) {
  if (runtime.autoRestartSuppressed) {
    return true;
  }
  const actual = String(runtime.actualStatus || '').toUpperCase();
  const health = String(runtime.health || '').toLowerCase();
  return actual === 'ERROR' || ['error', 'suppressed', 'unreachable', 'stale'].includes(health);
}

function runtimeAutoRestartAudit(runtime: RuntimeSupervisorRuntimeStatus): RuntimeAutoRestartAudit | null {
  if (runtime.autoRestartSuppressed) {
    return {
      label: 'suppressed',
      variant: 'destructive',
      at: runtime.autoRestartSuppressedAt,
      reason: runtime.autoRestartSuppressedReason,
      source: runtime.autoRestartSuppressedSource,
    };
  }
  if (runtime.autoRestartResumedAt || runtime.autoRestartResumedReason || runtime.autoRestartResumedSource) {
    return {
      label: 'resumed',
      variant: 'neutral',
      at: runtime.autoRestartResumedAt,
      reason: runtime.autoRestartResumedReason,
      source: runtime.autoRestartResumedSource,
    };
  }
  return null;
}

function runtimeAutoRestartAuditText(audit: RuntimeAutoRestartAudit) {
  const reason = String(audit.reason || '').trim() || '--';
  const meta = [
    audit.at ? `at ${formatTime(audit.at)}` : undefined,
    audit.source ? `source ${audit.source}` : undefined,
  ].filter(Boolean);
  return meta.length > 0 ? `${audit.label}: ${reason} (${meta.join(', ')})` : `${audit.label}: ${reason}`;
}

function runtimeAutoRestartAuditTitle(audit: RuntimeAutoRestartAudit) {
  return [
    `state=${audit.label}`,
    audit.at ? `at=${audit.at}` : undefined,
    audit.source ? `source=${audit.source}` : undefined,
    audit.reason ? `reason=${audit.reason}` : undefined,
  ].filter(Boolean).join(' ');
}

function PolicyBadge({ enabled, enabledLabel, disabledLabel }: { enabled: boolean; enabledLabel: string; disabledLabel: string }) {
  return <Badge variant={enabled ? 'secondary' : 'neutral'}>{enabled ? enabledLabel : disabledLabel}</Badge>;
}

function applicationRestartPlanTitle(plan?: RuntimeSupervisorRuntimeStatus['applicationRestartPlan']) {
  if (!plan) {
    return undefined;
  }
  return [
    `decision=${plan.decision || 'blocked'}`,
    `enabled=${plan.enabled}`,
    `healthzOk=${plan.healthzOk}`,
    `supported=${plan.supported}`,
    `due=${plan.due}`,
    `duplicate=${plan.duplicate}`,
    plan.blockedReason ? `blockedReason=${plan.blockedReason}` : undefined,
    plan.eligibleReason ? `eligibleReason=${plan.eligibleReason}` : undefined,
  ].filter(Boolean).join(' ');
}

function runtimeRestartSupported(runtime: RuntimeSupervisorRuntimeStatus) {
  const kind = String(runtime.runtimeKind || '').trim().toLowerCase();
  return kind === 'signal' || kind === 'signal-runtime';
}

function runtimeControlActionKey(runtime: RuntimeRow, action: RuntimeControlActionKind) {
  return `${runtime.targetName}:${runtime.runtimeKind}:${runtime.runtimeId}:${action}`;
}

function runtimeControlPath(action: RuntimeControlActionKind) {
  if (action === 'suppress-auto-restart') {
    return '/api/v1/runtime/suppress-auto-restart';
  }
  if (action === 'resume-auto-restart') {
    return '/api/v1/runtime/resume-auto-restart';
  }
  return '/api/v1/runtime/restart';
}

function runtimeControlLabel(action: RuntimeControlActionKind) {
  if (action === 'suppress-auto-restart') {
    return 'Suppress Auto Restart';
  }
  if (action === 'resume-auto-restart') {
    return 'Resume Auto Restart';
  }
  return 'Restart Signal Runtime';
}

function dashboardRuntimeControlReason(runtime: RuntimeRow, action: RuntimeControlActionKind) {
  return `dashboard manual ${action}: target=${runtime.targetName} runtime=${runtime.runtimeId}`;
}

export function SupervisorStage() {
  const [snapshot, setSnapshot] = useState<RuntimeSupervisorSnapshot | null>(null);
  const [loadState, setLoadState] = useState<LoadState>('idle');
  const [error, setError] = useState<string | null>(null);
  const [controlDialog, setControlDialog] = useState<RuntimeControlDialogState | null>(null);
  const [controlReason, setControlReason] = useState('');
  const [controlSubmittingKey, setControlSubmittingKey] = useState<string | null>(null);
  const [controlNotice, setControlNotice] = useState<string | null>(null);
  const [controlError, setControlError] = useState<string | null>(null);

  const loadSnapshot = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState('loading');
    }
    try {
      const payload = await fetchJSON<RuntimeSupervisorSnapshot>('/api/v1/supervisor/status');
      setSnapshot(payload);
      setError(null);
      setLoadState('loaded');
      return true;
    } catch (err) {
      setError(err instanceof Error ? err.message : '读取 supervisor 状态失败');
      setLoadState('error');
      return false;
    }
  }, []);

  const openControlDialog = useCallback((runtime: RuntimeRow, action: RuntimeControlActionKind) => {
    setControlDialog({ runtime, action });
    setControlReason(dashboardRuntimeControlReason(runtime, action));
    setControlNotice(null);
    setControlError(null);
  }, []);

  const closeControlDialog = useCallback(() => {
    if (controlSubmittingKey) {
      return;
    }
    setControlDialog(null);
    setControlReason('');
  }, [controlSubmittingKey]);

  const submitRuntimeControl = useCallback(async () => {
    if (!controlDialog) {
      return;
    }
    const { runtime, action } = controlDialog;
    const reason = controlReason.trim();
    if (!reason) {
      setControlError('reason is required');
      return;
    }
    const key = runtimeControlActionKey(runtime, action);
    setControlSubmittingKey(key);
    setControlError(null);
    setControlNotice(null);
    try {
      await fetchJSON(runtimeControlPath(action), {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({
          runtimeId: runtime.runtimeId,
          runtimeKind: runtime.runtimeKind,
          confirm: true,
          force: action === 'restart' ? false : undefined,
          reason,
        }),
      });
    } catch (err) {
      setControlError(err instanceof Error ? err.message : 'runtime control failed');
      setControlSubmittingKey(null);
      return;
    }

    setControlDialog(null);
    setControlReason('');
    const refreshed = await loadSnapshot(true);
    const acceptedMessage = `${runtimeControlLabel(action)} accepted: ${shrink(runtime.runtimeId)}`;
    if (refreshed) {
      setControlNotice(acceptedMessage);
    } else {
      setControlNotice(`${acceptedMessage}; refresh failed`);
    }
    setControlSubmittingKey(null);
  }, [controlDialog, controlReason, loadSnapshot]);

  useEffect(() => {
    void loadSnapshot();
    const timer = window.setInterval(() => {
      void loadSnapshot(true);
    }, REFRESH_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [loadSnapshot]);

  const {
    runtimeRows,
    controlActionRows,
    fallbackCount,
    executableFallbackCount,
    dryRunFallbackCount,
    attentionCount,
    fullyReachableTargets,
  } = useMemo(() => {
    const targets = snapshot?.targets ?? [];
    const runtimes = targets.flatMap((target) =>
      (target.status?.runtimes ?? []).map((runtime) => ({
        ...runtime,
        targetName: target.name,
      }))
    );
    return {
      runtimeRows: runtimes,
      controlActionRows: targets.flatMap((target) =>
        (target.controlActions ?? []).map((action) => ({
          ...action,
          targetName: target.name,
        }))
      ),
      fallbackCount: targets.filter((target) => target.serviceState.containerFallbackCandidate).length,
      executableFallbackCount: targets.filter((target) => target.containerFallbackPlan?.executable).length,
      dryRunFallbackCount: targets.filter((target) => target.containerFallbackPlan?.executable && target.containerFallbackPlan.executorDryRun).length,
      attentionCount: runtimes.filter(runtimeNeedsAttention).length,
      fullyReachableTargets: targets.filter((target) => isProbeOK(target.healthz) && isProbeOK(target.runtimeStatus)).length,
    };
  }, [snapshot]);

  const targets = snapshot?.targets ?? [];
  const policy = snapshot?.policy;
  const isLoading = loadState === 'loading' || loadState === 'idle';

  return (
    <div className="absolute inset-0 overflow-y-auto bg-[var(--bk-canvas)] p-6">
      <div className="mx-auto flex w-full max-w-[1600px] flex-col gap-6">
        <div className="flex flex-wrap items-center justify-between gap-4">
          <div className="flex items-center gap-3">
            <div className="flex size-11 items-center justify-center rounded-lg border border-[var(--bk-border-strong)] bg-[var(--bk-surface)] text-[var(--bk-accent)]">
              <ServerCog className="size-5" />
            </div>
            <div className="flex flex-col gap-1">
              <span className="text-xs font-semibold uppercase text-[var(--bk-text-muted)]">Runtime Supervisor</span>
              <h1 className="text-2xl font-semibold text-[var(--bk-text-primary)]">统一运行态视图</h1>
            </div>
          </div>
          <div className="flex items-center gap-2">
            <Badge variant="neutral" className="h-7 rounded-lg">
              <Clock3 className="size-3" />
              {snapshot ? formatOptionalTime(snapshot.checkedAt) : '--'}
            </Badge>
            <Button
              type="button"
              size="icon-sm"
              variant="bento-outline"
              className="rounded-lg"
              onClick={() => void loadSnapshot()}
              disabled={isLoading}
              title="刷新"
              aria-label="刷新"
            >
              <RefreshCw className={cn('size-4', isLoading && 'animate-spin')} />
            </Button>
          </div>
        </div>

        <Separator className="bg-[var(--bk-border-strong)] opacity-60" />

        {loadState === 'error' && !snapshot ? (
          <Card tone="bento" className="rounded-lg">
            <CardContent className="flex items-center gap-3 py-6">
              <AlertTriangle className="size-5 text-[var(--bk-status-warning)]" />
              <div className="flex flex-col gap-1">
                <span className="font-medium text-[var(--bk-text-primary)]">Supervisor 状态不可用</span>
                <span className="text-sm text-[var(--bk-text-muted)]">{error}</span>
              </div>
            </CardContent>
          </Card>
        ) : (
          <>
            <div className="grid gap-4 md:grid-cols-2 xl:grid-cols-4">
              <MetricCard
                icon={ServerCog}
                label="Targets"
                value={targets.length}
                detail={`${fullyReachableTargets}/${targets.length} fully reachable`}
                tone={fullyReachableTargets === targets.length && targets.length > 0 ? 'success' : 'warning'}
              />
              <MetricCard
                icon={Signal}
                label="Runtimes"
                value={runtimeRows.length}
                detail={`${attentionCount} need attention`}
                tone={attentionCount > 0 ? 'warning' : 'success'}
              />
              <MetricCard
                icon={ShieldAlert}
                label="Fallback"
                value={fallbackCount}
                detail={
                  dryRunFallbackCount > 0
                    ? `${executableFallbackCount} eligible, ${dryRunFallbackCount} dry-run`
                    : `${executableFallbackCount} eligible`
                }
                tone={fallbackCount > 0 ? 'danger' : 'neutral'}
              />
              <MetricCard
                icon={RotateCw}
                label="Controls"
                value={controlActionRows.length}
                detail="application restart intents"
                tone={controlActionRows.some((action) => action.error) ? 'danger' : 'neutral'}
              />
            </div>

            {policy && (
              <Card tone="bento" className="rounded-lg">
                <CardHeader>
                  <CardTitle>Supervisor Policy</CardTitle>
                  <CardAction>
                    <div className="flex flex-wrap justify-end gap-2">
                      <Badge variant={policy.containerExecutorConfigured ? 'success' : 'neutral'}>
                        {policy.containerExecutorConfigured ? 'executor ready' : 'no executor'}
                      </Badge>
                      <Badge variant="neutral">{executorKindLabel(policy.containerExecutorKind)}</Badge>
                      {policy.containerExecutorDryRun && <Badge variant="secondary">dry-run</Badge>}
                    </div>
                  </CardAction>
                </CardHeader>
                <CardContent>
                  <div className="grid gap-3 md:grid-cols-2 xl:grid-cols-4">
                    <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-muted)] px-3 py-2">
                      <div className="flex min-w-0 flex-col gap-1">
                        <span className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">Application Restart</span>
                        <PolicyBadge enabled={policy.applicationRestartEnabled} enabledLabel="enabled" disabledLabel="disabled" />
                      </div>
                      <RotateCw className="size-4 shrink-0 text-[var(--bk-text-muted)]" />
                    </div>
                    <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-muted)] px-3 py-2">
                      <div className="flex min-w-0 flex-col gap-1">
                        <span className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">Failure Threshold</span>
                        <Badge variant="neutral">{policy.serviceFailureThreshold}</Badge>
                      </div>
                      <SlidersHorizontal className="size-4 shrink-0 text-[var(--bk-text-muted)]" />
                    </div>
                    <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-muted)] px-3 py-2">
                      <div className="flex min-w-0 flex-col gap-1">
                        <span className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">Container Restart</span>
                        <PolicyBadge enabled={policy.containerRestartEnabled} enabledLabel="opt-in" disabledLabel="disabled" />
                      </div>
                      <ShieldAlert className="size-4 shrink-0 text-[var(--bk-text-muted)]" />
                    </div>
                    <div className="flex min-w-0 items-center justify-between gap-3 rounded-lg border border-[var(--bk-border)] bg-[var(--bk-surface-muted)] px-3 py-2">
                      <div className="flex min-w-0 flex-col gap-1">
                        <span className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">Container Executor</span>
                        <div className="flex flex-wrap gap-1">
                          <PolicyBadge enabled={policy.containerExecutorConfigured} enabledLabel="ready" disabledLabel="not configured" />
                          <Badge variant="neutral">{executorKindLabel(policy.containerExecutorKind)}</Badge>
                          {policy.containerExecutorDryRun && <Badge variant="secondary">dry-run</Badge>}
                        </div>
                      </div>
                      <ServerCog className="size-4 shrink-0 text-[var(--bk-text-muted)]" />
                    </div>
                  </div>
                </CardContent>
              </Card>
            )}

            <Card tone="bento" className="rounded-lg">
              <CardHeader>
                <CardTitle>Service Targets</CardTitle>
                <CardAction>
                  {error ? <Badge variant="destructive">{error}</Badge> : <Badge variant="neutral">auto refresh 15s</Badge>}
                </CardAction>
              </CardHeader>
              <CardContent>
                {targets.length === 0 ? (
                  <div className="flex h-28 items-center justify-center rounded-lg border border-dashed border-[var(--bk-border)] text-sm text-[var(--bk-text-muted)]">
                    暂无 supervisor target
                  </div>
                ) : (
                  <Table tone="bento">
                    <TableHeader>
                      <TableRow>
                        <TableHead>Target</TableHead>
                        <TableHead>Healthz</TableHead>
                        <TableHead>Runtime API</TableHead>
                        <TableHead>Failures</TableHead>
                        <TableHead>Fallback</TableHead>
                        <TableHead>Runtimes</TableHead>
                        <TableHead>Checked</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {targets.map((target) => {
                        const runtimeCount = target.status?.runtimes.length ?? 0;
                        const runtimeErrors = target.status?.runtimes.filter(runtimeNeedsAttention).length ?? 0;
                        const fallbackPlan = target.containerFallbackPlan;
                        const fallbackDecision = fallbackPlan?.decision || (fallbackPlan?.executable ? 'eligible' : 'blocked');
                        const fallbackAttemptCount = target.serviceState.containerFallbackAttemptCount ?? 0;
                        const fallbackDetail =
                          fallbackPlan?.blockedReason ||
                          fallbackPlan?.eligibleReason ||
                          target.serviceState.lastContainerFallbackDecisionReason ||
                          fallbackPlan?.reason ||
                          target.serviceState.containerFallbackReason;
                        return (
                          <TableRow key={`${target.name}:${target.baseUrl}`}>
                            <TableCell>
                              <div className="flex max-w-[260px] flex-col gap-1">
                                <span className="font-medium text-[var(--bk-text-primary)]">{target.name}</span>
                                <span className="truncate text-xs text-[var(--bk-text-muted)]" title={target.baseUrl}>
                                  {target.baseUrl}
                                </span>
                              </div>
                            </TableCell>
                            <TableCell>
                              <ProbeBadge probe={target.healthz} />
                            </TableCell>
                            <TableCell>
                              <ProbeBadge probe={target.runtimeStatus} />
                            </TableCell>
                            <TableCell>
                              <div className="flex flex-col gap-1">
                                <span className="font-mono text-sm tabular-nums">
                                  {target.serviceState.consecutiveFailures}/{target.serviceState.failureThreshold}
                                </span>
                                {target.serviceState.lastFailureReason && (
                                  <span className="max-w-[220px] truncate text-xs text-[var(--bk-text-muted)]" title={target.serviceState.lastFailureReason}>
                                    {target.serviceState.lastFailureReason}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="flex max-w-[220px] flex-col gap-1">
                                <div className="flex flex-wrap gap-1">
                                  <Badge variant={target.serviceState.containerFallbackCandidate ? 'destructive' : 'neutral'}>
                                    {target.serviceState.containerFallbackCandidate ? 'candidate' : 'clear'}
                                  </Badge>
                                  {fallbackPlan && (
                                    <Badge variant={fallbackDecision === 'eligible' ? 'success' : 'neutral'}>
                                      {fallbackDecision}
                                    </Badge>
                                  )}
                                  {fallbackPlan && (
                                    <Badge variant={fallbackPlan.enabled ? 'secondary' : 'neutral'}>
                                      {fallbackPlan.enabled ? 'opt-in' : 'disabled'}
                                    </Badge>
                                  )}
                                  {fallbackPlan && (
                                    <Badge variant={fallbackPlan.executorConfigured ? 'success' : 'neutral'}>
                                      {fallbackPlan.executorConfigured ? 'executor ready' : 'no executor'}
                                    </Badge>
                                  )}
                                  {fallbackPlan && <Badge variant="neutral">{executorKindLabel(fallbackPlan.executorKind)}</Badge>}
                                  {fallbackPlan?.executorDryRun && <Badge variant="secondary">dry-run</Badge>}
                                  {fallbackAttemptCount > 0 && (
                                    <Badge variant="neutral">
                                      attempts {fallbackAttemptCount}
                                    </Badge>
                                  )}
                                  {target.serviceState.containerFallbackSuppressed && (
                                    <Badge variant="destructive">suppressed</Badge>
                                  )}
                                  {fallbackPlan?.backoffActive && (
                                    <Badge variant="secondary">backoff</Badge>
                                  )}
                                </div>
                                {fallbackDetail && (
                                  <span className="truncate text-xs text-[var(--bk-text-muted)]" title={fallbackDetail}>
                                    {fallbackDetail}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="flex items-center gap-2">
                                <Badge variant={runtimeErrors > 0 ? 'destructive' : 'neutral'}>{runtimeCount}</Badge>
                                {runtimeErrors > 0 && <span className="text-xs text-[var(--bk-text-muted)]">{runtimeErrors} attention</span>}
                              </div>
                            </TableCell>
                            <TableCell>{formatOptionalTime(target.checkedAt)}</TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            <Card tone="bento" className="rounded-lg">
              <CardHeader>
                <CardTitle>Runtime Status</CardTitle>
                <CardAction>
                  <Badge variant={attentionCount > 0 ? 'secondary' : 'success'}>
                    {attentionCount > 0 ? `${attentionCount} attention` : 'healthy'}
                  </Badge>
                </CardAction>
              </CardHeader>
              <CardContent>
                {runtimeRows.length === 0 ? (
                  <div className="flex h-28 items-center justify-center rounded-lg border border-dashed border-[var(--bk-border)] text-sm text-[var(--bk-text-muted)]">
                    暂无 runtime 状态
                  </div>
                ) : (
                  <Table tone="bento">
                    <TableHeader>
                      <TableRow>
                        <TableHead>Target</TableHead>
                        <TableHead>Runtime</TableHead>
                        <TableHead>Kind</TableHead>
                        <TableHead>Desired</TableHead>
                        <TableHead>Actual</TableHead>
                        <TableHead>Health</TableHead>
                        <TableHead>Restart</TableHead>
                        <TableHead>Next</TableHead>
                        <TableHead>Checked</TableHead>
                        <TableHead>Action</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {runtimeRows.map((runtime) => {
                        const restartPlan = runtime.applicationRestartPlan;
                        const restartPlanTitle = applicationRestartPlanTitle(restartPlan);
                        const restartPlanReason =
                          restartPlan?.blockedReason ||
                          restartPlan?.eligibleReason ||
                          restartPlan?.reason;
                        const canRestart = runtimeRestartSupported(runtime);
                        const restartSubmitting = controlSubmittingKey === runtimeControlActionKey(runtime, 'restart');
                        const suppressAction: RuntimeControlActionKind = runtime.autoRestartSuppressed ? 'resume-auto-restart' : 'suppress-auto-restart';
                        const suppressSubmitting = controlSubmittingKey === runtimeControlActionKey(runtime, suppressAction);
                        const autoRestartAudit = runtimeAutoRestartAudit(runtime);
                        return (
                          <TableRow key={`${runtime.targetName}:${runtime.runtimeKind}:${runtime.runtimeId}`}>
                            <TableCell>{runtime.targetName}</TableCell>
                            <TableCell>
                              <div className="flex max-w-[220px] flex-col gap-1">
                                <span className="font-mono text-xs text-[var(--bk-text-primary)]">{shrink(runtime.runtimeId)}</span>
                                {runtime.accountId && <span className="text-xs text-[var(--bk-text-muted)]">{shrink(runtime.accountId)}</span>}
                              </div>
                            </TableCell>
                            <TableCell><Badge variant="metal">{runtime.runtimeKind}</Badge></TableCell>
                            <TableCell><StatusBadge value={runtime.desiredStatus} /></TableCell>
                            <TableCell><StatusBadge value={runtime.actualStatus} /></TableCell>
                            <TableCell>
                              <div className="flex max-w-[220px] flex-col gap-1">
                                <div className="flex flex-wrap items-center gap-2">
                                  <StatusBadge value={runtime.health} />
                                  {autoRestartAudit && <Badge variant={autoRestartAudit.variant}>{autoRestartAudit.label}</Badge>}
                                </div>
                                {autoRestartAudit && (
                                  <span className="truncate text-xs text-[var(--bk-text-muted)]" title={runtimeAutoRestartAuditTitle(autoRestartAudit)}>
                                    {runtimeAutoRestartAuditText(autoRestartAudit)}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell>
                              <div className="flex max-w-[240px] flex-col gap-1">
                                <div className="flex flex-wrap items-center gap-1">
                                  <span className="font-mono text-sm tabular-nums">{runtime.restartAttempt}</span>
                                  {runtime.restartSeverity && <StatusBadge value={runtime.restartSeverity} />}
                                  {restartPlan && (
                                    <Badge variant={restartPlan.decision === 'eligible' ? 'success' : 'neutral'} title={restartPlanTitle}>
                                      {restartPlan.decision || 'blocked'}
                                    </Badge>
                                  )}
                                  {restartPlan && !restartPlan.supported && <Badge variant="secondary" title={restartPlanTitle}>unsupported</Badge>}
                                  {restartPlan?.duplicate && <Badge variant="neutral" title={restartPlanTitle}>duplicate</Badge>}
                                </div>
                                {restartPlanReason && (
                                  <span className="truncate text-xs text-[var(--bk-text-muted)]" title={restartPlanReason}>
                                    {restartPlanReason}
                                  </span>
                                )}
                                {runtime.lastRestartError && (
                                  <span className="truncate text-xs text-[var(--bk-text-muted)]" title={runtime.lastRestartError}>
                                    {runtime.lastRestartError}
                                  </span>
                                )}
                              </div>
                            </TableCell>
                            <TableCell>{formatOptionalTime(runtime.nextRestartAt)}</TableCell>
                            <TableCell>{formatOptionalTime(runtime.lastCheckedAt)}</TableCell>
                            <TableCell>
                              {canRestart ? (
                                <div className="flex items-center gap-1">
                                  <Button
                                    type="button"
                                    size="icon-sm"
                                    variant="bento-outline"
                                    className="rounded-lg"
                                    onClick={() => openControlDialog(runtime, 'restart')}
                                    disabled={Boolean(controlSubmittingKey) || restartSubmitting}
                                    title="Restart signal runtime"
                                    aria-label={`Restart signal runtime ${runtime.runtimeId}`}
                                  >
                                    <RotateCw className={cn(restartSubmitting && 'animate-spin')} />
                                  </Button>
                                  <Button
                                    type="button"
                                    size="icon-sm"
                                    variant={runtime.autoRestartSuppressed ? 'bento-outline' : 'bento-ghost'}
                                    className="rounded-lg"
                                    onClick={() => openControlDialog(runtime, suppressAction)}
                                    disabled={Boolean(controlSubmittingKey) || suppressSubmitting}
                                    title={runtime.autoRestartSuppressed ? 'Resume auto restart' : 'Suppress auto restart'}
                                    aria-label={`${runtime.autoRestartSuppressed ? 'Resume' : 'Suppress'} auto restart for ${runtime.runtimeId}`}
                                  >
                                    {runtime.autoRestartSuppressed ? (
                                      <CheckCircle2 className={cn(suppressSubmitting && 'animate-pulse')} />
                                    ) : (
                                      <ShieldAlert className={cn(suppressSubmitting && 'animate-pulse')} />
                                    )}
                                  </Button>
                                </div>
                              ) : (
                                <span className="text-xs text-[var(--bk-text-muted)]">--</span>
                              )}
                            </TableCell>
                          </TableRow>
                        );
                      })}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            <Card tone="bento" className="rounded-lg">
              <CardHeader>
                <CardTitle>Control Actions</CardTitle>
                <CardAction>
                  <div className="flex flex-wrap justify-end gap-2">
                    {controlError && <Badge variant="destructive" title={controlError}>control failed</Badge>}
                    {controlNotice && <Badge variant="success" title={controlNotice}>control accepted</Badge>}
                    <Badge variant="neutral">{controlActionRows.length}</Badge>
                  </div>
                </CardAction>
              </CardHeader>
              <CardContent>
                {controlActionRows.length === 0 ? (
                  <div className="flex h-24 items-center justify-center rounded-lg border border-dashed border-[var(--bk-border)] text-sm text-[var(--bk-text-muted)]">
                    暂无应用内控制动作
                  </div>
                ) : (
                  <Table tone="bento">
                    <TableHeader>
                      <TableRow>
                        <TableHead>Requested</TableHead>
                        <TableHead>Target</TableHead>
                        <TableHead>Action</TableHead>
                        <TableHead>Runtime</TableHead>
                        <TableHead>Status</TableHead>
                        <TableHead>Reason</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {controlActionRows.map((action) => (
                        <TableRow key={`${action.targetName}:${action.runtimeKind}:${action.runtimeId}:${action.requestedAt}`}>
                          <TableCell>{formatOptionalTime(action.requestedAt)}</TableCell>
                          <TableCell>{action.targetName}</TableCell>
                          <TableCell><Badge variant="metal">{action.action}</Badge></TableCell>
                          <TableCell>
                            <div className="flex flex-col gap-1">
                              <span className="font-mono text-xs">{shrink(action.runtimeId)}</span>
                              <span className="text-xs text-[var(--bk-text-muted)]">{action.runtimeKind}</span>
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex items-center gap-2">
                              {action.submitted ? (
                                <CheckCircle2 className="size-4 text-[var(--bk-status-success)]" />
                              ) : (
                                <XCircle className="size-4 text-[var(--bk-status-danger)]" />
                              )}
                              <Badge variant={action.error ? 'destructive' : 'neutral'}>{action.statusCode || '--'}</Badge>
                            </div>
                          </TableCell>
                          <TableCell>
                            <span className="block max-w-[420px] truncate text-xs text-[var(--bk-text-muted)]" title={action.error || action.reason}>
                              {action.error || action.reason || '--'}
                            </span>
                          </TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>
          </>
        )}
      </div>
      <Dialog open={Boolean(controlDialog)} onOpenChange={(open) => !open && closeControlDialog()}>
        <DialogContent tone="bento" className="max-w-lg rounded-lg border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-0">
          <DialogHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] px-6 py-4">
            <DialogTitle className="text-lg font-semibold text-[var(--bk-text-primary)]">
              {controlDialog ? runtimeControlLabel(controlDialog.action) : 'Runtime Control'}
            </DialogTitle>
            <DialogDescription className="text-sm text-[var(--bk-text-muted)]">
              {controlDialog ? `${controlDialog.runtime.targetName} / ${shrink(controlDialog.runtime.runtimeId)}` : '--'}
            </DialogDescription>
          </DialogHeader>
          <div className="flex flex-col gap-4 px-6 py-5">
            <div className="flex flex-col gap-2">
              <label htmlFor="supervisor-runtime-restart-reason" className="text-xs font-medium uppercase text-[var(--bk-text-muted)]">
                Reason
              </label>
              <Textarea
                id="supervisor-runtime-restart-reason"
                value={controlReason}
                onChange={(event) => setControlReason(event.target.value)}
                disabled={Boolean(controlSubmittingKey)}
                rows={4}
                aria-invalid={controlReason.trim() === ''}
              />
            </div>
            {controlError && (
              <div className="rounded-lg border border-[var(--bk-status-danger)] bg-[var(--bk-status-danger-soft)] px-3 py-2 text-sm text-[var(--bk-status-danger)]">
                {controlError}
              </div>
            )}
          </div>
          <DialogFooter className="border-t border-[var(--bk-border-soft)] bg-[var(--bk-surface-muted)]/30 px-6 py-4">
            <Button
              type="button"
              variant="bento-outline"
              className="rounded-lg"
              onClick={closeControlDialog}
              disabled={Boolean(controlSubmittingKey)}
            >
              Cancel
            </Button>
            <Button
              type="button"
              variant="bento-destructive"
              className="rounded-lg"
              onClick={() => void submitRuntimeControl()}
              disabled={!controlReason.trim() || Boolean(controlSubmittingKey)}
            >
              <RotateCw data-icon="inline-start" className={cn(controlSubmittingKey && 'animate-spin')} />
              Submit
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  );
}
