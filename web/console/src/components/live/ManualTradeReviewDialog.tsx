import * as React from 'react';
import { CheckCircle2, Loader2 } from 'lucide-react';
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
import { formatTime } from '../../utils/format';

interface ManualTradeReviewDialogProps {
  pair: LiveTradePair | null;
  sessionId: string | null | undefined;
  onClose: () => void;
  onSuccess: () => void;
}

export function ManualTradeReviewDialog({ pair, sessionId, onClose, onSuccess }: ManualTradeReviewDialogProps) {
  const [reviewNotes, setReviewNotes] = React.useState("人工审核对账完毕，确认无持仓。");
  const [isVerifying, setIsVerifying] = React.useState(false);

  // Reset notes when pair changes
  React.useEffect(() => {
    if (pair) {
      setReviewNotes("人工审核对账完毕，确认无持仓。");
    }
  }, [pair]);

  const handleManualVerify = async () => {
    if (!pair || !sessionId) return;
    
    const lastOrderId = pair.exitOrderIds?.[pair.exitOrderIds.length - 1];
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

      toast.success("复核成功，订单状态已同步");
      onSuccess();
      onClose();
    } catch (err) {
      console.error('Manual verification failed', err);
      toast.error(err instanceof Error ? err.message : "核验提交失败");
    } finally {
      setIsVerifying(false);
    }
  };

  return (
    <Dialog open={!!pair} onOpenChange={(open) => !open && onClose()}>
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
          {pair && (
            <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-faint)]/50 p-4 space-y-2">
              <div className="flex justify-between text-[11px]">
                <span className="font-bold text-[var(--bk-text-muted)]">交易对</span>
                <span className="font-black text-[var(--bk-text-primary)]">{pair.symbol} {pair.side}</span>
              </div>
              <div className="flex justify-between text-[11px]">
                <span className="font-bold text-[var(--bk-text-muted)]">开仓时间</span>
                <span className="font-mono text-[var(--bk-text-primary)]">{formatTime(pair.entryAt)}</span>
              </div>
              <div className="flex justify-between text-[11px]">
                <span className="font-bold text-[var(--bk-text-muted)]">成交数量</span>
                <span className="font-mono font-black text-[var(--bk-text-primary)]">{pair.entryQuantity}</span>
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
  );
}
