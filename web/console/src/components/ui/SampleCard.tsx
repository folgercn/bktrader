import { ReplaySample } from '../../types/domain';
import { sampleStatus } from '../../utils/derivation';
import { formatMaybeNumber } from '../../utils/format';

export function SampleCard(props: { sample: ReplaySample; selected?: boolean; onSelect: () => void }) {
  const sample = props.sample;
  const status = sampleStatus(sample);
  return (
    <button
      type="button"
      className={`sample-card sample-card-button ${props.selected ? "sample-card-selected" : ""}`}
      onClick={props.onSelect}
    >
      <div className="sample-header">
        <div className="sample-title">{String(sample.reason ?? sample.entryCause ?? "sample")}</div>
        <span className={`sample-status sample-status-${status.tone}`}>{status.label}</span>
      </div>
      <div className="sample-line">
        entry: {String(sample.entryTime ?? "--")} · {formatMaybeNumber(sample.entryPrice)}
      </div>
      <div className="sample-line">
        exit: {String(sample.exitTime ?? "--")} · {formatMaybeNumber(sample.exitPrice)}
      </div>
      <div className="sample-line">
        fill: {formatMaybeNumber(sample.bracketEntryFill)} → {formatMaybeNumber(sample.bracketExitPrice)}
      </div>
      <div className="sample-line">
        cause: {String(sample.entryCause ?? "--")} / {String(sample.exitCause ?? sample.bracketExitType ?? "--")}
      </div>
      <div className="sample-line">pnl: {formatMaybeNumber(sample.bracketRealizedPnL)}</div>
    </button>
  );
}
