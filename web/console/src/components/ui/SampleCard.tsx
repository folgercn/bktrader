import React from 'react';
import { ReplaySample } from '../../types/domain';
import { sampleStatus } from '../../utils/derivation';
import { formatMaybeNumber } from '../../utils/format';
import { Badge } from './badge';

export function SampleCard(props: { sample: ReplaySample; selected?: boolean; onSelect: () => void }) {
  const sample = props.sample;
  const status = sampleStatus(sample);
  
  return (
    <button
      type="button"
      className={`w-full text-left p-3 rounded-xl border transition-all duration-200 group ${
        props.selected 
          ? "border-[var(--bk-border-strong)] bg-[var(--bk-surface-muted)] shadow-inner" 
          : "border-[var(--bk-border)] bg-[color:color-mix(in_srgb,var(--bk-accent-soft)_50%,var(--bk-surface)_50%)] hover:bg-[var(--bk-surface)] hover:shadow-sm"
      }`}
      onClick={props.onSelect}
    >
      <div className="flex items-center justify-between mb-2">
        <div className={`text-[11px] font-black uppercase tracking-tight ${props.selected ? 'text-[var(--bk-text-primary)]' : 'text-[var(--bk-text-secondary)]'}`}>
          {String(sample.reason ?? sample.entryCause ?? "trade sample")}
        </div>
        <Badge className={`text-[9px] h-4 px-1 ${
          status.tone === 'positive' ? 'bg-[var(--bk-status-success)] text-[var(--bk-canvas)]' : 
          status.tone === 'negative' ? 'bg-[var(--bk-status-danger)] text-[var(--bk-canvas)]' : 'bg-[var(--bk-text-muted)] text-[var(--bk-canvas)]'
        }`}>
          {status.label}
        </Badge>
      </div>

      <div className="space-y-1.5">
        <div className="flex justify-between items-center text-[10px]">
          <span className="font-medium text-[color:color-mix(in_srgb,var(--bk-text-secondary)_70%,transparent)]">成交详情</span>
          <span className="font-mono font-bold text-[var(--bk-text-primary)]">{formatMaybeNumber(sample.bracketEntryFill)} → {formatMaybeNumber(sample.bracketExitPrice)}</span>
        </div>
        
        <div className="grid grid-cols-2 gap-2 text-[9px] font-mono leading-tight">
          <div className="text-[var(--bk-text-secondary)]">
            IN: {String(sample.entryTime ?? "--").split('T')[1]?.slice(0, 5) ?? "--"}
          </div>
          <div className="text-right text-[var(--bk-text-secondary)]">
            OUT: {String(sample.exitTime ?? "--").split('T')[1]?.slice(0, 5) ?? "--"}
          </div>
        </div>

        <div className="flex justify-between items-center pt-1 border-t border-[color:color-mix(in_srgb,var(--bk-border)_30%,transparent)]">
          <span className="text-[9px] font-bold uppercase text-[color:color-mix(in_srgb,var(--bk-text-primary)_60%,transparent)]">{String(sample.exitCause ?? sample.bracketExitType ?? "unknown")}</span>
          <span className={`text-xs font-black ${Number(sample.bracketRealizedPnL) >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'}`}>
            {Number(sample.bracketRealizedPnL) > 0 ? '+' : ''}{formatMaybeNumber(sample.bracketRealizedPnL)}
          </span>
        </div>
      </div>
    </button>
  );
}
