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
  XCircle,
} from 'lucide-react';
import { Badge } from '../components/ui/badge';
import { Button } from '../components/ui/button';
import { Card, CardAction, CardContent, CardHeader, CardTitle } from '../components/ui/card';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow,
} from '../components/ui/table';
import { Separator } from '../components/ui/separator';
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

export function SupervisorStage() {
  const [snapshot, setSnapshot] = useState<RuntimeSupervisorSnapshot | null>(null);
  const [loadState, setLoadState] = useState<LoadState>('idle');
  const [error, setError] = useState<string | null>(null);

  const loadSnapshot = useCallback(async (silent = false) => {
    if (!silent) {
      setLoadState('loading');
    }
    try {
      const payload = await fetchJSON<RuntimeSupervisorSnapshot>('/api/v1/supervisor/status');
      setSnapshot(payload);
      setError(null);
      setLoadState('loaded');
    } catch (err) {
      setError(err instanceof Error ? err.message : '读取 supervisor 状态失败');
      setLoadState('error');
    }
  }, []);

  useEffect(() => {
    void loadSnapshot();
    const timer = window.setInterval(() => {
      void loadSnapshot(true);
    }, REFRESH_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [loadSnapshot]);

  const { runtimeRows, controlActionRows, fallbackCount, attentionCount, reachableTargets } = useMemo(() => {
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
      attentionCount: runtimes.filter(runtimeNeedsAttention).length,
      reachableTargets: targets.filter((target) => isProbeOK(target.healthz) && isProbeOK(target.runtimeStatus)).length,
    };
  }, [snapshot]);

  const targets = snapshot?.targets ?? [];
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
                detail={`${reachableTargets}/${targets.length} reachable`}
                tone={reachableTargets === targets.length && targets.length > 0 ? 'success' : 'warning'}
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
                detail="container candidates"
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
                                <Badge variant={target.serviceState.containerFallbackCandidate ? 'destructive' : 'neutral'}>
                                  {target.serviceState.containerFallbackCandidate ? 'candidate' : 'clear'}
                                </Badge>
                                {target.serviceState.containerFallbackReason && (
                                  <span className="truncate text-xs text-[var(--bk-text-muted)]" title={target.serviceState.containerFallbackReason}>
                                    {target.serviceState.containerFallbackReason}
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
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {runtimeRows.map((runtime) => (
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
                            <div className="flex items-center gap-2">
                              <StatusBadge value={runtime.health} />
                              {runtime.autoRestartSuppressed && <Badge variant="destructive">suppressed</Badge>}
                            </div>
                          </TableCell>
                          <TableCell>
                            <div className="flex flex-col gap-1">
                              <span className="font-mono text-sm tabular-nums">{runtime.restartAttempt}</span>
                              {runtime.restartSeverity && <StatusBadge value={runtime.restartSeverity} />}
                              {runtime.lastRestartError && (
                                <span className="max-w-[220px] truncate text-xs text-[var(--bk-text-muted)]" title={runtime.lastRestartError}>
                                  {runtime.lastRestartError}
                                </span>
                              )}
                            </div>
                          </TableCell>
                          <TableCell>{formatOptionalTime(runtime.nextRestartAt)}</TableCell>
                          <TableCell>{formatOptionalTime(runtime.lastCheckedAt)}</TableCell>
                        </TableRow>
                      ))}
                    </TableBody>
                  </Table>
                )}
              </CardContent>
            </Card>

            <Card tone="bento" className="rounded-lg">
              <CardHeader>
                <CardTitle>Control Actions</CardTitle>
                <CardAction>
                  <Badge variant="neutral">{controlActionRows.length}</Badge>
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
    </div>
  );
}
