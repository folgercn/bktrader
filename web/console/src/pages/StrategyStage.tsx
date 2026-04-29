import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { formatMaybeNumber, formatTime } from '../utils/format';
import { strategyLabel, getRecord } from '../utils/derivation';
import { Card, CardHeader, CardTitle, CardContent, CardDescription } from '../components/ui/card';
import { Button } from '../components/ui/button';
import { Input } from '../components/ui/input';
import { Label } from '../components/ui/label';
import { Textarea } from '../components/ui/textarea';
import {
  Table,
  TableBody,
  TableCell,
  TableHead,
  TableHeader,
  TableRow
} from '../components/ui/table';
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue
} from '../components/ui/select';
import { Badge } from '../components/ui/badge';
import { 
  Tooltip, 
  TooltipContent, 
  TooltipProvider, 
  TooltipTrigger 
} from '../components/ui/tooltip';
import { Separator } from '../components/ui/separator';
import { 
  HelpCircle, 
  Plus, 
  Save, 
  RotateCcw, 
  Layout, 
  FileJson, 
  List, 
  Info,
  AlertTriangle
} from 'lucide-react';

interface StrategyStageProps {
  createStrategy: () => void;
  saveStrategyParameters: () => void;
}

export function StrategyStage({ createStrategy, saveStrategyParameters }: StrategyStageProps) {
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

  // Derived states
  const selectedStrategy =
    strategies.find((item) => item.id === (selectedStrategyId || strategyEditorForm.strategyId)) ?? strategies[0] ?? null;
  const selectedStrategyVersion = selectedStrategy?.currentVersion ?? null;
  const selectedStrategyParameters = getRecord(selectedStrategyVersion?.parameters);

  return (
    <div className="absolute inset-0 overflow-y-auto space-y-8 bg-[var(--bk-canvas)] p-8">
      {/* 顶部总控 - 参照 AccountStage 范式 */}
      <Card tone="bento" className="overflow-hidden rounded-[24px] border border-[var(--bk-border-strong)] shadow-sm">
        <div className="py-3 px-6 flex flex-col md:flex-row items-center justify-between gap-4">
           <div className="flex items-center gap-6 overflow-hidden">
              <div className="shrink-0">
                <p className="mb-0.5 font-mono text-[9px] font-black uppercase tracking-widest text-[var(--bk-status-success)]">Strategy Control Center</p>
                <h2 className="whitespace-nowrap text-lg font-black tracking-tight text-[var(--bk-text-primary)]">策略库与开发实验室</h2>
              </div>
              
              <Separator orientation="vertical" className="hidden h-8 bg-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] lg:block" />
              
              <div className="hidden h-8 items-center gap-4 rounded-xl border border-[color-mix(in_srgb,var(--bk-border)_50%,transparent)] bg-[color-mix(in_srgb,var(--bk-surface-strong)_50%,transparent)] px-3 lg:flex">
                <div className="flex items-center gap-1.5">
                  <span className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-40">Active:</span>
                  <span className="text-[10px] font-black text-[var(--bk-text-primary)]">{strategies.length} 策略</span>
                </div>
                <Separator orientation="vertical" className="h-3 bg-[color-mix(in_srgb,var(--bk-border)_40%,transparent)]" />
                <div className="flex items-center gap-1.5">
                  <span className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-40">Engines:</span>
                  <span className="text-[10px] font-black text-[var(--bk-text-primary)]">{signalRuntimeAdapters.length} 适配器</span>
                </div>
              </div>
           </div>
           
           <div className="flex items-center gap-2">
              <div className="flex items-center rounded-xl border border-[color-mix(in_srgb,var(--bk-border)_20%,transparent)] bg-[var(--bk-surface-faint)] p-1">
                <Button 
                   variant="bento-ghost" 
                   size="sm" 
                   className="h-8 rounded-lg px-4 text-[10px] font-black shadow-none hover:bg-[var(--bk-surface)]"
                   onClick={() => document.getElementById('new-strategy-section')?.scrollIntoView({ behavior: 'smooth' })}
                >
                  创建新策略
                </Button>
              </div>
           </div>
        </div>
      </Card>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* 左侧：策略字典 (Archive) */}
        <Card tone="bento" className="lg:col-span-8 overflow-hidden rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div>
                <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Strategy Portfolio</p>
                <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">策略目录</CardTitle>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <div className="overflow-x-auto">
              <Table tone="bento">
                <TableHeader className="border-y border-[var(--bk-border)] bg-[var(--bk-surface-muted)]/45">
                  <TableRow className="hover:bg-transparent">
                    <TableHead className="h-10 px-6 text-[10px] font-black uppercase">策略名称 / ID</TableHead>
                    <TableHead className="h-10 text-[10px] font-black uppercase">版本</TableHead>
                    <TableHead className="h-10 text-[10px] font-black uppercase">信号周期</TableHead>
                    <TableHead className="h-10 text-[10px] font-black uppercase">运行引擎</TableHead>
                  </TableRow>
                </TableHeader>
                <TableBody className="divide-y divide-[color-mix(in_srgb,var(--bk-border)_50%,transparent)]">
                  {strategies.length > 0 ? (
                    strategies.map((strategy) => {
                      const params = getRecord(strategy.currentVersion?.parameters);
                      const isSelected = strategy.id === selectedStrategy?.id;
                      return (
                        <TableRow 
                          key={strategy.id} 
                          className={`group relative z-10 cursor-pointer transition-all duration-200 ${isSelected ? 'bg-[var(--bk-surface)] shadow-inner' : 'bg-[var(--bk-surface-strong)] hover:bg-[var(--bk-surface)]'}`}
                          onClick={() => setSelectedStrategyId(strategy.id)}
                        >
                          <TableCell className="px-6 py-4">
                            <div className="flex items-center gap-3">
                               <div className={`h-8 w-1 rounded-full transition-all ${isSelected ? 'bg-[var(--bk-status-success)]' : 'bg-transparent group-hover:bg-[var(--bk-border)]'}`} />
                               <div className="flex flex-col min-w-0">
                                 <span className="truncate text-sm font-black tracking-tight text-[var(--bk-text-primary)] tabular-nums">{strategy.name}</span>
                                 <span className="truncate font-mono text-[9px] uppercase text-[var(--bk-text-muted)] opacity-60">{strategy.id}</span>
                               </div>
                            </div>
                          </TableCell>
                          <TableCell>
                            <Badge variant="neutral" className={`h-4.5 text-[10px] font-bold ${isSelected ? 'border-transparent bg-[var(--bk-surface-inverse)] text-[var(--bk-text-contrast)]' : 'bg-[var(--bk-surface)] text-[var(--bk-text-muted)]'}`}>
                              v{strategy.currentVersion?.version ?? "1"}
                            </Badge>
                          </TableCell>
                          <TableCell className="font-mono text-xs font-black text-[color-mix(in_srgb,var(--bk-text-primary)_70%,transparent)]">
                            {String(params.signalTimeframe ?? strategy.currentVersion?.signalTimeframe ?? "--")}
                          </TableCell>
                          <TableCell>
                             <div className="flex w-fit items-center gap-1.5 rounded-md border border-[color-mix(in_srgb,var(--bk-border)_50%,transparent)] bg-[var(--bk-surface)] px-2 py-0.5">
                               <div className="size-1 rounded-full bg-[var(--bk-status-success)]" />
                               <span className="text-[10px] font-black uppercase text-[var(--bk-text-primary)]">{String(params.strategyEngine ?? "bk-default")}</span>
                             </div>
                          </TableCell>
                        </TableRow>
                      );
                    })
                  ) : (
                    <TableRow>
                      <TableCell colSpan={4} className="h-40 text-center text-[11px] font-medium italic text-[var(--bk-text-muted)]">暂无策略数据</TableCell>
                    </TableRow>
                  )}
                </TableBody>
              </Table>
            </div>
          </CardContent>
        </Card>
 
        {/* 右侧：版本摘要 (Details) - 参照 AccountStage 细节卡片 */}
        <Card tone="bento" className="lg:col-span-4 flex flex-col overflow-hidden rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div>
                <p className="font-mono text-[10px] font-bold uppercase tracking-widest text-[var(--bk-status-success)]">Summary</p>
                <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">当前版本摘要</CardTitle>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-6 space-y-6 flex-1">
            {selectedStrategy ? (
              <>
                <div className="space-y-4">
                  <div className="grid grid-cols-2 gap-3">
                     <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface)] p-3.5 shadow-sm">
                        <span className="mb-1.5 block text-[8px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Signal Source</span>
                        <p className="truncate text-xs font-black text-[var(--bk-text-primary)]">{String(selectedStrategyParameters.executionDataSource ?? "--")}</p>
                     </div>
                     <div className="rounded-2xl border border-[color-mix(in_srgb,var(--bk-border)_60%,transparent)] bg-[var(--bk-surface)] p-3.5 shadow-sm">
                        <span className="mb-1.5 block text-[8px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Engine Type</span>
                        <p className="text-xs font-black uppercase text-[var(--bk-text-primary)]">{String(selectedStrategyParameters.strategyEngine ?? "bk-default")}</p>
                     </div>
                  </div>
 
                  <div className="space-y-3 rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface-strong)] p-4 shadow-inner">
                     <div className="flex items-center gap-2 border-b border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] pb-2">
                       <Info size={14} className="text-[var(--bk-status-success)]" />
                       <span className="text-[9px] font-black uppercase tracking-widest text-[var(--bk-status-success)]">Metadata</span>
                     </div>
                     <div className="grid grid-cols-1 gap-2.5">
                       <div className="flex justify-between items-center text-[11px]">
                         <span className="font-medium text-[var(--bk-text-muted)]">创建时间</span>
                         <span className="font-mono font-bold text-[var(--bk-text-primary)]">{formatTime(selectedStrategy.createdAt).split(' ')[0]}</span>
                       </div>
                       <div className="flex justify-between items-center text-[11px]">
                         <span className="font-medium text-[var(--bk-text-muted)]">最后编译版本</span>
                         <Badge variant="neutral" className="h-4 text-[9px] font-black">v{selectedStrategyVersion?.version ?? "1"}</Badge>
                       </div>
                     </div>
                  </div>
                </div>
 
                <div className="min-h-[80px] flex-1 rounded-2xl border border-dashed border-[var(--bk-border)] bg-[var(--bk-surface-faint)] p-4">
                   <p className="mb-2 text-[10px] font-black uppercase tracking-widest text-[var(--bk-text-muted)] opacity-60">Description</p>
                   <p className="text-xs font-medium leading-relaxed text-[var(--bk-text-primary)]">
                     {selectedStrategy.description || "暂无描述内容"}
                   </p>
                </div>
              </>
            ) : (
              <div className="flex flex-1 flex-col items-center justify-center space-y-3 text-[var(--bk-border)]">
                <RotateCcw className="size-10 animate-spin-slow opacity-20" />
                <p className="text-xs font-black uppercase tracking-widest">请选择策略</p>
              </div>
            )}
          </CardContent>
          {selectedStrategy && (
            <div className="mx-6 mb-6 rounded-xl border border-amber-200 bg-amber-50 p-4">
               <div className="flex gap-2">
                 <AlertTriangle size={12} className="shrink-0 mt-0.5 text-[var(--bk-status-warning)]" />
                 <p className="text-[9px] leading-normal font-medium italic text-[var(--bk-status-warning)]">
                    警告：热更新模式已启用。编辑参数将立即波及所有运行中的反射引擎。
                 </p>
               </div>
            </div>
          )}
        </Card>
      </div>

      <div id="new-strategy-section" className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* 开发区左侧：建立新策略 - 风格对齐 AccountStage 的 Modal 样式 */}
        <Card tone="bento" className="overflow-hidden rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-[var(--bk-canvas-strong)] p-1.5">
                <Plus className="size-4 text-[var(--bk-text-primary)]" />
              </div>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">建立新策略</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-6 space-y-5">
            <div className="space-y-4">
              <div className="space-y-1.5">
                <Label className="ml-1 text-[11px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">Strategy Name</Label>
                <Input 
                  value={strategyCreateForm.name}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="例如：BK-TREND-01"
                  className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-bold shadow-sm transition-all"
                />
              </div>
              <div className="space-y-1.5">
                <Label className="ml-1 text-[11px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">Internal Description</Label>
                <Input 
                  value={strategyCreateForm.description}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, description: e.target.value }))}
                  placeholder="描述逻辑边界或执行目的"
                  className="h-10 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface)] text-xs font-medium shadow-sm transition-all"
                />
              </div>
            </div>
            <Button 
              variant="bento"
              className="h-11 w-full rounded-xl text-xs font-black shadow-md transition-all active:scale-95 disabled:opacity-50"
              disabled={strategyCreateAction || !strategyCreateForm.name.trim()}
              onClick={createStrategy}
            >
              {strategyCreateAction ? "正在注入逻辑..." : "确认并提交策略库"}
            </Button>
          </CardContent>
        </Card>
 
        {/* 开发区右侧：参数反射编辑器 (Refl Editor) */}
        <Card tone="bento" className="flex flex-col overflow-hidden rounded-[24px] shadow-[var(--bk-shadow-card)]">
          <CardHeader className="pb-4 flex flex-row items-center justify-between">
            <div className="flex items-center gap-2">
              <div className="rounded-lg bg-[var(--bk-canvas-strong)] p-1.5">
                <FileJson className="size-4 text-[var(--bk-text-primary)]" />
              </div>
              <CardTitle className="text-lg font-black text-[var(--bk-text-primary)]">参数反射编辑器</CardTitle>
            </div>
            <TooltipProvider>
              <Tooltip>
                <TooltipTrigger>
                  <Button variant="bento-ghost" size="icon" className="h-8 w-8 rounded-xl hover:bg-[var(--bk-surface-faint)]">
                    <HelpCircle className="size-4 text-[var(--bk-text-muted)]" />
                  </Button>
                </TooltipTrigger>
                <TooltipContent className="w-64 rounded-xl border-0 bg-[var(--bk-surface-inverse)] p-3 text-[10px] text-[var(--bk-text-contrast)] shadow-2xl">
                   热更新模式下，保存参数将通过反射引擎立即更新至对应的运行实例，建议在生产运行前进行参数校验。
                </TooltipContent>
              </Tooltip>
            </TooltipProvider>
          </CardHeader>
          <CardContent className="p-6 space-y-5 flex-1 flex flex-col">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-1.5">
                <Label className="ml-1 text-[11px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">Engine</Label>
                <Select 
                  value={strategyEditorForm.strategyEngine}
                  onValueChange={(val: any) => setStrategyEditorForm(prev => ({ ...prev, strategyEngine: val }))}
                >
                  <SelectTrigger tone="bento" className="h-10 rounded-xl bg-[var(--bk-surface)] text-xs font-black">
                    <SelectValue placeholder="选择引擎" />
                  </SelectTrigger>
                  <SelectContent tone="bento" className="rounded-xl bg-[var(--bk-surface)] shadow-2xl">
                    {[...new Set(["bk-default", ...strategies.map((item) => String(getRecord(item.currentVersion?.parameters).strategyEngine || "bk-default"))])].map((engineKey) => (
                      <SelectItem key={engineKey} value={engineKey}>{engineKey}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-1.5">
                <Label className="ml-1 text-[11px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">Timeframe</Label>
                <Select 
                  value={strategyEditorForm.signalTimeframe}
                  onValueChange={(val: any) => setStrategyEditorForm(prev => ({ ...prev, signalTimeframe: val }))}
                >
                  <SelectTrigger tone="bento" className="h-10 rounded-xl bg-[var(--bk-surface)] text-xs font-black">
                    <SelectValue placeholder="周期" />
                  </SelectTrigger>
                  <SelectContent tone="bento" className="rounded-xl bg-[var(--bk-surface)] shadow-2xl">
                    <SelectItem value="5m">5m</SelectItem>
                    <SelectItem value="15m">15m</SelectItem>
                    <SelectItem value="30m">30m</SelectItem>
                    <SelectItem value="4h">4h</SelectItem>
                    <SelectItem value="1d">1d</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>
 
            <div className="space-y-2 flex-1 flex flex-col">
              <div className="flex items-center justify-between">
                 <Label className="ml-1 text-[11px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">Parameters Schema</Label>
                 <Button 
                   variant="ghost" 
                   size="sm" 
                   className="h-6 px-2 text-[9px] font-black text-[var(--bk-status-success)]"
                   onClick={() => {
                     try { setStrategyEditorForm(prev => ({ ...prev, parametersJson: JSON.stringify(JSON.parse(prev.parametersJson), null, 2) })) } catch (e) {}
                   }}
                 >
                   格式化 JSON
                 </Button>
              </div>
              <div className="relative flex-1 group">
                 <div className="pointer-events-none absolute bottom-0 left-0 top-0 flex w-8 flex-col items-center rounded-l-2xl border-r border-[color-mix(in_srgb,var(--bk-border)_40%,transparent)] bg-[color-mix(in_srgb,var(--bk-canvas)_30%,transparent)] pt-3 opacity-40">
                    {[1,2,3,4,5,6,7].map(n => <span key={n} className="text-[8px] font-mono leading-relaxed">{n}</span>)}
                 </div>
                 <Textarea 
                   value={strategyEditorForm.parametersJson}
                   onChange={(e) => setStrategyEditorForm(prev => ({ ...prev, parametersJson: e.target.value }))}
                   className="h-full min-h-[160px] w-full resize-none rounded-2xl border-[var(--bk-border)] bg-[color-mix(in_srgb,var(--bk-surface-strong)_40%,transparent)] py-3 pl-10 pr-4 font-mono text-[11px] leading-relaxed shadow-inner transition-all focus:bg-[var(--bk-surface)]"
                   placeholder='{"key": "value"}'
                 />
              </div>
            </div>
 
            <div className="flex gap-3 pt-2">
               <Button 
                 variant="bento"
                 className="h-11 flex-1 rounded-xl text-xs font-black shadow-md transition-transform active:scale-95"
                 disabled={strategySaveAction || !strategyEditorForm.strategyId}
                 onClick={saveStrategyParameters}
               >
                 <Save size={14} className="mr-2" />
                 {strategySaveAction ? "正在提交同步..." : "保存反射参数"}
               </Button>
               <Button 
                 variant="bento-outline" 
                 size="icon" 
                 className="h-11 w-11 rounded-xl bg-[var(--bk-surface)] hover:bg-[var(--bk-surface-strong)]"
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
      <div className="h-8 shrink-0" /> {/* Bottom Padding */}
    </div>
  );
}
