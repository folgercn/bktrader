import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { formatTime, formatPercent, formatSigned, formatMaybeNumber } from '../utils/format';
import { strategyLabel, getRecord, getNumber, buildSampleKey, buildSampleRange } from '../utils/derivation';
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Textarea } from '../components/ui/textarea';
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
  Tooltip, 
  TooltipContent, 
  TooltipProvider, 
  TooltipTrigger 
} from '../components/ui/tooltip';
import { Separator } from '../components/ui/separator';
import { SampleCard } from '../components/ui/SampleCard';
import { API_BASE } from '../utils/api';
import { ExecutionTrade, ReplaySample } from '../types/domain';
import { 
  HelpCircle, 
  Plus, 
  Save, 
  RotateCcw, 
  Layout, 
  FileJson, 
  List, 
  Info, 
  FlaskConical, 
  Play, 
  History, 
  BarChart4, 
  Maximize2, 
  FileDown, 
  Database,
  Clock,
  ArrowRight,
  AlertTriangle
} from 'lucide-react';

interface StrategyStageProps {
  createStrategy: () => void;
  saveStrategyParameters: () => void;
  createBacktestRun: () => Promise<void>;
}

export function StrategyStage({ createStrategy, saveStrategyParameters, createBacktestRun }: StrategyStageProps) {
  // Strategy States
  const strategies = useTradingStore(s => s.strategies);
  const signalRuntimeAdapters = useTradingStore(s => s.signalRuntimeAdapters);
  const strategyCreateForm = useUIStore(s => s.strategyCreateForm);
  const setStrategyCreateForm = useUIStore(s => s.setStrategyCreateForm);
  const strategyCreateAction = useUIStore(s => s.strategyCreateAction);
  const strategyEditorForm = useUIStore(s => s.strategyEditorForm);
  const setStrategyEditorForm = useUIStore(s => s.setStrategyEditorForm);
  const strategySaveAction = useUIStore(s => s.strategySaveAction);
  const selectedStrategyId = useTradingStore(s => s.selectedStrategyId);
  const setSelectedStrategyId = useTradingStore(s => s.setSelectedStrategyId);

  // Backtest States (Migrated from SidePanel)
  const backtestForm = useUIStore(s => s.backtestForm);
  const setBacktestForm = useUIStore(s => s.setBacktestForm);
  const backtestAction = useUIStore(s => s.backtestAction);
  const backtestOptions = useTradingStore(s => s.backtestOptions);
  const backtests = useTradingStore(s => s.backtests);
  const selectedBacktestId = useUIStore(s => s.selectedBacktestId);
  const setSelectedBacktestId = useUIStore(s => s.setSelectedBacktestId);
  const setChartOverrideRange = useUIStore(s => s.setChartOverrideRange);
  const setFocusNonce = useUIStore(s => s.setFocusNonce);
  const selectedSample = useUIStore(s => s.selectedSample);
  const setSelectedSample = useUIStore(s => s.setSelectedSample);
  const setSourceFilter = useUIStore(s => s.setSourceFilter);
  const setEventFilter = useUIStore(s => s.setEventFilter);

  // Derived states
  const selectedStrategy =
    strategies.find((item) => item.id === (selectedStrategyId || strategyEditorForm.strategyId)) ?? strategies[0] ?? null;
  const selectedStrategyVersion = selectedStrategy?.currentVersion ?? null;
  const selectedStrategyParameters = getRecord(selectedStrategyVersion?.parameters);

  const selectedBacktest =
    backtests.find((item) => item.id === selectedBacktestId) ??
    (backtests.length > 0 ? backtests[backtests.length - 1] : null);
  const latestBacktestSummary = (selectedBacktest?.resultSummary ?? {}) as Record<string, unknown>;
  
  const latestReplaySkippedSamples = Array.isArray(latestBacktestSummary.replayLedgerSkippedSamples)
    ? (latestBacktestSummary.replayLedgerSkippedSamples as ReplaySample[])
    : [];
  const latestReplayCompletedSamples = Array.isArray(latestBacktestSummary.replayLedgerCompletedSamples)
    ? (latestBacktestSummary.replayLedgerCompletedSamples as ReplaySample[])
    : [];

  const selectedExecutionSymbols = backtestOptions?.supportedSymbols?.[backtestForm.executionDataSource] ?? [];
  const selectedSymbolAvailable =
    selectedExecutionSymbols.length === 0 || selectedExecutionSymbols.includes(backtestForm.symbol.trim().toUpperCase());

  return (
    <div className="absolute inset-0 overflow-y-auto p-8 space-y-8 bg-[#f3f0e7]">
      {/* 顶部总控 - 参照 AccountStage 范式 */}
      <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-sm rounded-[24px] overflow-hidden">
        <div className="py-3 px-6 flex flex-col md:flex-row items-center justify-between gap-4">
           <div className="flex items-center gap-6 overflow-hidden">
              <div className="shrink-0">
                <p className="text-[#0e6d60] text-[9px] font-black uppercase tracking-widest font-mono mb-0.5">Strategy Control Center</p>
                <h2 className="text-lg font-black text-[#1f2328] tracking-tight whitespace-nowrap">策略库与开发实验室</h2>
              </div>
              
              <Separator orientation="vertical" className="h-8 bg-[#d8cfba]/40 hidden lg:block" />
              
              <div className="hidden lg:flex items-center gap-4 h-8 px-3 rounded-xl bg-[#fffbf2]/50 border border-[#d8cfba]/50">
                <div className="flex items-center gap-1.5">
                  <span className="text-[9px] font-black text-[#687177] uppercase opacity-40">Active:</span>
                  <span className="text-[10px] font-black text-[#1f2328]">{strategies.length} 策略</span>
                </div>
                <Separator orientation="vertical" className="h-3 bg-[#d8cfba]/40" />
                <div className="flex items-center gap-1.5">
                  <span className="text-[9px] font-black text-[#687177] uppercase opacity-40">Engines:</span>
                  <span className="text-[10px] font-black text-[#1f2328]">{signalRuntimeAdapters.length} 适配器</span>
                </div>
              </div>
           </div>
           
           <div className="flex items-center gap-2">
              <div className="flex items-center p-1 rounded-xl bg-white/40 border border-[#d8cfba]/20">
                <Button 
                   variant="ghost" 
                   size="sm" 
                   className="h-8 px-4 text-[10px] font-black text-[#1f2328] rounded-lg hover:bg-white shadow-none"
                   onClick={() => document.getElementById('new-strategy-section')?.scrollIntoView({ behavior: 'smooth' })}
                >
                  创建新策略
                </Button>
                <Separator orientation="vertical" className="h-4 bg-[#d8cfba]/30 mx-1" />
                <Button 
                   variant="ghost" 
                   size="sm" 
                   className="h-8 px-4 text-[10px] font-black text-[#1f2328] rounded-lg hover:bg-white shadow-none"
                   onClick={() => document.getElementById('backtest-lab-section')?.scrollIntoView({ behavior: 'smooth' })}
                >
                  进入实验室
                </Button>
              </div>
           </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* 左侧：策略字典 (Archive) */}
        <Card className="lg:col-span-8 border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div>
                <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Strategy Portfolio</p>
                <CardTitle className="text-lg font-black text-[#1f2328]">策略目录</CardTitle>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <Table>
                <TableHeader className="bg-[#f8f6f0] border-y border-[#d8cfba]">
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="px-6 h-10 text-[10px] uppercase font-black text-[#687177]">策略名称 / ID</TableHead>
                    <TableHead className="h-10 text-[10px] uppercase font-black text-[#687177]">版本</TableHead>
                    <TableHead className="h-10 text-[10px] uppercase font-black text-[#687177]">信号周期</TableHead>
                    <TableHead className="h-10 text-[10px] uppercase font-black text-[#687177]">运行引擎</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="divide-y divide-[#d8cfba]/50">
                  {strategies.length > 0 ? (
                    strategies.map((strategy) => {
                      const params = getRecord(strategy.currentVersion?.parameters);
                      const isSelected = strategy.id === selectedStrategy?.id;
                      return (
                        <TableRow 
                          key={strategy.id} 
                          className={`group cursor-pointer transition-all duration-200 ${isSelected ? 'bg-white shadow-inner relative z-10' : 'bg-[#fff8ea] hover:bg-white'}`}
                          onClick={() => setSelectedStrategyId(strategy.id)}
                        >
                          <TableCell className="px-6 py-4">
                            <div className="flex items-center gap-3">
                               <div className={`w-1 h-8 rounded-full transition-all ${isSelected ? 'bg-[#0e6d60]' : 'bg-transparent group-hover:bg-[#d8cfba]'}`} />
                               <div className="flex flex-col min-w-0">
                                 <span className="font-black text-[#1f2328] text-sm tabular-nums tracking-tight truncate">{strategy.name}</span>
                                 <span className="text-[9px] text-[#687177] font-mono opacity-60 uppercase truncate">{strategy.id}</span>
                               </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="outline" className={`text-[10px] h-4.5 border-[#d8cfba] font-bold ${isSelected ? 'bg-[#1f2328] text-white border-transparent' : 'bg-white text-[#687177]'}`}>
                              v{strategy.currentVersion?.version ?? "1"}
                            </Badge>
                          </TableCell>
                          <TableCell className="text-xs font-black text-[#1f2328]/70 font-mono">
                            {String(params.signalTimeframe ?? strategy.currentVersion?.signalTimeframe ?? "--")}
                          </TableCell>
                          <TableCell>
                             <div className="flex items-center gap-1.5 px-2 py-0.5 rounded-md bg-white border border-[#d8cfba]/50 w-fit">
                               <div className="size-1 rounded-full bg-[#0e6d60]" />
                               <span className="text-[10px] font-black text-[#1f2328] uppercase">{String(params.strategyEngine ?? "bk-default")}</span>
                             </div>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  ) : (
                    <TableRow>
                      <TableCell colSpan={4} className="h-40 text-center text-[11px] font-medium text-[#687177] italic">暂无策略数据</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
 
        {/* 右侧：版本摘要 (Details) - 参照 AccountStage 细节卡片 */}
        <Card className="lg:col-span-4 border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden flex flex-col">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div>
                <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Summary</p>
                <CardTitle className="text-lg font-black text-[#1f2328]">当前版本摘要</CardTitle>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-6 space-y-6 flex-1">
            {selectedStrategy ? (
              <>
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3">
                     <div className="p-3.5 bg-white border border-[#d8cfba]/60 rounded-2xl shadow-sm">
                        <span className="block text-[8px] font-black text-[#687177] uppercase tracking-widest mb-1.5 opacity-60">Signal Source</span>
                        <p className="text-xs font-black text-[#1f2328] truncate">{String(selectedStrategyParameters.executionDataSource ?? "--")}</p>
                     </div>
                     <div className="p-3.5 bg-white border border-[#d8cfba]/60 rounded-2xl shadow-sm">
                        <span className="block text-[8px] font-black text-[#687177] uppercase tracking-widest mb-1.5 opacity-60">Engine Type</span>
                        <p className="text-xs font-black text-[#1f2328] uppercase">{String(selectedStrategyParameters.strategyEngine ?? "bk-default")}</p>
                     </div>
                  </div>
 
                  <div className="bg-[#fff8ea] border border-[#d8cfba] p-4 rounded-2xl shadow-inner space-y-3">
                     <div className="flex items-center gap-2 pb-2 border-b border-[#d8cfba]/40">
                       <Info size={14} className="text-[#0e6d60]" />
                       <span className="text-[9px] font-black text-[#0e6d60] uppercase tracking-widest">Metadata</span>
                     </div>
                     <div className="grid grid-cols-1 gap-2.5">
                       <div className="flex justify-between items-center text-[11px]">
                         <span className="text-[#687177] font-medium">创建时间</span>
                         <span className="font-mono font-bold text-[#1f2328]">{formatTime(selectedStrategy.createdAt).split(' ')[0]}</span>
                       </div>
                       <div className="flex justify-between items-center text-[11px]">
                         <span className="text-[#687177] font-medium">最后编译版本</span>
                         <Badge variant="outline" className="h-4 text-[9px] font-black border-[#d8cfba]">v{selectedStrategyVersion?.version ?? "1"}</Badge>
                       </div>
                     </div>
                  </div>
                </div>
 
                <div className="flex-1 min-h-[80px] p-4 bg-white/40 border border-[#d8cfba] rounded-2xl border-dashed">
                   <p className="text-[10px] font-black text-[#687177] uppercase tracking-widest mb-2 opacity-60">Description</p>
                   <p className="text-xs text-[#1f2328] leading-relaxed font-medium">
                     {selectedStrategy.description || "暂无描述内容"}
                   </p>
                </div>
              </>
            ) : (
              <div className="flex-1 flex flex-col items-center justify-center space-y-3 text-[#d8cfba]">
                <RotateCcw className="size-10 animate-spin-slow opacity-20" />
                <p className="text-xs font-black uppercase tracking-widest">请选择策略</p>
              </div>
            )}
          </CardContent>
          {selectedStrategy && (
            <div className="p-4 bg-amber-50 mx-6 mb-6 rounded-xl border border-amber-200">
               <div className="flex gap-2">
                 <AlertTriangle size={12} className="text-amber-600 shrink-0 mt-0.5" />
                 <p className="text-[9px] text-amber-800 leading-normal font-medium italic">
                    警告：热更新模式已启用。编辑参数将立即波及所有运行中的反射引擎。
                 </p>
               </div>
            </div>
          )}
        </Card>
      </div>

      <div id="new-strategy-section" className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* 开发区左侧：建立新策略 - 风格对齐 AccountStage 的 Modal 样式 */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                <Plus className="size-4 text-[#1f2328]" />
              </div>
              <CardTitle className="text-lg font-black text-[#1f2328]">建立新策略</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-6 space-y-5">
            <div className="space-y-4">
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-1 uppercase tracking-wider">Strategy Name</Label>
                <Input 
                  value={strategyCreateForm.name}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="例如：BK-TREND-01"
                  className="h-10 rounded-xl border-[#d8cfba] bg-white text-xs font-bold focus:ring-2 focus:ring-[#0e6d60]/10 transition-all shadow-sm"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-1 uppercase tracking-wider">Internal Description</Label>
                <Input 
                  value={strategyCreateForm.description}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, description: e.target.value }))}
                  placeholder="描述逻辑边界或执行目的"
                  className="h-10 rounded-xl border-[#d8cfba] bg-white text-xs font-medium focus:ring-2 focus:ring-[#0e6d60]/10 transition-all shadow-sm"
                />
              </div>
            </div>
            <Button 
              className="w-full h-11 bg-[#1f2328] hover:bg-[#2f353c] text-white font-black text-xs rounded-xl shadow-md transition-all active:scale-95 disabled:opacity-50"
              disabled={strategyCreateAction || !strategyCreateForm.name.trim()}
              onClick={createStrategy}
            >
              {strategyCreateAction ? "正在注入逻辑..." : "确认并提交策略库"}
            </Button>
          </CardContent>
        </Card>
 
        {/* 开发区右侧：参数反射编辑器 (Refl Editor) */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden flex flex-col">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-1.5 bg-[#ebe5d5] rounded-lg">
                <FileJson className="size-4 text-[#1f2328]" />
              </div>
              <CardTitle className="text-lg font-black text-[#1f2328]">参数反射编辑器</CardTitle>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <Button variant="ghost" size="icon" className="h-8 w-8 rounded-xl hover:bg-white/50">
                    <HelpCircle className="size-4 text-[#687177]" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent className="bg-[#1f2328] text-white p-3 text-[10px] rounded-xl border-0 shadow-2xl w-64">
                   热更新模式下，保存参数将通过反射引擎立即更新至对应的运行实例，建议在生产运行前进行参数校验。
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </CardHeader>
          <CardContent className="p-6 space-y-5 flex-1 flex flex-col">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-1 uppercase tracking-wider">Engine</Label>
                <Select 
                  value={strategyEditorForm.strategyEngine}
                  onValueChange={(val: any) => setStrategyEditorForm(prev => ({ ...prev, strategyEngine: val }))}
                >
                  <SelectTrigger className="h-10 rounded-xl border-[#d8cfba] bg-white text-xs font-black">
                    <SelectValue placeholder="选择引擎" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl shadow-2xl">
                    {[...new Set(["bk-default", ...strategies.map((item) => String(getRecord(item.currentVersion?.parameters).strategyEngine || "bk-default"))])].map((engineKey) => (
                      <SelectItem key={engineKey} value={engineKey}>{engineKey}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="text-[11px] font-black text-[#687177] ml-1 uppercase tracking-wider">Timeframe</Label>
                <Select 
                  value={strategyEditorForm.signalTimeframe}
                  onValueChange={(val: any) => setStrategyEditorForm(prev => ({ ...prev, signalTimeframe: val }))}
                >
                  <SelectTrigger className="h-10 rounded-xl border-[#d8cfba] bg-white text-xs font-black">
                    <SelectValue placeholder="周期" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl shadow-2xl">
                    <SelectItem value="5m">5m</SelectItem>
                    <SelectItem value="1h">1h</SelectItem>
                    <SelectItem value="4h">4h</SelectItem>
                    <SelectItem value="1d">1d</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
 
            <div className="space-y-2 flex-1 flex flex-col">
              <div className="flex items-center justify-between">
                 <Label className="text-[11px] font-black text-[#687177] ml-1 uppercase tracking-wider">Parameters Schema</Label>
                 <Button 
                   variant="ghost" 
                   size="sm" 
                   className="h-6 px-2 text-[9px] font-black text-[#0e6d60]"
                   onClick={() => {
                     try { setStrategyEditorForm(prev => ({ ...prev, parametersJson: JSON.stringify(JSON.parse(prev.parametersJson), null, 2) })) } catch (e) {}
                   }}
                 >
                   格式化 JSON
                 </Button>
              </div>
              <div className="relative flex-1 group">
                 <div className="absolute left-0 top-0 bottom-0 w-8 bg-[#f3f0e7]/30 border-r border-[#d8cfba]/40 rounded-l-2xl flex flex-col items-center pt-3 pointer-events-none opacity-40">
                    {[1,2,3,4,5,6,7].map(n => <span key={n} className="text-[8px] font-mono leading-relaxed">{n}</span>)}
                 </div>
                 <Textarea 
                   value={strategyEditorForm.parametersJson}
                   onChange={(e) => setStrategyEditorForm(prev => ({ ...prev, parametersJson: e.target.value }))}
                   className="w-full h-full min-h-[160px] pl-10 pr-4 py-3 font-mono text-[11px] rounded-2xl border-[#d8cfba] bg-[#fffbf2]/40 focus:bg-white focus:ring-2 focus:ring-[#d8cfba]/40 transition-all shadow-inner leading-relaxed resize-none"
                   placeholder='{"key": "value"}'
                 />
              </div>
            </div>
 
            <div className="flex gap-3 pt-2">
               <Button 
                 className="flex-1 h-11 bg-[#0e6d60] hover:bg-[#0a5a4f] text-white font-black text-xs rounded-xl shadow-md transition-transform active:scale-95"
                 disabled={strategySaveAction || !strategyEditorForm.strategyId}
                 onClick={saveStrategyParameters}
               >
                 <Save size={14} className="mr-2" />
                 {strategySaveAction ? "正在提交同步..." : "保存反射参数"}
               </Button>
               <Button 
                 variant="outline" 
                 size="icon" 
                 className="h-11 w-11 rounded-xl border-[#d8cfba] bg-white hover:bg-[#fff8ea]"
                 onClick={() => {
                    if (!selectedStrategy) return;
                    const params = getRecord(selectedStrategy.currentVersion?.parameters);
                    setStrategyEditorForm({
                      strategyId: selectedStrategy.id,
                      strategyEngine: String(params.strategyEngine ?? "bk-default"),
                      signalTimeframe: String(params.signalTimeframe ?? selectedStrategy.currentVersion?.signalTimeframe ?? "1d"),
                      executionDataSource: String(params.executionDataSource ?? "tick"),
                      parametersJson: JSON.stringify(params, null, 2),
                    });
                 }}
               >
                 <RotateCcw size={16} />
               </Button>
            </div>
          </CardContent>
        </Card>
      </div>
 
      {/* 底部：回测实验室 (Backtest Research Lab) - 全新整合版 */}
      <Card id="backtest-lab-section" className="border-[#d8cfba] bg-[var(--panel)] shadow-[var(--shadow)] rounded-[24px] overflow-hidden">
        <CardHeader className="pb-6">
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="p-2 bg-[#ebe5d5] rounded-xl">
                 <FlaskConical size={20} className="text-[#1f2328]" />
              </div>
              <div>
                <p className="text-[#0e6d60] text-[10px] font-bold uppercase tracking-widest font-mono">Research Lab</p>
                <CardTitle className="text-xl font-black text-[#1f2328]">回测控制台与回放审计</CardTitle>
              </div>
            </div>
            <div className="flex items-center gap-4">
               <Badge variant="outline" className="h-6 border-[#d8cfba] bg-white text-[#1f2328] font-black tabular-nums">
                 {backtests.length} RUNS RECORDED
               </Badge>
            </div>
          </div>
        </CardHeader>
        <CardContent className="p-0 border-t border-[#d8cfba]/40 bg-[#fffbf2]/30">
          <div className="grid grid-cols-1 lg:grid-cols-12 min-h-[520px]">
            {/* 左侧：参数配置 (Lab Config) */}
            <div className="lg:col-span-3 p-6 space-y-6 border-r border-[#d8cfba]/40 bg-white/40">
               <div className="space-y-4">
                  <div className="space-y-1.5">
                    <Label className="text-[10px] font-black text-[#687177] uppercase tracking-widest ml-1">Symbol</Label>
                    <Input 
                       value={backtestForm.symbol}
                       onChange={(e) => setBacktestForm(curr => ({ ...curr, symbol: e.target.value.toUpperCase() }))}
                       placeholder="BTCUSDT"
                       className={`h-10 rounded-xl font-mono font-black text-xs border-[#d8cfba] ${!selectedSymbolAvailable ? 'ring-2 ring-rose-500/20 border-rose-300' : ''}`}
                    />
                  </div>
                  <div className="grid grid-cols-2 gap-3">
                    <div className="space-y-1.5">
                      <Label className="text-[10px] font-black text-[#687177] uppercase ml-1">Period</Label>
                      <Select value={backtestForm.signalTimeframe} onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, signalTimeframe: val }))}>
                        <SelectTrigger className="h-10 rounded-xl border-[#d8cfba] text-xs font-black"><SelectValue /></SelectTrigger>
                        <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                          {(backtestOptions?.signalTimeframes ?? ["5m", "4h", "1d"]).map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-[10px] font-black text-[#687177] uppercase ml-1">Source</Label>
                      <Select value={backtestForm.executionDataSource} onValueChange={(val: any) => setBacktestForm(curr => ({ ...curr, executionDataSource: val }))}>
                        <SelectTrigger className="h-10 rounded-xl border-[#d8cfba] text-xs font-black"><SelectValue /></SelectTrigger>
                        <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                          {(backtestOptions?.executionDataSources ?? ["tick", "1min"]).map((item) => <SelectItem key={item} value={item}>{item}</SelectItem>)}
                        </SelectContent>
                      </Select>
                    </div>
                  </div>
                  <div className="grid grid-cols-1 gap-3">
                    <div className="space-y-1.5">
                      <Label className="text-[10px] font-black text-[#687177] uppercase ml-1">Range From</Label>
                      <Input value={backtestForm.from} onChange={(e) => setBacktestForm(curr => ({ ...curr, from: e.target.value }))} className="h-10 rounded-xl font-mono text-xs border-[#d8cfba]" />
                    </div>
                    <div className="space-y-1.5">
                      <Label className="text-[10px] font-black text-[#687177] uppercase ml-1">Range To</Label>
                      <Input value={backtestForm.to} onChange={(e) => setBacktestForm(curr => ({ ...curr, to: e.target.value }))} className="h-10 rounded-xl font-mono text-xs border-[#d8cfba]" />
                    </div>
                  </div>
               </div>
 
               <Button 
                  className="w-full h-12 bg-[#1f2328] hover:bg-[#0e6d60] text-white font-black text-xs rounded-2xl shadow-lg transition-all active:scale-95 disabled:opacity-50"
                  disabled={backtestAction || !selectedSymbolAvailable || !selectedStrategy}
                  onClick={() => {
                    const versionId = selectedStrategy?.currentVersion?.id ?? "";
                    if (!versionId) return;
                    setBacktestForm(curr => ({ ...curr, strategyVersionId: versionId }));
                    createBacktestRun();
                  }}
               >
                 <Play size={14} className="mr-2" />
                 {backtestAction ? "正在计算路径..." : "启动压力测试"}
               </Button>
 
               {backtestOptions && (
                 <div className="p-4 rounded-2xl bg-[#f3f0e7]/50 border border-[#d8cfba]/40 space-y-3">
                    <div className="flex items-center gap-2 text-[10px] text-[#687177] font-black uppercase opacity-60">
                       <Database size={12} /> <span>数据就绪状态</span>
                    </div>
                    <div className="space-y-2">
                       {['tick', '1min'].map(type => (
                         <div key={type} className="flex justify-between items-center text-[11px]">
                           <span className="text-[#687177] font-medium">{type} Support</span>
                           <Badge variant="outline" className={`h-4 text-[9px] font-black border-transparent uppercase ${backtestOptions.availability?.[type as 'tick'|'1min'] === 'ready' ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
                             {String(backtestOptions.availability?.[type as 'tick'|'1min'] ?? "unknown")}
                           </Badge>
                         </div>
                       ))}
                    </div>
                 </div>
               )}
            </div>
 
            {/* 中间：队列表格 (Lab History) */}
            <div className="lg:col-span-5 border-r border-[#d8cfba]/40 flex flex-col min-w-0 bg-white/20">
               <div className="p-4 border-b border-[#d8cfba]/40 bg-[#fff8ea]/50 flex items-center justify-between shrink-0">
                  <div className="flex items-center gap-2">
                    <History size={16} className="text-[#687177]" />
                    <span className="text-[11px] font-black text-[#1f2328] uppercase tracking-widest">历史任务队列</span>
                  </div>
               </div>
               <div className="flex-1 overflow-y-auto">
                  <Table>
                    <TableHeader className="bg-white/50 sticky top-0 z-10">
                      <TableRow className="hover:bg-transparent border-[#d8cfba]/40">
                        <TableHead className="px-5 h-10 text-[9px] uppercase font-black text-[#687177]">执行时间</TableHead>
                        <TableHead className="h-10 text-[9px] uppercase font-black text-[#687177]">币种</TableHead>
                        <TableHead className="h-10 text-[9px] uppercase font-black text-[#687177]">PnL (%)</TableHead>
                        <TableHead className="h-10 text-[9px] uppercase font-black text-[#687177] pr-5 text-right">状态</TableHead>
                      </TableRow>
                    </TableHeader>
                    <TableBody>
                      {backtests.length > 0 ? (
                        backtests.slice().reverse().map((item) => {
                          const isSelected = item.id === selectedBacktest?.id;
                          const pnl = getNumber(item.resultSummary?.return);
                          return (
                            <TableRow 
                              key={item.id} 
                              className={`group cursor-pointer transition-all ${isSelected ? 'bg-white shadow-inner' : 'hover:bg-white/50'}`}
                              onClick={() => setSelectedBacktestId(item.id)}
                            >
                              <TableCell className="px-5 py-3 text-[10px] font-mono text-[#687177]">
                                {formatTime(item.createdAt).split(' ')[1]}
                              </TableCell>
                              <TableCell className="py-3 font-black text-[11px] text-[#1f2328]">
                                {String(item.parameters?.symbol ?? "--")}
                              </TableCell>
                              <TableCell className={`py-3 font-mono font-black text-xs ${(pnl ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600'}`}>
                                {pnl !== undefined ? (pnl > 0 ? "+" : "") + formatPercent(pnl) : "--"}
                              </TableCell>
                              <TableCell className="py-3 pr-5 text-right">
                                <div className={`size-2 rounded-full inline-block ${item.status === 'COMPLETED' ? 'bg-[#0e6d60]' : 'bg-amber-500'}`} title={item.status} />
                              </TableCell>
                            </TableRow>
                          );
                        })
                      ) : (
                        <TableRow>
                          <TableCell colSpan={4} className="h-64 text-center text-xs text-[#687177] italic opacity-40">队列为空，等待第一次实验</TableCell>
                        </TableRow>
                      )}
                    </TableBody>
                  </Table>
               </div>
            </div>
 
            {/* 右侧：分析审计 (Auditor) */}
            <div className="lg:col-span-4 p-6 flex flex-col bg-white/10 overflow-y-auto">
               {selectedBacktest ? (
                 <div className="space-y-8">
                    <div className="flex items-center justify-between">
                       <div className="flex items-center gap-2">
                          <BarChart4 size={18} className="text-[#1f2328]" />
                          <h4 className="text-sm font-black text-[#1f2328] uppercase tracking-wider">执行回放结果</h4>
                       </div>
                       <Badge className={`text-[9px] font-black h-5 ${selectedBacktest.status === 'COMPLETED' ? 'bg-[#1f2328]' : 'bg-rose-500'}`}>
                         {selectedBacktest.status}
                       </Badge>
                    </div>
 
                    <div className="grid grid-cols-2 gap-3">
                       {[
                         { label: "Trades", value: String(latestBacktestSummary.executionTradeCount ?? "--"), icon: Clock },
                         { label: "Win Rate", value: formatPercent(latestBacktestSummary.executionWinRate), icon: Play },
                         { label: "Total PnL", value: formatSigned(getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0), color: (getNumber(latestBacktestSummary.executionRealizedPnL) ?? 0) >= 0 ? 'text-[#0e6d60]' : 'text-rose-600' },
                         { label: "Max DD", value: formatPercent(latestBacktestSummary.maxDrawdown), color: 'text-amber-700' },
                       ].map((stat, i) => (
                         <div key={i} className="bg-white p-3.5 rounded-2xl border border-[#d8cfba]/40 shadow-sm">
                           <span className="block text-[8px] font-black text-[#687177] uppercase mb-1.5 opacity-60 tracking-widest">{stat.label}</span>
                           <strong className={`text-[13px] font-black block tabular-nums ${stat.color || 'text-[#1f2328]'}`}>{stat.value}</strong>
                         </div>
                       ))}
                    </div>
 
                    <div className="flex gap-2">
                       <Button 
                         variant="outline" 
                         className="flex-1 h-10 border-[#d8cfba] bg-white hover:bg-[#fff8ea] rounded-xl text-[10px] font-black"
                         disabled={!selectedBacktest?.parameters?.from || !selectedBacktest?.parameters?.to}
                         onClick={() => {
                            const from = Date.parse(String(selectedBacktest?.parameters?.from ?? ""));
                            const to = Date.parse(String(selectedBacktest?.parameters?.to ?? ""));
                            if (!Number.isFinite(from) || !Number.isFinite(to)) return;
                            setChartOverrideRange({ from: Math.floor(from/1000), to: Math.floor(to/1000), label: "Bktr Range" });
                            setFocusNonce((v) => v + 1);
                         }}
                       >
                         <Maximize2 size={12} className="mr-2 opacity-50" /> 还原复现窗口
                       </Button>
                       <Button 
                         variant="outline" 
                         className="h-10 w-10 border-[#d8cfba] bg-white hover:bg-[#fff8ea] rounded-xl flex items-center justify-center p-0"
                         onClick={() => window.open(`${API_BASE}/api/v1/backtests/${selectedBacktest.id}/execution-trades.csv`)}
                       >
                         <FileDown size={14} className="text-[#1f2328]" />
                       </Button>
                    </div>
 
                    <div className="space-y-4">
                       <div className="flex items-center gap-2 border-b border-[#d8cfba]/60 pb-2">
                          <Database size={13} className="text-[#687177]" />
                          <span className="text-[10px] font-black text-[#687177] uppercase tracking-widest">成交与观测样本审计</span>
                       </div>
                       
                       <div className="grid grid-cols-1 gap-2.5 max-h-[300px] overflow-y-auto pr-2 custom-scrollbar">
                          {latestReplayCompletedSamples.length > 0 && (
                            <div className="space-y-2">
                              {latestReplayCompletedSamples.map((sample, idx) => (
                                <SampleCard 
                                  key={`c-${idx}`} 
                                  sample={sample} 
                                  selected={selectedSample?.key === buildSampleKey("completed", idx, sample)} 
                                  onSelect={() => {
                                    const r = buildSampleRange(sample); if(!r) return;
                                    setSelectedSample({ key: buildSampleKey("completed", idx, sample), sample });
                                    setChartOverrideRange(r); setSourceFilter("backtest"); setEventFilter("all"); setFocusNonce(v => v+1);
                                  }}
                                />
                              ))}
                            </div>
                          )}
                          {latestReplaySkippedSamples.length > 0 && (
                            <div className="space-y-2">
                              {latestReplaySkippedSamples.map((sample, idx) => (
                                <SampleCard 
                                  key={`s-${idx}`} 
                                  sample={sample} 
                                  selected={selectedSample?.key === buildSampleKey("skipped", idx, sample)} 
                                  onSelect={() => {
                                    const r = buildSampleRange(sample); if(!r) return;
                                    setSelectedSample({ key: buildSampleKey("skipped", idx, sample), sample });
                                    setChartOverrideRange(r); setSourceFilter("backtest"); setEventFilter("all"); setFocusNonce(v => v+1);
                                  }}
                                />
                              ))}
                            </div>
                          )}
                          {(latestReplayCompletedSamples.length === 0 && latestReplaySkippedSamples.length === 0) && (
                            <div className="p-8 text-center text-[10px] text-[#687177] italic bg-[#fff8ea]/40 rounded-2xl border border-dashed border-[#d8cfba]">
                              未产生审计点
                            </div>
                          )}
                       </div>
                    </div>
                 </div>
               ) : (
                 <div className="flex-1 flex flex-col items-center justify-center opacity-30">
                    <BarChart4 size={40} className="mb-4" />
                    <p className="text-xs font-black uppercase tracking-widest text-[#1f2328]">分析审计就绪</p>
                 </div>
               )}
            </div>
          </div>
        </CardContent>
      </Card>
      <div className="h-8 shrink-0" /> {/* Bottom Padding */}
    </div>
  );
}
