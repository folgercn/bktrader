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
          ? "bg-[#ebe5d5] border-[#1f2328] shadow-inner" 
          : "bg-[#fff8ea] border-[#d8cfba] hover:bg-white hover:shadow-sm"
      }`}
      onClick={props.onSelect}
    >
      <div className="flex items-center justify-between mb-2">
        <div className={`text-[11px] font-black uppercase tracking-tight ${props.selected ? 'text-[#1f2328]' : 'text-[#687177]'}`}>
          {String(sample.reason ?? sample.entryCause ?? "trade sample")}
        </div>
        <Badge className={`text-[9px] h-4 px-1 ${
          status.tone === 'positive' ? 'bg-[#0e6d60]' : 
          status.tone === 'negative' ? 'bg-rose-600' : 'bg-zinc-400'
        }`}>
          {status.label}
        </Badge>
      </div>

      <div className="space-y-1.5">
        <div className="flex justify-between items-center text-[10px]">
          <span className="text-[#687177]/70 font-medium">成交详情</span>
          <span className="font-mono text-[#1f2328] font-bold">{formatMaybeNumber(sample.bracketEntryFill)} → {formatMaybeNumber(sample.bracketExitPrice)}</span>
        </div>
        
        <div className="grid grid-cols-2 gap-2 text-[9px] font-mono leading-tight">
          <div className="text-[#687177]">
            IN: {String(sample.entryTime ?? "--").split('T')[1]?.slice(0, 5) ?? "--"}
          </div>
          <div className="text-[#687177] text-right">
            OUT: {String(sample.exitTime ?? "--").split('T')[1]?.slice(0, 5) ?? "--"}
          </div>
        </div>

        <div className="flex justify-between items-center pt-1 border-t border-[#d8cfba]/30">
          <span className="text-[9px] font-bold text-[#1f2328]/60 uppercase">{String(sample.exitCause ?? sample.bracketExitType ?? "unknown")}</span>
          <span className={`text-xs font-black ${Number(sample.bracketRealizedPnL) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
            {Number(sample.bracketRealizedPnL) > 0 ? '+' : ''}{formatMaybeNumber(sample.bracketRealizedPnL)}
          </span>
        </div>
      </div>
    </button>
  );
}
