import * as React from 'react';
import { ArrowRightLeft, Activity, CheckCircle2, AlertCircle, Loader2 } from 'lucide-react';
import { Card, CardContent, CardDescription, CardHeader, CardTitle } from '../ui/card';
import { Badge } from '../ui/badge';
import { Table, TableBody, TableCell, TableHead, TableHeader, TableRow } from '../ui/table';
import { 
  Dialog, 
  DialogContent, 
  DialogHeader, 
  DialogTitle, 
  DialogDescription, 
  DialogFooter,
  DialogClose
} from '../ui/dialog';
import { Textarea } from '../ui/textarea';
import { Button } from '../ui/button';
import { toast } from 'sonner';
import { LiveTradePair } from '../../types/domain';
import { formatMaybeNumber, formatSigned, formatTime, shrink } from '../../utils/format';
import { cn } from '../../lib/utils';

type LiveTradePairsCardProps = {
  title: string;
  description: string;
  pairs: LiveTradePair[];
  loading: boolean;
  error: string | null;
  sessionId?: string | null;
  onRefresh?: () => void;
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
  sessionId,
  onRefresh,
  className,
}: LiveTradePairsCardProps) {
  // Manual Review State
  const [selectedPairForReview, setSelectedPairForReview] = React.useState<any | null>(null);
  const [reviewNotes, setReviewNotes] = React.useState("人工审核对账完毕，确认无持仓。");
  const [isVerifying, setIsVerifying] = React.useState(false);

  const handleManualVerify = async () => {
    if (!selectedPairForReview || !sessionId) return;
    
    const lastOrderId = selectedPairForReview.exitOrderIds?.[selectedPairForReview.exitOrderIds.length - 1];
    if (!lastOrderId) {
      toast.error("找不到有效的出场订单 ID，无法核验");
      return;
    }

    setIsVerifying(true);
    try {
      const resp = await fetch(`/api/v1/live/sessions/${sessionId}/orders/${lastOrderId}/verifications`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ notes: reviewNotes })
      });

      if (!resp.ok) {
        const err = await resp.text();
        throw new Error(err || "请求失败");
      }

      toast.success("复核成功");
      setSelectedPairForReview(null);
      onRefresh?.();
    } catch (err: any) {
      toast.error(`复核失败: ${err.message}`);
    } finally {
      setIsVerifying(false);
    }
  };

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
                          <div 
                            className={cn(
                              'text-[10px] font-black', 
                              tradePairVerdictTone(pair.exitVerdict),
                              String(pair.exitVerdict).toLowerCase() === 'mismatch' && "cursor-pointer hover:underline flex items-center gap-1"
                            )}
                            onClick={() => {
                              if (String(pair.exitVerdict).toLowerCase() === 'mismatch') {
                                setSelectedPairForReview(pair);
                              }
                            }}
                          >
                            {tradePairVerdictLabel(pair.exitVerdict)}
                            {String(pair.exitVerdict).toLowerCase() === 'mismatch' && <AlertCircle size={10} />}
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

      {/* Manual Review Dialog */}
      <Dialog open={!!selectedPairForReview} onOpenChange={(open) => !open && setSelectedPairForReview(null)}>
        <DialogContent tone="bento" className="max-w-md rounded-[32px] border-[var(--bk-border)] bg-[var(--bk-surface-overlay-strong)] p-0 overflow-hidden shadow-2xl">
          <div className="bg-[var(--bk-surface-muted)]/30 px-6 py-5 border-b border-[var(--bk-border-soft)]">
            <DialogHeader>
              <div className="flex items-center gap-3 mb-1">
                <div className="flex size-8 items-center justify-center rounded-xl bg-amber-500/10 text-amber-500">
                  <CheckCircle2 size={18} />
                </div>
                <DialogTitle className="text-lg font-black text-[var(--bk-text-primary)]">人工复核平仓异常</DialogTitle>
              </div>
              <DialogDescription className="text-[11px] font-medium text-[var(--bk-text-muted)]">
                您正在手动标记一笔交易为“已核验”。请确保交易所内该币种已无实际持仓。
              </DialogDescription>
            </DialogHeader>
          </div>

          <div className="p-6 space-y-4">
            {selectedPairForReview && (
              <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)]/50 p-4 space-y-2">
                <div className="flex justify-between text-[11px]">
                  <span className="font-bold text-[var(--bk-text-muted)]">交易对</span>
                  <span className="font-black text-[var(--bk-text-primary)]">{selectedPairForReview.symbol} {selectedPairForReview.side}</span>
                </div>
                <div className="flex justify-between text-[11px]">
                  <span className="font-bold text-[var(--bk-text-muted)]">开仓时间</span>
                  <span className="font-mono text-[var(--bk-text-primary)]">{formatTime(selectedPairForReview.entryAt)}</span>
                </div>
                <div className="flex justify-between text-[11px]">
                  <span className="font-bold text-[var(--bk-text-muted)]">成交数量</span>
                  <span className="font-mono font-black text-[var(--bk-text-primary)]">{selectedPairForReview.entryQuantity}</span>
                </div>
              </div>
            )}

            <div className="space-y-2">
              <label className="text-[10px] font-black uppercase tracking-wider text-[var(--bk-text-muted)] opacity-70">复核备注</label>
              <Textarea 
                className="min-h-[80px] rounded-2xl text-[12px] font-medium resize-none"
                placeholder="请输入核验说明..."
                value={reviewNotes}
                onChange={(e) => setReviewNotes(e.target.value)}
              />
            </div>
          </div>

          <DialogFooter className="bg-[var(--bk-surface-muted)]/30 px-6 py-4 border-t border-[var(--bk-border-soft)] flex items-center justify-end gap-3">
            <DialogClose
              render={
                <Button variant="ghost" className="h-10 rounded-xl px-4 text-[12px] font-black" />
              }
            >
              取消
            </DialogClose>
            <Button 
              disabled={isVerifying}
              onClick={handleManualVerify}
              className="h-10 rounded-xl px-6 bg-[var(--bk-status-success)] hover:bg-[var(--bk-status-success)]/90 text-white text-[12px] font-black shadow-lg shadow-emerald-500/20"
            >
              {isVerifying ? (
                <>
                  <Loader2 className="mr-2 h-4 w-4 animate-spin" />
                  提交中...
                </>
              ) : "确认无持仓并平账"}
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </Card>
  );
}
