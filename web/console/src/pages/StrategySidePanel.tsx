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
    <div className="p-4 space-y-6 animate-in slide-in-from-right duration-500">
      {/* 1. 回测配置面板 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-lg rounded-[24px] overflow-hidden">
        <CardHeader className="bg-white/30 border-b border-[#d8cfba]/50 py-4 px-5">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                <FlaskConical className="size-4 text-[#1f2328]" />
              </div>
              <CardTitle className="text-base font-bold text-[#1f2328]">回测配置</CardTitle>
            </div>
            <Badge variant="outline" className="text-[9px] border-[#d8cfba] font-mono tracking-tight">
              {backtests.length} RUNS
            </Badge>
          </div>
        </CardHeader>
        <CardContent className="p-5 space-y-4">
          <div className="grid grid-cols-1 gap-4">
            <div className="space-y-1.5">
              <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase tracking-wide">Strategy</Label>
              <Select 
                value={backtestForm.strategyVersionId}
                onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, strategyVersionId: val }))}
              >
                <SelectTrigger className="h-9 rounded-xl border-[#d8cfba] bg-white/50 text-[12px] font-medium">
                  <SelectValue placeholder="选择策略版本" />
                </SelectTrigger>
                <SelectContent className="bg-white border-[#d8cfba] rounded-xl shadow-xl">
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
                <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase">Timeframe</Label>
                <Select 
                  value={backtestForm.signalTimeframe}
                  onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, signalTimeframe: val }))}
                >
                  <SelectTrigger className="h-9 rounded-xl border-[#d8cfba] bg-white/50 text-[12px]">
                    <SelectValue placeholder="周期" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                    {(backtestOptions?.signalTimeframes ?? ["4h", "1d"]).map((item) => (
                      <SelectItem key={item} value={item}>{item}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase">Source</Label>
                <Select 
                  value={backtestForm.executionDataSource}
                  onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, executionDataSource: val }))}
                >
                  <SelectTrigger className="h-9 rounded-xl border-[#d8cfba] bg-white/50 text-[12px]">
                    <SelectValue placeholder="数据源" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                    {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => (
                      <SelectItem key={item} value={item}>{item}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-1.5">
              <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase">Symbol</Label>
              <Input 
                value={backtestForm.symbol}
                onChange={(e) => setBacktestForm(curr => ({ ...curr, symbol: e.target.value.toUpperCase() }))}
                placeholder="例如：BTCUSDT"
                className={`h-9 rounded-xl text-[12px] font-mono font-bold border-[#d8cfba] bg-white/50 focus:bg-white ${!selectedSymbolAvailable ? 'border-rose-300 ring-rose-100 ring-2' : ''}`}
              />
            </div>

            <div className="grid grid-cols-2 gap-3">
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase">From</Label>
                <Input 
                  value={backtestForm.from}
                  onChange={(e) => setBacktestForm(curr => ({ ...curr, from: e.target.value }))}
                  placeholder="2024-01-01..."
                  className="h-9 rounded-xl text-[10px] font-mono border-[#d8cfba] bg-white/50"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-0.5 uppercase">To</Label>
                <Input 
                  value={backtestForm.to}
                  onChange={(e) => setBacktestForm(curr => ({ ...curr, to: e.target.value }))}
                  placeholder="2024-01-31..."
                  className="h-9 rounded-xl text-[10px] font-mono border-[#d8cfba] bg-white/50"
                />
              </div>
            </div>
          </div>

          <Button 
            className="w-full h-11 rounded-2xl bg-[#1f2328] hover:bg-[#2f353c] text-white font-black text-sm transition-all shadow-md active:scale-95 disabled:opacity-50"
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
            <div className="p-3 rounded-xl bg-[#fff8ea] border border-[#d8cfba]/40 space-y-2">
              <div className="flex items-center gap-2 text-[10px] text-[#687177] font-bold">
                 <Database className="size-3" />
                 <span>数据就绪检查</span>
              </div>
              <div className="grid grid-cols-1 gap-1">
                 <div className="flex justify-between text-[9px]">
                   <span className="text-[#687177]">Tick Availability</span>
                   <span className={`font-mono font-bold ${backtestOptions.availability?.tick === 'ready' ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
                     {String(backtestOptions.availability?.tick ?? "unknown")}
                   </span>
                 </div>
                 <div className="flex justify-between text-[9px]">
                   <span className="text-[#687177]">1min Availability</span>
                   <span className={`font-mono font-bold ${backtestOptions.availability?.['1min'] === 'ready' ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
                     {String(backtestOptions.availability?.['1min'] ?? "unknown")}
                   </span>
                 </div>
              </div>
            </div>
          )}
        </CardContent>
      </Card>

      {/* 2. 回测历史 Tab 式概览 (简化版) */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-lg rounded-[24px] overflow-hidden">
        <CardHeader className="bg-white/30 border-b border-[#d8cfba]/50 py-3 px-5">
          <div className="flex items-center gap-2">
            <History className="size-4 text-[#1f2328]/40" />
            <CardTitle className="text-xs font-bold text-[#1f2328]/60 uppercase tracking-widest">历史队列</CardTitle>
          </div>
        </CardHeader>
        <CardContent className="p-0">
          <Table>
            <TableHeader className="bg-[#ebe5d5]/20">
              <TableRow className="border-[#d8cfba]/40 hover:bg-transparent">
                <TableHead className="h-8 py-0 px-5 text-[9px] uppercase font-black text-[#687177]">Time</TableHead>
                <TableHead className="h-8 py-0 text-[9px] uppercase font-black text-[#687177]">Symbol</TableHead>
                <TableHead className="h-8 py-0 text-right pr-5 text-[9px] uppercase font-black text-[#687177]">PnL</TableHead>
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
                      className={`cursor-pointer transition-colors border-[#d8cfba]/20 ${isSelected ? 'bg-white shadow-inner' : 'hover:bg-white/50'}`}
                      onClick={() => setSelectedBacktestId(item.id)}
                    >
                      <TableCell className="px-5 py-2 text-[10px] font-mono text-[#687177]">
                        {formatTime(item.createdAt).split(' ')[1]}
                      </TableCell>
                      <TableCell className="py-2">
                        <span className="text-[10px] font-bold text-[#1f2328]">{String(item.parameters?.symbol ?? "--")}</span>
                      </TableCell>
                      <TableCell className={`py-2 pr-5 text-right font-mono text-[10px] font-black ${(pnl ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
                        {formatPercent(pnl)}
                      </TableCell>
                    </TableRow>
                  );
                })
              ) : (
                <TableRow>
                  <TableCell colSpan={3} className="h-20 text-center text-[10px] italic text-[#687177]">暂无执行记录</TableCell>
                </TableRow>
              )}
            </TableBody>
          </Table>
        </CardContent>
      </Card>

      {/* 3. 详细统计与采样 */}
      {selectedBacktest && (
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[24px] overflow-hidden">
          <CardHeader className="bg-white/30 border-b border-[#d8cfba]/50 py-4 px-5">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-2">
                <BarChart4 className="size-4 text-[#1f2328]" />
                <CardTitle className="text-base font-bold text-[#1f2328]">回放审计</CardTitle>
              </div>
              <Badge className={`text-[9px] font-black ${selectedBacktest.status === 'COMPLETED' ? 'bg-[#1f2328]' : 'bg-rose-500'}`}>
                {selectedBacktest.status}
              </Badge>
            </div>
          </CardHeader>
          <CardContent className="p-5 space-y-6">
            <div className="grid grid-cols-2 gap-2">
              {[
                { label: "Trade Count", value: String(latestBacktestSummary.executionTradeCount ?? "--"), icon: Clock },
                { label: "Win Rate", value: formatPercent(latestBacktestSummary.executionWinRate), icon: Play },
                { label: "Total PnL", value: formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0), color: (getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600' },
                { label: "Max DD", value: formatPercent(latestBacktestSummary.maxDrawdown), color: 'text-amber-700' },
              ].map((stat, i) => (
                <div key={i} className="bg-white/60 border border-[#d8cfba]/50 rounded-xl p-2.5 shadow-sm">
                  <span className="block text-[9px] font-black text-[#687177] uppercase mb-1">{stat.label}</span>
                  <strong className={`text-[12px] font-black block tracking-tight ${stat.color || 'text-[#1f2328]'}`}>{stat.value}</strong>
                </div>
              ))}
            </div>

            <div className="flex flex-wrap gap-2">
              <Button 
                variant="outline" 
                size="sm" 
                className="flex-1 h-8 text-[10px] font-bold border-[#d8cfba] bg-white hover:bg-[#fff8ea] rounded-xl"
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
                variant="outline" 
                size="sm" 
                className="h-8 text-[10px] font-bold border-[#d8cfba] bg-white hover:bg-[#fff8ea] rounded-xl"
                onClick={() => window.open(`${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv`)}
              >
                <FileDown className="size-3" />
              </Button>
            </div>

            {/* 采样区 */}
            {latestReplayCompletedSamples.length > 0 || latestReplaySkippedSamples.length > 0 ? (
              <div className="space-y-4">
                <div className="flex items-center gap-2 border-b border-[#d8cfba] pb-2">
                  <Database className="size-3 text-[#687177]" />
                  <span className="text-[10px] font-black text-[#687177] uppercase tracking-widest">成交/观测样本点</span>
                </div>
                
                <div className="grid grid-cols-1 gap-3 max-h-[400px] overflow-y-auto pr-1">
                  {latestReplayCompletedSamples.length > 0 && (
                    <div className="space-y-2">
                      <p className="text-[9px] font-black text-[#0e6d60] ml-1 uppercase">✓ 成交样本</p>
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
                      <p className="text-[9px] font-black text-rose-600 ml-1 uppercase">⚠ 跳过/异常样本</p>
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
              <div className="p-8 text-center text-[10px] text-[#687177] italic bg-[#fff8ea]/40 rounded-2xl border border-dashed border-[#d8cfba]">
                该回测轮次未产生观测样本
              </div>
            )}
          </CardContent>
        </Card>
      )}
    </div>
  );
}
