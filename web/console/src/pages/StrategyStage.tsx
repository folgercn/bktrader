import React, { useMemo, useState } from 'react';
import { useUIStore } from '../store/useUIStore';
import { useTradingStore } from '../store/useTradingStore';
import { formatTime } from '../utils/format';
import { strategyLabel, getRecord } from '../utils/derivation';
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
import { HelpCircle, Plus, Save, RotateCcw, Layout, FileJson, List, Info } from 'lucide-react';

interface StrategyStageProps {
  createStrategy: () => void;
  saveStrategyParameters: () => void;
}

export function StrategyStage({ createStrategy, saveStrategyParameters }: StrategyStageProps) {
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
  const strategyOptions = useMemo(
    () =>
      strategies.map((strategy) => ({
        value: strategy.id,
        label: strategyLabel(strategy),
      })),
    [strategies]
  );

  const selectedStrategy =
    strategies.find((item) => item.id === (selectedStrategyId || strategyEditorForm.strategyId)) ?? strategies[0] ?? null;
  const selectedStrategyVersion = selectedStrategy?.currentVersion ?? null;
  const selectedStrategyParameters = getRecord(selectedStrategyVersion?.parameters);

  return (
    <div className="p-8 space-y-8 animate-in fade-in duration-500">
      {/* 顶部统计与导航 */}
      <div className="flex items-center justify-between">
        <div>
          <h2 className="text-3xl font-black text-[#1f2328] tracking-tight">策略主舞台</h2>
          <p className="text-[#687177] text-sm mt-1">管理交易逻辑的核心引擎与参数矩阵</p>
        </div>
        <div className="flex items-center gap-3 bg-[#ebe5d5]/50 px-4 py-2 rounded-2xl border border-[#d8cfba]">
          <div className="flex flex-col items-end">
            <span className="text-[10px] text-[#687177] uppercase font-bold tracking-widest">Active Pool</span>
            <span className="text-sm font-black text-[#1f2328]">{strategies.length} 策略 · {signalRuntimeAdapters.length} 引擎</span>
          </div>
          <Layout className="text-[#1f2328]/20 size-8" />
        </div>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-12 gap-8">
        {/* 左侧：策略字典 (Table) */}
        <Card className="lg:col-span-8 border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden">
          <CardHeader className="border-b border-[#d8cfba]/50 bg-white/30 px-8 py-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-[#ebe5d5] rounded-xl">
                  <List className="size-5 text-[#1f2328]" />
                </div>
                <CardTitle className="text-xl font-bold text-[#1f2328]">策略目录</CardTitle>
              </div>
            </div>
          </CardHeader>
          <CardContent className="p-0">
            <Table>
              <TableHeader className="bg-[#ebe5d5]/30">
                <TableRow className="hover:bg-transparent border-[#d8cfba]/50">
                  <TableHead className="px-8 h-12 text-[11px] uppercase font-black text-[#687177]">策略名称</TableHead>
                  <TableHead className="h-12 text-[11px] uppercase font-black text-[#687177]">版本</TableHead>
                  <TableHead className="h-12 text-[11px] uppercase font-black text-[#687177]">信号周期</TableHead>
                  <TableHead className="h-12 text-[11px] uppercase font-black text-[#687177]">数据源</TableHead>
                  <TableHead className="h-12 text-[11px] uppercase font-black text-[#687177]">运行引擎</TableHead>
                </TableRow>
              </TableHeader>
              <TableBody>
                {strategies.length > 0 ? (
                  strategies.map((strategy) => {
                    const parameters = getRecord(strategy.currentVersion?.parameters);
                    const isSelected = strategy.id === selectedStrategy?.id;
                    return (
                      <TableRow 
                        key={strategy.id} 
                        className={`group cursor-pointer border-[#d8cfba]/30 transition-all ${isSelected ? 'bg-white shadow-inner' : 'hover:bg-white/50'}`}
                        onClick={() => setSelectedStrategyId(strategy.id)}
                      >
                        <TableCell className="px-8 py-4">
                          <div className="flex flex-col">
                            <span className="font-bold text-[#1f2328]">{strategy.name}</span>
                            <span className="text-[10px] text-[#687177] font-mono">{strategy.id}</span>
                          </div>
                        </TableCell>
                        <TableCell>
                          <Badge variant="outline" className="font-mono text-[10px] bg-white border-[#d8cfba]">
                            v{strategy.currentVersion?.version ?? "--"}
                          </Badge>
                        </TableCell>
                        <TableCell className="text-xs font-medium text-[#687177]">
                          {String(parameters.signalTimeframe ?? strategy.currentVersion?.signalTimeframe ?? "--")}
                        </TableCell>
                        <TableCell className="text-xs font-mono text-[#0e6d60]">
                          {String(parameters.executionDataSource ?? parameters.executionTimeframe ?? strategy.currentVersion?.executionTimeframe ?? "--")}
                        </TableCell>
                        <TableCell>
                          <span className="text-xs font-bold text-[#1f2328]/70">
                            {String(parameters.strategyEngine ?? "bk-default")}
                          </span>
                        </TableCell>
                      </TableRow>
                    );
                  })
                ) : (
                  <TableRow>
                    <TableCell colSpan={5} className="h-40 text-center text-[#687177] italic">暂无策略数据</TableCell>
                  </TableRow>
                )}
              </TableBody>
            </Table>
          </CardContent>
        </Card>

        {/* 右侧：版本摘要 (Details) */}
        <Card className="lg:col-span-4 border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden flex flex-col">
          <CardHeader className="border-b border-[#d8cfba]/50 bg-white/30 px-6 py-6">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-[#ebe5d5] rounded-xl">
                <Info className="size-5 text-[#1f2328]" />
              </div>
              <CardTitle className="text-lg font-bold text-[#1f2328]">当前版本摘要</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-6 flex-1">
            {selectedStrategy ? (
              <div className="space-y-4">
                <div className="bg-white/50 rounded-[20px] border border-[#d8cfba]/50 p-5 space-y-4">
                  <div className="flex justify-between items-start">
                    <div className="space-y-1">
                      <span className="text-[10px] text-[#687177] uppercase font-bold">策略描述</span>
                      <p className="text-xs text-[#1f2328] font-medium leading-relaxed">
                        {selectedStrategy.description || "暂无描述内容"}
                      </p>
                    </div>
                  </div>
                  
                  <div className="h-px bg-[#d8cfba]/30 w-full" />

                  <div className="grid grid-cols-2 gap-4">
                    <div className="space-y-1">
                      <span className="text-[10px] text-[#687177] uppercase font-bold">创建时间</span>
                      <p className="text-xs text-[#1f2328] font-mono">{formatTime(selectedStrategy.createdAt)}</p>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-[#687177] uppercase font-bold">运行引擎</span>
                      <p className="text-xs text-[#1f2328] font-bold">{String(selectedStrategyParameters.strategyEngine ?? "bk-default")}</p>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-[#687177] uppercase font-bold">信号周期</span>
                      <p className="text-xs text-[#1f2328] font-bold">{String(selectedStrategyParameters.signalTimeframe ?? selectedStrategyVersion?.signalTimeframe ?? "--")}</p>
                    </div>
                    <div className="space-y-1">
                      <span className="text-[10px] text-[#687177] uppercase font-bold">执行源</span>
                      <p className="text-xs text-[#1f2328] font-mono text-[#0e6d60]">{String(selectedStrategyParameters.executionDataSource ?? selectedStrategyVersion?.executionTimeframe ?? "--")}</p>
                    </div>
                  </div>
                </div>

                <div className="p-4 rounded-2xl bg-[#fff8ea] border border-[#d8cfba] flex gap-3">
                  <HelpCircle className="size-4 text-[#1f2328]/40 shrink-0 mt-0.5" />
                  <p className="text-[10px] text-[#687177] leading-relaxed italic">
                    警告：当前版本为“开发热更新”模式。直接编辑参数将立即同步至所有运行中进程，可能导致实盘逻辑突跳。请在低频或空仓期间进行操作。
                  </p>
                </div>
              </div>
            ) : (
              <div className="h-full flex flex-col items-center justify-center space-y-3 opacity-40">
                <RotateCcw className="size-10 animate-spin-slow" />
                <p className="text-sm font-medium">请在左侧选择策略</p>
              </div>
            )}
          </CardContent>
        </Card>
      </div>

      <div className="grid grid-cols-1 lg:grid-cols-2 gap-8">
        {/* 下排左侧：创建新策略 */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden">
          <CardHeader className="border-b border-[#d8cfba]/50 bg-white/30 px-8 py-6">
            <div className="flex items-center gap-3">
              <div className="p-2 bg-[#ebe5d5] rounded-xl">
                <Plus className="size-5 text-[#1f2328]" />
              </div>
              <CardTitle className="text-xl font-bold text-[#1f2328]">创建新策略</CardTitle>
            </div>
          </CardHeader>
          <CardContent className="p-8 space-y-6">
            <div className="grid grid-cols-1 gap-6">
              <div className="space-y-2">
                <Label className="text-sm font-black text-[#1f2328] ml-1">策略名称</Label>
                <Input 
                  value={strategyCreateForm.name}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, name: e.target.value }))}
                  placeholder="例如：BK 4H Runner"
                  className="h-11 rounded-xl border-[#d8cfba] bg-white/50 focus:bg-white transition-all shadow-sm"
                />
              </div>
              <div className="space-y-2">
                <Label className="text-sm font-black text-[#1f2328] ml-1">策略说明</Label>
                <Input 
                  value={strategyCreateForm.description}
                  onChange={(e) => setStrategyCreateForm(prev => ({ ...prev, description: e.target.value }))}
                  placeholder="记录这条策略的用途、市场和执行方式"
                  className="h-11 rounded-xl border-[#d8cfba] bg-white/50 focus:bg-white transition-all shadow-sm"
                />
              </div>
            </div>
            <Button 
              className="w-full h-12 rounded-xl bg-[#1f2328] hover:bg-[#2f353c] text-white font-bold text-base transition-all shadow-lg active:scale-95 disabled:opacity-50"
              disabled={strategyCreateAction || !strategyCreateForm.name.trim()}
              onClick={createStrategy}
            >
              <Plus className="size-4 mr-2" />
              {strategyCreateAction ? "正在构建环境..." : "建立该策略"}
            </Button>
          </CardContent>
        </Card>

        {/* 下排右侧：策略参数编辑库 */}
        <Card className="border-[#d8cfba] bg-[var(--panel)] shadow-xl rounded-[32px] overflow-hidden">
          <CardHeader className="border-b border-[#d8cfba]/50 bg-white/30 px-8 py-6">
            <div className="flex items-center justify-between">
              <div className="flex items-center gap-3">
                <div className="p-2 bg-[#ebe5d5] rounded-xl">
                  <FileJson className="size-5 text-[#1f2328]" />
                </div>
                <CardTitle className="text-xl font-bold text-[#1f2328]">配置编辑器</CardTitle>
              </div>
              <TooltipProvider>
                <Tooltip>
                  <TooltipTrigger>
                    <Button variant="ghost" size="icon" className="rounded-full hover:bg-white/50">
                      <HelpCircle className="size-4 text-[#687177]" />
                    </Button>
                  </TooltipTrigger>
                  <TooltipContent className="bg-[#1f2328] text-white border-0 rounded-lg p-2 text-[10px]">
                    直接修改 JSON 参数将同步至 Runtime 反射引擎
                  </TooltipContent>
                </Tooltip>
              </TooltipProvider>
            </div>
          </CardHeader>
          <CardContent className="p-8 space-y-6">
            <div className="grid grid-cols-2 gap-4">
              <div className="space-y-2">
                <Label className="text-sm font-black text-[#1f2328]">运行引擎</Label>
                <Select 
                  value={strategyEditorForm.strategyEngine}
                  onValueChange={(val) => setStrategyEditorForm(prev => ({ ...prev, strategyEngine: val }))}
                >
                  <SelectTrigger className="h-11 rounded-xl border-[#d8cfba] bg-white/50">
                    <SelectValue placeholder="选择引擎" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                    {[...new Set(["bk-default", ...strategies.map((item) => String(getRecord(item.currentVersion?.parameters).strategyEngine || "bk-default"))])].map((engineKey) => (
                      <SelectItem key={engineKey} value={engineKey}>{engineKey}</SelectItem>
                    ))}
                  </SelectContent>
                </Select>
              </div>
              <div className="space-y-2">
                <Label className="text-sm font-black text-[#1f2328]">信号周期</Label>
                <Select 
                  value={strategyEditorForm.signalTimeframe}
                  onValueChange={(val) => setStrategyEditorForm(prev => ({ ...prev, signalTimeframe: val }))}
                >
                  <SelectTrigger className="h-11 rounded-xl border-[#d8cfba] bg-white/50">
                    <SelectValue placeholder="周期" />
                  </SelectTrigger>
                  <SelectContent className="bg-white border-[#d8cfba] rounded-xl">
                    <SelectItem value="4h">4h</SelectItem>
                    <SelectItem value="1d">1d</SelectItem>
                  </SelectContent>
                </Select>
              </div>
            </div>

            <div className="space-y-2">
              <div className="flex items-center justify-between ml-1">
                <Label className="text-sm font-black text-[#1f2328]">参数 JSON (Advanced)</Label>
                <Badge variant="outline" className="text-[9px] border-[#d8cfba] uppercase text-[#687177]">Raw Edit</Badge>
              </div>
              <Textarea 
                rows={10}
                value={strategyEditorForm.parametersJson}
                onChange={(e) => setStrategyEditorForm(prev => ({ ...prev, parametersJson: e.target.value }))}
                className="font-mono text-xs rounded-2xl border-[#d8cfba] bg-[#fffbf2]/80 focus:bg-white transition-all shadow-inner leading-relaxed"
                placeholder='{"stop_loss_atr":0.05,"profit_protect_atr":1.0}'
              />
            </div>

            <div className="flex gap-4">
              <Button 
                className="flex-1 h-12 rounded-xl bg-[#0e6d60] hover:bg-[#128071] text-white font-bold shadow-lg transition-all active:scale-95"
                disabled={strategySaveAction || !strategyEditorForm.strategyId}
                onClick={saveStrategyParameters}
              >
                <Save className="size-4 mr-2" />
                {strategySaveAction ? "提交变更中..." : "保存反射参数"}
              </Button>
              <Button 
                variant="outline"
                className="h-12 px-6 rounded-xl border-[#d8cfba] hover:bg-white text-[#1f2328] font-bold"
                onClick={() => {
                  if (!selectedStrategy) return;
                  const currentParameters = getRecord(selectedStrategy.currentVersion?.parameters);
                  setStrategyEditorForm({
                    strategyId: selectedStrategy.id,
                    strategyEngine: String(currentParameters.strategyEngine ?? "bk-default"),
                    signalTimeframe: String(
                      currentParameters.signalTimeframe ??
                        selectedStrategy.currentVersion?.signalTimeframe ??
                        "1d"
                    ),
                    executionDataSource: String(
                      currentParameters.executionDataSource ??
                        currentParameters.executionTimeframe ??
                        selectedStrategy.currentVersion?.executionTimeframe ??
                        "tick"
                    ),
                    parametersJson: JSON.stringify(currentParameters, null, 2),
                  });
                }}
              >
                <RotateCcw className="size-4" />
              </Button>
            </div>
          </CardContent>
        </Card>
      </div>
    </div>
  );
}
