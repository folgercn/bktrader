import React from 'react';
import { ArrowRightLeft, Activity } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { LiveTradePair } from '../../types/domain';
import { formatMaybeNumber, formatSigned, formatTime, shrink } from '../../utils/format';
import { cn } from '../../lib/utils';

type LiveTradePairsCardProps = {
  title: string;
  description: string;
  pairs: LiveTradePair[];
  loading: boolean;
  error: string | null;
  className?: string;
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

export function LiveTradePairsCard({
  title,
  description,
  pairs,
  loading,
  error,
  className,
}: LiveTradePairsCardProps) {
  return (
    <Card tone="bento" className={cn('rounded-[24px] shadow-[var(--bk-shadow-card)]', className)}>
      <CardHeader className="pb-4">
        <div className="flex items-center justify-between">
          <div className="space-y-1">
            <div className="flex items-center gap-2">
              <ArrowRightLeft size={16} className="text-[var(--bk-status-success)]" />
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">{title}</CardTitle>
            </div>
            <CardDescription className="text-[11px] font-medium text-[var(--bk-text-muted)]">
              {description}
            </CardDescription>
          </div>
          <Badge variant="metal">{pairs.length} 笔</Badge>
        </div>
      </CardHeader>
      <CardContent>
        {loading ? (
          <div className="flex items-center justify-center gap-3 rounded-[20px] border border-[var(--bk-border)] bg-[var(--bk-surface-faint)] p-10 text-[var(--bk-text-muted)]">
            <Activity size={16} className="animate-pulse" />
            <span className="text-xs font-bold">正在聚合开平订单对…</span>
          </div>
        ) : error ? (
          <div className="rounded-[20px] border border-[var(--bk-status-danger-soft)] bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)] p-4 text-[11px] font-medium text-[var(--bk-status-danger)]">
            {error}
          </div>
        ) : pairs.length === 0 ? (
          <div className="rounded-[20px] border border-dashed border-[var(--bk-border)] p-10 text-center text-[11px] font-medium text-[var(--bk-text-muted)]">
            当前会话还没有可追溯的开平订单对。
          </div>
        ) : (
          <div className="overflow-hidden rounded-[20px] border border-[var(--bk-border)] bg-[var(--bk-surface-strong)]">
            <Table>
              <TableHeader>
                <TableRow>
                  <TableHead>状态</TableHead>
                  <TableHead>方向</TableHead>
                  <TableHead>开仓</TableHead>
                  <TableHead>平仓</TableHead>
                  <TableHead>数量</TableHead>
                  <TableHead>退出</TableHead>
                  <TableHead>PnL</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {pairs.map((pair) => {
                  const netPositive = Number(pair.netPnl ?? 0) >= 0;
                  const quantity = String(pair.status).toLowerCase() === 'open' ? pair.openQuantity : pair.entryQuantity;
                  return (
                    <TableRow key={pair.id}>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <Badge variant={String(pair.status).toLowerCase() === 'open' ? 'neutral' : 'success'}>
                            {tradePairStatusLabel(pair.status)}
                          </Badge>
                          <div className={cn('text-[10px] font-black', tradePairVerdictTone(pair.exitVerdict))}>
                            {tradePairVerdictLabel(pair.exitVerdict)}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <div className="text-[12px] font-black text-[var(--bk-text-primary)]">{pair.side}</div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">{pair.symbol}</div>
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                            {formatMaybeNumber(pair.entryAvgPrice)}
                          </div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">{formatTime(pair.entryAt)}</div>
                          <div className="text-[10px] font-bold uppercase text-[var(--bk-text-muted)]">
                            {pair.entryReason || '--'}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        {String(pair.status).toLowerCase() === 'closed' ? (
                          <div className="space-y-1">
                            <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                              {formatMaybeNumber(pair.exitAvgPrice)}
                            </div>
                            <div className="text-[10px] text-[var(--bk-text-muted)]">
                              {pair.exitAt ? formatTime(pair.exitAt) : '--'}
                            </div>
                            <div className="text-[10px] font-bold uppercase text-[var(--bk-text-muted)]">
                              {pair.exitReason || '--'}
                            </div>
                          </div>
                        ) : (
                          <div className="space-y-1">
                            <div className="text-[11px] font-black text-[var(--bk-text-primary)]">持仓中</div>
                            <div className="text-[10px] text-[var(--bk-text-muted)]">
                              未实现 {formatSigned(pair.unrealizedPnl)}
                            </div>
                          </div>
                        )}
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <div className="font-mono text-[12px] font-black text-[var(--bk-text-primary)]">
                            {formatMaybeNumber(quantity)}
                          </div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">
                            开 {pair.entryFillCount} / 平 {pair.exitFillCount}
                          </div>
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <div className="text-[12px] font-black text-[var(--bk-text-primary)]">
                            {pair.exitClassifier || '--'}
                          </div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">
                            费 {formatMaybeNumber(pair.fees, 6)}
                          </div>
                          {pair.notes && pair.notes.length > 0 && (
                            <div className="text-[10px] text-[var(--bk-text-muted)]">
                              {shrink(pair.notes.join(', '))}
                            </div>
                          )}
                        </div>
                      </TableCell>
                      <TableCell className="align-top">
                        <div className="space-y-1">
                          <div className={cn('font-mono text-[12px] font-black', netPositive ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]')}>
                            {formatSigned(pair.netPnl)}
                          </div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">
                            已实 {formatSigned(pair.realizedPnl)}
                          </div>
                          <div className="text-[10px] text-[var(--bk-text-muted)]">
                            未实 {formatSigned(pair.unrealizedPnl)}
                          </div>
                        </div>
                      </TableCell>
                    </TableRow>
                  );
                })}
              </TableBody>
            </Table>
          </div>
        )}
      </CardContent>
    </Card>
  );
}
