import React, { useMemo } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { SampleCard } from '../components/ui/SampleCard';
import { API_BASE } from '../utils/api';
import { formatTime, formatPercent, formatSigned, formatMaybeNumber } from '../utils/format';
import { strategyLabel, getNumber, buildSampleKey, buildSampleRange } from '../utils/derivation';
import { ReplayReasonStats, ExecutionTrade, ReplaySample } from '../types/domain';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from '../components/ui/select';
import { 
  Table, 
  TableBody, 
  TableCell, 
  TableHead, 
  TableHeader, 
  TableRow 
} from '../components/ui/table';
import { Badge } from '../components/ui/badge';
import { 
  Play, 
  History, 
  FileDown, 
  Maximize2, 
  Database, 
  FlaskConical, 
  Clock, 
  BarChart4, 
  ExternalLink 
} from 'lucide-react';

interface StrategySidePanelProps {
  createBacktestRun: () => Promise<void>;
}

export function StrategySidePanel({ createBacktestRun }: StrategySidePanelProps) {
  const backtestForm = useUIStore(s => s.backtestForm);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);
  const backtestAction = useUIStore(s => s.backtestAction);
  const backtestOptions = useTradingStore(s => s.backtestOptions);
  const strategies = useTradingStore(s => s.strategies);
  const backtests = useTradingStore(s => s.backtests);
  const selectedBacktestId = useUIStore(s => s.selectedBacktestId);
  const setSelectedBacktestId = useUIStore(s => s.setSelectedBacktestId);
  const chartOverrideRange = useUIStore(s => s.chartOverrideRange);
  const setChartOverrideRange = useUIStore(s => s.setChartOverrideRange);
  const setFocusNonce = useUIStore(s => s.setFocusNonce);
  const selectedSample = useUIStore(s => s.selectedSample);
  const setSelectedSample = useUIStore(s => s.setSelectedSample);
  const setSourceFilter = useUIStore(s => s.setSourceFilter);
  const setEventFilter = useUIStore(s => s.setEventFilter);

  // Derived states
  const selectedExecutionAvailability = backtestOptions?.availability?.[backtestForm.executionDataSource] ?? "unknown";
  const selectedExecutionSymbols = backtestOptions?.supportedSymbols?.[backtestForm.executionDataSource] ?? [];
  const selectedSymbolAvailable =
    selectedExecutionSymbols.length === 0 || selectedExecutionSymbols.includes(backtestForm.symbol.trim().toUpperCase());
  const backtestItems = backtests.slice().reverse().slice(0, 8);
  const selectedBacktest =
    backtests.find((item) => item.id === selectedBacktestId) ??
    (backtests.length > 0 ? backtests[backtests.length - 1] : null);
  const latestBacktestSummary = (selectedBacktest?.resultSummary ?? {}) as Record<string, unknown>;
  const latestExecutionSource = String(latestBacktestSummary.executionDataSource ?? selectedBacktest?.parameters?.executionDataSource ?? "");
  
  const latestExecutionTrades = Array.isArray(latestBacktestSummary.executionTrades)
    ? (latestBacktestSummary.executionTrades as ExecutionTrade[])
    : [];
  const latestReplaySkippedSamples = Array.isArray(latestBacktestSummary.replayLedgerSkippedSamples)
    ? (latestBacktestSummary.replayLedgerSkippedSamples as ReplaySample[])
    : [];
  const latestReplayCompletedSamples = Array.isArray(latestBacktestSummary.replayLedgerCompletedSamples)
    ? (latestBacktestSummary.replayLedgerCompletedSamples as ReplaySample[])
    : [];

  return (
    <div className="space-y-6 p-4 animate-in slide-in-from-right duration-500">
      {/* 1. 回测配置面板 */}
      <Card tone="bento" className="overflow-hidden rounded-[24px] shadow-lg">
        <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] py-4 px-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-[var(--bk-surface-muted)] p-1.5">
                <FlaskConical className="size-4 text-[var(--bk-text-primary)]" />
              </div>
              <CardTitle className="text-base font-bold text-[var(--bk-text-primary)]">回测配置</CardTitle>
            </div>
            <Badge variant="outline" className="border-[var(--bk-border)] text-[9px] font-mono tracking-tight text-[var(--bk-text-secondary)]">
              {backtests.length} RUNS
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="p-5 space-y-4">
          <div className="grid grid-cols-1 gap-4">
            <div className="space-y-1.5">
              <Label className="ml-0.5 text-[11px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]">Strategy</Label>
              <Select 
                value={backtestForm.strategyVersionId}
                onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, strategyVersionId: val }))}
              >
                <SelectTrigger tone="bento" className="h-9 rounded-xl text-[12px] font-medium">
                  <SelectValue placeholder="选择策略版本" />
                </SelectTrigger>
                <SelectContent tone="bento" className="rounded-xl shadow-xl">
                  {strategies.map((strategy) => (
                    <SelectItem key={strategy.id} value={strategy.currentVersion?.id ?? ""}>
                      {strategyLabel(strategy)}
                    </SelectItem>
                  ))}
                </SelectContent>
              </Select>
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="ml-0.5 text-[11px] font-black uppercase text-[var(--bk-text-secondary)]">Timeframe</Label>
                <Select 
                  value={backtestForm.signalTimeframe}
                  onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, signalTimeframe: val }))}
                >
                  <SelectTrigger tone="bento" className="h-9 rounded-xl text-[12px]">
                    <SelectValue placeholder="周期" />
                  </SelectTrigger>
                  <SelectContent tone="bento" className="rounded-xl">
                    {(backtestOptions?.signalTimeframes ?? ["5m", "4h", "1d"]).map((item) => (
                      <SelectItem key={item} value={item}>{item}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="ml-0.5 text-[11px] font-black uppercase text-[var(--bk-text-secondary)]">Source</Label>
                <Select 
                  value={backtestForm.executionDataSource}
                  onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, executionDataSource: val }))}
                >
                  <SelectTrigger tone="bento" className="h-9 rounded-xl text-[12px]">
                    <SelectValue placeholder="数据源" />
                  </SelectTrigger>
                  <SelectContent tone="bento" className="rounded-xl">
                    {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                      <SelectItem key={item} value={item}>{item}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="ml-0.5 text-[11px] font-black uppercase text-[var(--bk-text-secondary)]">Symbol</Label>
              <Input 
                value={backtestForm.symbol}
                onChange={(e) => setBacktestForm(curr => ({ ...curr, symbol: e.target.value.toUpperCase() }))}
                placeholder="例如：BTCUSDT"
                className={`h-9 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] text-[12px] font-mono font-bold focus:bg-[var(--bk-surface)] ${!selectedSymbolAvailable ? 'border-[var(--bk-status-danger)] ring-2 ring-[color:color-mix(in_srgb,var(--bk-status-danger)_14%,transparent)]' : ''}`}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="ml-0.5 text-[11px] font-black uppercase text-[var(--bk-text-secondary)]">From</Label>
                <Input 
                  value={backtestForm.from}
                  onChange={(e) => setBacktestForm(curr => ({ ...curr, from: e.target.value }))}
                  placeholder="2024-01-01..."
                  className="h-9 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] text-[10px] font-mono"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="ml-0.5 text-[11px] font-black uppercase text-[var(--bk-text-secondary)]">To</Label>
                <Input 
                  value={backtestForm.to}
                  onChange={(e) => setBacktestForm(curr => ({ ...curr, to: e.target.value }))}
                  placeholder="2024-01-31..."
                  className="h-9 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] text-[10px] font-mono"
                />
              </div>
            </div>
          </div>

          <Button 
            variant="bento"
            className="h-11 w-full rounded-2xl text-sm font-black shadow-md active:scale-95"
            disabled={
              backtestAction ||
              backtestForm.strategyVersionId.trim() === "" ||
              backtestForm.symbol.trim() === "" ||
              selectedExecutionAvailability === "missing" ||
              !selectedSymbolAvailable
            }
            onClick={createBacktestRun}
          >
            <Play className="size-4 mr-2 border-none" />
            {backtestAction ? "正在计算路径..." : "启动压力测试"}
          </Button>

          {backtestOptions && (
            <div className="space-y-2 rounded-xl border border-[var(--bk-border-soft)] bg-[color:color-mix(in_srgb,var(--bk-accent-soft)_55%,var(--bk-surface)_45%)] p-3">
              <div className="flex items-center gap-2 text-[10px] font-bold text-[var(--bk-text-secondary)]">
                 <Database className="size-3" />
                 <span>数据就绪检查</span>
              </div>
              <div className="grid grid-cols-1 gap-1">
                 <div className="flex justify-between text-[9px]">
                   <span className="text-[var(--bk-text-secondary)]">Tick Availability</span>
                   <span className={`font-mono font-bold ${backtestOptions.availability?.tick === 'ready' ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'}`}>
                     {String(backtestOptions.availability?.tick ?? "unknown")}
                   </span>
                 </div>
                 <div className="flex justify-between text-[9px]">
                   <span className="text-[var(--bk-text-secondary)]">1min Availability</span>
                   <span className={`font-mono font-bold ${backtestOptions.availability?.['1min'] === 'ready' ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'}`}>
                     {String(backtestOptions.availability?.['1min'] ?? "unknown")}
                   </span>
                 </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 2. 回测历史 Tab 式概览 (简化版) */}
      <Card tone="bento" className="overflow-hidden rounded-[24px] shadow-lg">
        <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] py-3 px-5">
          <div className="flex items-center gap-2">
            <History className="size-4 text-[var(--bk-text-secondary)]/70" />
            <CardTitle className="text-xs font-bold uppercase tracking-widest text-[var(--bk-text-secondary)]">历史队列</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <Table tone="bento">
            <TableHeader className="bg-[var(--bk-surface-muted)]/35">
              <TableRow className="border-[var(--bk-border-soft)] hover:bg-transparent">
                <TableHead className="h-8 px-5 py-0 text-[9px] font-black uppercase text-[var(--bk-text-secondary)]">Time</TableHead>
                <TableHead className="h-8 py-0 text-[9px] font-black uppercase text-[var(--bk-text-secondary)]">Symbol</TableHead>
                <TableHead className="h-8 py-0 pr-5 text-right text-[9px] font-black uppercase text-[var(--bk-text-secondary)]">PnL</TableHead>
              </TableRow>
            </TableHeader>
            <TableBody>
              {backtestItems.length > 0 ? (
                backtestItems.map((item) => {
                  const isSelected = item.id === selectedBacktest?.id;
                  const pnl = getNumber(item.resultSummary?.return);
                  return (
                    <TableRow 
                      key={item.id} 
                      className={`cursor-pointer border-[var(--bk-border-soft)] transition-colors ${isSelected ? 'bg-[var(--bk-surface)] shadow-inner' : 'hover:bg-[var(--bk-surface-overlay)]'}`}
                      onClick={() => setSelectedBacktestId(item.id)}
                    >
                      <TableCell className="px-5 py-2 text-[10px] font-mono text-[var(--bk-text-secondary)]">
                        {formatTime(item.createdAt).split(' ')[1]}
                      </TableCell>
                      <TableCell className="py-2">
                        <span className="text-[10px] font-bold text-[var(--bk-text-primary)]">{String(item.parameters?.symbol ?? "--")}</span>
                      </TableCell>
                      <TableCell className={`py-2 pr-5 text-right font-mono text-[10px] font-black ${(pnl ?? 0) >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]'}`}>
                        {formatPercent(pnl)}
                      </TableCell>
                    </TableRow>
                  );
                })
              ) : (
                <TableRow>
                  <TableCell colSpan={3} className="h-20 text-center text-[10px] italic text-[var(--bk-text-secondary)]">暂无执行记录</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* 3. 详细统计与采样 */}
      {selectedBacktest && (
        <Card tone="bento" className="overflow-hidden rounded-[24px] shadow-xl">
          <CardHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] py-4 px-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <BarChart4 className="size-4 text-[var(--bk-text-primary)]" />
                <CardTitle className="text-base font-bold text-[var(--bk-text-primary)]">回放审计</CardTitle>
              </div>
              <Badge className={`text-[9px] font-black ${selectedBacktest.status === 'COMPLETED' ? 'bg-[var(--bk-text-primary)] text-[var(--bk-canvas)]' : 'bg-[var(--bk-status-danger)] text-[var(--bk-canvas)]'}`}>
                {selectedBacktest.status}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="p-5 space-y-6">
            <div className="grid grid-cols-2 gap-2">
              {[
                { label: "Trade Count", value: String(latestBacktestSummary.executionTradeCount ?? "--"), icon: Clock },
                { label: "Win Rate", value: formatPercent(latestBacktestSummary.executionWinRate), icon: Play },
                { label: "Total PnL", value: formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0), color: (getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0) >= 0 ? 'text-[var(--bk-status-success)]' : 'text-[var(--bk-status-danger)]' },
                { label: "Max DD", value: formatPercent(latestBacktestSummary.maxDrawdown), color: 'text-[var(--bk-status-warning)]' },
              ].map((stat, i) => (
                <div key={i} className="rounded-xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] p-2.5 shadow-sm">
                  <span className="mb-1 block text-[9px] font-black uppercase text-[var(--bk-text-secondary)]">{stat.label}</span>
                  <strong className={`block text-[12px] font-black tracking-tight ${stat.color || 'text-[var(--bk-text-primary)]'}`}>{stat.value}</strong>
                </div>
              ))}
            </div>

            <div className="flex flex-wrap gap-2">
              <Button 
                variant="bento-outline"
                size="sm" 
                className="h-8 flex-1 rounded-xl text-[10px] font-bold"
                disabled={!selectedBacktest?.parameters?.from || !selectedBacktest?.parameters?.to}
                onClick={() => {
                  const from = Date.parse(String(selectedBacktest?.parameters?.from ?? ""));
                  const to = Date.parse(String(selectedBacktest?.parameters?.to ?? ""));
                  if (!Number.isFinite(from) || !Number.isFinite(to)) return;
                  setChartOverrideRange({ from: Math.floor(from / 1000), to: Math.floor(to / 1000), label: "Bktr Window" });
                  setFocusNonce((v) => v + 1);
                }}
              >
                <Maximize2 className="size-3 mr-1.5 opacity-40" />
                复现窗口
              </Button>
              <Button 
                variant="bento-outline"
                size="sm" 
                className="h-8 rounded-xl text-[10px] font-bold"
                onClick={() => window.open(`${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv`)}
              >
                <FileDown className="size-3" />
              </Button>
            </div>

            {/* 采样区 */}
            {latestReplayCompletedSamples.length > 0 || latestReplaySkippedSamples.length > 0 ? (
              <div className="space-y-4">
                <div className="flex items-center gap-2 border-b border-[var(--bk-border)] pb-2">
                  <Database className="size-3 text-[var(--bk-text-secondary)]" />
                  <span className="text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-secondary)]">成交/观测样本点</span>
                </div>
                
                <div className="grid grid-cols-1 gap-3 max-h-[400px] overflow-y-auto pr-1">
                  {latestReplayCompletedSamples.length > 0 && (
                    <div className="space-y-2">
                      <p className="ml-1 text-[9px] font-black uppercase text-[var(--bk-status-success)]">✓ 成交样本</p>
                      {latestReplayCompletedSamples.map((sample, index) => (
                        <SampleCard
                          key={`completed-${index}`}
                          sample={sample}
                          selected={selectedSample?.key === buildSampleKey("completed", index, sample)}
                          onSelect={() => {
                            const range = buildSampleRange(sample);
                            if (!range) return;
                            setSelectedSample({ key: buildSampleKey("completed", index, sample), sample });
                            setChartOverrideRange(range);
                            setSourceFilter("backtest");
                            setEventFilter("all");
                            setFocusNonce((v) => v + 1);
                          }}
                        />
                      ))}
                    </div>
                  )}

                  {latestReplaySkippedSamples.length > 0 && (
                    <div className="space-y-2">
                      <p className="ml-1 text-[9px] font-black uppercase text-[var(--bk-status-danger)]">⚠ 跳过/异常样本</p>
                      {latestReplaySkippedSamples.map((sample, index) => (
                        <SampleCard
                          key={`skipped-${index}`}
                          sample={sample}
                          selected={selectedSample?.key === buildSampleKey("skipped", index, sample)}
                          onSelect={() => {
                            const range = buildSampleRange(sample);
                            if (!range) return;
                            setSelectedSample({ key: buildSampleKey("skipped", index, sample), sample });
                            setChartOverrideRange(range);
                            setSourceFilter("backtest");
                            setEventFilter("all");
                            setFocusNonce((v) => v + 1);
                          }}
                        />
                      ))}
                    </div>
                  )}
                </div>
              </div>
            ) : (
              <div className="rounded-2xl border border-dashed border-[var(--bk-border)] bg-[color:color-mix(in_srgb,var(--bk-accent-soft)_42%,transparent)] p-8 text-center text-[10px] italic text-[var(--bk-text-secondary)]">
                该回测轮次未产生观测样本
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
