import React, { useState, useMemo, useEffect } from 'react';
import {
  ShieldAlert,
  Search,
  CheckCircle2,
  AlertCircle,
  Zap,
  Info,
  ArrowLeft,
  Activity,
  ArrowRight,
  Database,
  RefreshCw
} from 'lucide-react';
import { toast } from 'sonner';
import { useTradingStore } from '../store/useTradingStore';
import { useUIStore } from '../store/useUIStore';
import { Card, CardHeader, CardTitle, CardDescription, CardContent } from "../components/ui/card";
import { Button } from "../components/ui/button";
import { Badge } from "../components/ui/badge";
import { 
  Table, 
  TableBody, 
  TableCell, 
  TableHead, 
  TableHeader, 
  TableRow 
} from "../components/ui/table";
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from "../components/ui/select";
import { Separator } from "../components/ui/separator";
import { fetchJSON } from '../utils/api';
import { cn } from "../lib/utils";
import { formatMaybeNumber } from '../utils/format';
import { getRecord } from '../utils/derivation';

type Step = 'scope' | 'diagnose' | 'action' | 'verify';

interface RecoveryMismatch {
  scenario: string;
  level: string;
  message: string;
  mismatchFields?: string[];
}

interface RecoveryAction {
  action: string;
  label: string;
  description: string;
  allowed: boolean;
  blockedBy?: string;
  payload?: Record<string, any>;
}

interface DiagnosisResult {
  accountId: string;
  symbol: string;
  status?: 'ok' | 'warning' | 'error';
  summary?: string;
  exchangeFact: any;
  dbFact: any;
  mismatches: RecoveryMismatch[];
  actions: RecoveryAction[];
  authoritative: boolean;
  error?: {
    stage: string;
    message: string;
    retryable: boolean;
  };
  runtimeRole: string;
  diagnosedAt: string;
}

const STEPS: { id: Step; label: string; icon: any }[] = [
  { id: 'scope', label: '范围选择', icon: Search },
  { id: 'diagnose', label: '诊断报告', icon: Activity },
  { id: 'action', label: '修复动作', icon: Zap },
  { id: 'verify', label: '结果验证', icon: CheckCircle2 },
];

export function RecoveryStage() {
  const [step, setStep] = useState<Step>('scope');
  const [selectedAccountId, setSelectedAccountId] = useState<string>("");
  const [selectedSessionId, setSelectedSessionId] = useState<string>("all");
  const [isDiagnosing, setIsDiagnosing] = useState(false);
  const [diagnosis, setDiagnosis] = useState<DiagnosisResult | null>(null);
  const [executingActionId, setExecutingActionId] = useState<string | null>(null);
  const [verificationResult, setVerificationResult] = useState<any>(null);

  const rawAccounts = useTradingStore(s => s.accounts);
  const accounts = useMemo(() => rawAccounts.filter(a => a.mode === "LIVE"), [rawAccounts]);
  const sessions = useTradingStore(s => s.liveSessions);
  const setSidebarTab = useUIStore(s => s.setSidebarTab);

  const filteredSessions = useMemo(() => {
    if (!selectedAccountId) return [];
    return sessions.filter(s => s.accountId === selectedAccountId);
  }, [sessions, selectedAccountId]);
  const diagnosisActionBlocked = !!diagnosis && (!diagnosis.authoritative || diagnosis.status === 'error');

  const handleDiagnose = async () => {
    if (!selectedAccountId) return;
    setIsDiagnosing(true);
    try {
      let symbol = "";
      if (selectedSessionId !== "all") {
        const session = sessions.find(s => s.id === selectedSessionId);
        if (session) {
          symbol = String(getRecord(session.state)["symbol"] || "");
        }
      }

      const params = new URLSearchParams();
      if (selectedSessionId !== "all") params.append("sessionId", selectedSessionId);
      if (symbol) params.append("symbol", symbol);

      const url = `/api/v1/live/accounts/${selectedAccountId}/recovery/diagnose?${params.toString()}`;
      const result = await fetchJSON<DiagnosisResult>(url);
      setDiagnosis(result);
      setStep('diagnose');
      if (result.status === 'error') {
        toast.error(result.summary || "诊断完成，但未能获取权威事实", {
          description: result.error?.message,
        });
      } else if (result.mismatches.length > 0) {
        toast.info(result.summary || `诊断完成，发现 ${result.mismatches.length} 个状态差异`);
      } else {
        toast.success(result.summary || "诊断完成，未发现状态不一致");
      }
    } catch (err: any) {
      console.error("Diagnosis failed:", err);
      toast.error("状态诊断请求失败", {
        description: err instanceof Error ? err.message : "请稍后重试",
      });
    } finally {
      setIsDiagnosing(false);
    }
  };

  const handleExecuteAction = async (action: RecoveryAction) => {
    setExecutingActionId(action.action);
    try {
      const url = `/api/v1/live/accounts/${selectedAccountId}/recovery/execute`;
      const result = await fetchJSON<any>(url, {
        method: 'POST',
        body: JSON.stringify({
          action: action.action,
          payload: action.payload || {}
        })
      });
      setVerificationResult(result);
      setStep('verify');
      toast.success("修复动作已执行", {
        description: `${action.label} 返回成功`,
      });
    } catch (err: any) {
      console.error("Action execution failed:", err);
      toast.error("修复动作执行失败", {
        description: err instanceof Error ? err.message : "请重新诊断后再试",
      });
    } finally {
      setExecutingActionId(null);
    }
  };

  return (
    <div className="absolute inset-0 overflow-y-auto space-y-8 bg-[var(--bk-canvas)] p-8">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <div className="flex items-center gap-2 mb-1">
            <ShieldAlert className="w-5 h-5 text-[var(--bk-accent)]" />
            <p className="font-mono text-[10px] font-black uppercase tracking-widest text-[var(--bk-accent)]">Maintenance Mode</p>
          </div>
          <h1 className="text-3xl font-black tracking-tighter text-[var(--bk-text-primary)]">Live Recovery Workbench</h1>
        </div>
        <Button 
          variant="bento-ghost" 
          size="sm" 
          onClick={() => setSidebarTab('account')}
          className="rounded-xl"
        >
          <ArrowLeft className="w-4 h-4 mr-2" />
          返回账户管理
        </Button>
      </div>

      <Separator className="bg-[var(--bk-border-strong)] opacity-50" />

      {/* Stepper */}
      <div className="flex items-center justify-center mb-8">
        {STEPS.map((s, i) => {
          const Icon = s.icon;
          const isActive = step === s.id;
          const isDone = i < STEPS.findIndex(x => x.id === step);
          
          return (
            <React.Fragment key={s.id}>
              <div className="flex flex-col items-center relative">
                <div className={cn(
                  "w-10 h-10 rounded-full flex items-center justify-center transition-all duration-300 border-2",
                  isActive ? "bg-[var(--bk-accent)] border-[var(--bk-accent)] text-white shadow-lg scale-110" : 
                  isDone ? "bg-[var(--bk-status-success)] border-[var(--bk-status-success)] text-white" : 
                  "bg-[var(--bk-surface-strong)] border-[var(--bk-border)] text-[var(--bk-text-muted)]"
                )}>
                  {isDone ? <CheckCircle2 className="w-6 h-6" /> : <Icon className="w-5 h-5" />}
                </div>
                <span className={cn(
                  "absolute -bottom-6 whitespace-nowrap text-[10px] font-black uppercase tracking-wider",
                  isActive ? "text-[var(--bk-accent)]" : "text-[var(--bk-text-muted)]"
                )}>
                  {s.label}
                </span>
              </div>
              {i < STEPS.length - 1 && (
                <div className={cn(
                  "w-20 h-0.5 mx-2 transition-all duration-500",
                  isDone ? "bg-[var(--bk-status-success)]" : "bg-[var(--bk-border)]"
                )} />
              )}
            </React.Fragment>
          );
        })}
      </div>

      <div className="max-w-5xl mx-auto">
        {step === 'scope' && (
          <Card tone="bento" className="rounded-[32px] border-2 border-[var(--bk-border-strong)]">
            <CardHeader>
              <CardTitle className="text-xl font-black">1. 选择修复范围</CardTitle>
              <CardDescription>选择需要进行状态诊断和修复的实盘账户。建议在系统出现不一致报警或重启后执行。</CardDescription>
            </CardHeader>
            <CardContent className="space-y-6">
              <div className="grid grid-cols-1 md:grid-cols-2 gap-6">
                <div className="space-y-2">
                  <label className="text-[10px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">实盘账户</label>
                  <Select value={selectedAccountId} onValueChange={(val) => setSelectedAccountId(val || "")}>
                    <SelectTrigger className="h-12 rounded-xl bg-[var(--bk-surface-faint)] border-[var(--bk-border)]">
                      <SelectValue placeholder="选择账户..." />
                    </SelectTrigger>
                    <SelectContent>
                      {accounts.map(a => (
                        <SelectItem key={a.id} value={a.id}>{a.name} ({a.exchange})</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>

                <div className="space-y-2">
                  <label className="text-[10px] font-black uppercase tracking-wider text-[var(--bk-text-muted)]">目标会话 (可选)</label>
                  <Select value={selectedSessionId} onValueChange={(val) => setSelectedSessionId(val || "all")} disabled={!selectedAccountId}>
                    <SelectTrigger className="h-12 rounded-xl bg-[var(--bk-surface-faint)] border-[var(--bk-border)]">
                      <SelectValue placeholder="所有会话" />
                    </SelectTrigger>
                    <SelectContent>
                      <SelectItem value="all">所有活跃会话</SelectItem>
                      {filteredSessions.map(s => (
                        <SelectItem key={s.id} value={s.id}>{String(getRecord(s.state)["symbol"] || s.id)} - {s.strategyId}</SelectItem>
                      ))}
                    </SelectContent>
                  </Select>
                </div>
              </div>

              <div className="bg-[var(--bk-surface-strong)] rounded-2xl p-6 border border-[var(--bk-border)]">
                <h4 className="text-xs font-black uppercase mb-3 flex items-center gap-2">
                  <Info className="w-4 h-4 text-[var(--bk-accent)]" />
                  注意事项
                </h4>
                <ul className="text-xs space-y-2 text-[var(--bk-text-muted)] leading-relaxed">
                  <li>• 诊断过程是只读的，不会对账户状态进行任何修改。</li>
                  <li>•  workbench 会对比 Binance REST API 的实时事实与本地数据库的缓存态。</li>
                  <li>• 修复动作可能包含删除本地过时持仓、强制对齐持仓数量等<b>破坏性操作</b>，请务必仔细核对。</li>
                  <li>• 如果存在未处理的挂单，建议先在交易所或系统内撤单再进行持仓对齐。</li>
                </ul>
              </div>

              <div className="flex justify-end pt-4">
                <Button 
                  variant="bento-primary" 
                  size="lg" 
                  disabled={!selectedAccountId || isDiagnosing}
                  onClick={handleDiagnose}
                  className="rounded-2xl px-12 h-14 font-black shadow-xl"
                >
                  {isDiagnosing ? <RefreshCw className="w-5 h-5 mr-2 animate-spin" /> : <Activity className="w-5 h-5 mr-2" />}
                  开始状态诊断
                </Button>
              </div>
            </CardContent>
          </Card>
        )}

        {step === 'diagnose' && diagnosis && (
          <div className="space-y-6">
            <div className="grid grid-cols-1 md:grid-cols-3 gap-6">
              <Card tone="bento" className="rounded-2xl border border-[var(--bk-border)]">
                <CardContent className="p-6">
                  <p className="text-[10px] font-black uppercase text-[var(--bk-text-muted)] mb-1">不一致数量</p>
                  <div className="flex items-end gap-2">
                    <span className="text-4xl font-black tracking-tighter">{diagnosis.mismatches.length}</span>
                    <Badge variant={diagnosis.mismatches.length > 0 ? "destructive" : "secondary"}>
                      {diagnosis.mismatches.length > 0 ? "需处理" : "状态正常"}
                    </Badge>
                  </div>
                </CardContent>
              </Card>
              <Card tone="bento" className="rounded-2xl border border-[var(--bk-border)]">
                <CardContent className="p-6">
                  <p className="text-[10px] font-black uppercase text-[var(--bk-text-muted)] mb-1">建议修复动作</p>
                  <div className="flex items-end gap-2">
                    <span className="text-4xl font-black tracking-tighter">{diagnosis.actions.length}</span>
                    <Badge variant="outline" className="text-[var(--bk-accent)] border-[var(--bk-accent)]">
                      {diagnosis.actions.filter(a => a.allowed).length} 可用
                    </Badge>
                  </div>
                </CardContent>
              </Card>
              <Card tone="bento" className="rounded-2xl border border-[var(--bk-border)]">
                <CardContent className="p-6">
                  <p className="text-[10px] font-black uppercase text-[var(--bk-text-muted)] mb-1">诊断时间</p>
                  <div className="text-xl font-black mt-2">
                    {new Date(diagnosis.diagnosedAt).toLocaleTimeString()}
                  </div>
                </CardContent>
              </Card>
            </div>

            {(diagnosis.summary || diagnosis.error) && (
              <Card tone="bento" className={cn(
                "rounded-2xl border",
                diagnosis.status === 'error' ? "border-[var(--bk-status-danger)]" : "border-[var(--bk-border)]"
              )}>
                <CardContent className="p-5 flex items-start gap-4">
                  {diagnosis.status === 'error' ? (
                    <AlertCircle className="w-5 h-5 text-[var(--bk-status-danger)] shrink-0 mt-0.5" />
                  ) : (
                    <Info className="w-5 h-5 text-[var(--bk-accent)] shrink-0 mt-0.5" />
                  )}
                  <div className="min-w-0 space-y-2">
                    <div className="flex flex-wrap items-center gap-2">
                      <Badge variant={diagnosis.authoritative ? "outline" : "destructive"}>
                        {diagnosis.authoritative ? "authoritative" : "non-authoritative"}
                      </Badge>
                      {diagnosis.error?.stage && (
                        <Badge variant="metal" className="font-mono text-[9px]">{diagnosis.error.stage}</Badge>
                      )}
                    </div>
                    <p className="text-sm font-bold text-[var(--bk-text-primary)]">{diagnosis.summary}</p>
                    {diagnosis.error?.message && (
                      <p className="font-mono text-xs text-[var(--bk-text-muted)] break-words">{diagnosis.error.message}</p>
                    )}
                  </div>
                </CardContent>
              </Card>
            )}

            <Card tone="bento" className="rounded-[32px] border border-[var(--bk-border-strong)] overflow-hidden">
              <div className="bg-[var(--bk-surface-strong)] px-8 py-4 border-b border-[var(--bk-border)] flex items-center justify-between">
                <h3 className="font-black flex items-center gap-2">
                  <Database className="w-4 h-4" />
                  状态差异列表
                </h3>
                <Badge variant="metal"> authorizations required for fix </Badge>
              </div>
              <div className="p-0">
                <Table>
                  <TableHeader className="bg-[var(--bk-surface-faint)]">
                    <TableRow className="border-b-[var(--bk-border)]">
                      <TableHead className="w-[120px] font-black text-[10px] uppercase">Symbol</TableHead>
                      <TableHead className="w-[180px] font-black text-[10px] uppercase">差异类型</TableHead>
                      <TableHead className="font-black text-[10px] uppercase">数据库 (Local)</TableHead>
                      <TableHead className="font-black text-[10px] uppercase">交易所 (Fact)</TableHead>
                      <TableHead className="font-black text-[10px] uppercase">严重性</TableHead>
                    </TableRow>
                  </TableHeader>
                  <TableBody>
                    {diagnosis.mismatches.length === 0 ? (
                      <TableRow>
                        <TableCell colSpan={5} className="h-32 text-center text-[var(--bk-text-muted)] font-bold">
                          未发现任何状态不一致，该账户运行良好。
                        </TableCell>
                      </TableRow>
                    ) : (
                      diagnosis.mismatches.map((m, i) => (
                        <TableRow key={i} className="hover:bg-[var(--bk-surface-faint)] border-b-[var(--bk-border)]">
                          <TableCell className="font-black text-sm">{diagnosis.symbol || '-'}</TableCell>
                          <TableCell>
                            <div className="text-xs font-bold mb-1">{m.message}</div>
                            <div className="text-[9px] font-mono text-[var(--bk-text-muted)] uppercase">{m.scenario}</div>
                          </TableCell>
                          <TableCell className="font-mono text-xs max-w-[200px] truncate">{JSON.stringify(diagnosis.dbFact.position)}</TableCell>
                          <TableCell className="font-mono text-xs text-[var(--bk-status-success)] max-w-[200px] truncate">{JSON.stringify(diagnosis.exchangeFact.position)}</TableCell>
                          <TableCell>
                            <Badge variant={m.level === 'critical' ? 'destructive' : 'outline'} className="capitalize">
                              {m.level}
                            </Badge>
                          </TableCell>
                        </TableRow>
                      ))
                    )}
                  </TableBody>
                </Table>
              </div>
            </Card>

            <div className="flex justify-between items-center">
              <Button variant="outline" onClick={() => setStep('scope')} className="rounded-xl font-black">
                重新诊断
              </Button>
              <Button 
                variant="bento-primary" 
                onClick={() => setStep('action')}
                disabled={diagnosis.actions.length === 0 || diagnosisActionBlocked}
                className="rounded-xl px-10 h-12 font-black"
              >
                进入修复阶段
                <ArrowRight className="w-4 h-4 ml-2" />
              </Button>
            </div>
          </div>
        )}

        {step === 'action' && diagnosis && (
          <div className="space-y-6">
            <Card tone="bento" className="rounded-[32px] border-2 border-[var(--bk-border-strong)] bg-[var(--bk-surface-faint)]">
              <CardHeader className="border-b border-[var(--bk-border)]">
                <div className="flex items-center gap-2 mb-2">
                  <Badge variant="outline" className="bg-[var(--bk-accent)] text-white border-none font-black px-3">STEP 3</Badge>
                  <CardTitle className="text-xl font-black">待执行修复动作</CardTitle>
                </div>
                <CardDescription>请逐一审核建议的修复动作。点击执行后，系统将对持仓或订单状态进行强制对齐。</CardDescription>
              </CardHeader>
              <CardContent className="p-0">
                <div className="divide-y divide-[var(--bk-border)]">
                  {diagnosis.actions.length === 0 ? (
                    <div className="p-12 text-center text-[var(--bk-text-muted)] font-black">
                      没有需要执行的修复动作。
                    </div>
                  ) : (
                    diagnosis.actions.map((action) => (
                      <div key={action.action} className="p-8 hover:bg-[var(--bk-surface)] transition-all">
                        <div className="flex items-start justify-between gap-6">
                          <div className="space-y-3">
                            <div className="flex items-center gap-3">
                              <h4 className="text-lg font-black tracking-tight">{action.label}</h4>
                              <Badge variant="metal" className="text-[9px] uppercase">{action.action}</Badge>
                            </div>
                            <p className="text-sm text-[var(--bk-text-muted)] leading-relaxed max-w-2xl">{action.description}</p>
                            
                            <div className="flex items-center gap-6 mt-4">
                              <div className="flex items-center gap-2">
                                <Activity className="w-4 h-4 text-[var(--bk-accent)]" />
                                <span className="text-[10px] font-black uppercase text-[var(--bk-text-muted)]">是否可用:</span>
                                <span className={cn(
                                  "text-[10px] font-black uppercase",
                                  action.allowed && !diagnosisActionBlocked ? "text-[var(--bk-status-success)]" : "text-[var(--bk-status-danger)]"
                                )}>
                                  {action.allowed && !diagnosisActionBlocked ? "ALLOWED" : "BLOCKED"}
                                </span>
                              </div>
                              {action.blockedBy && (
                                <div className="flex items-center gap-2 px-3 py-1 rounded-full bg-[var(--bk-status-warning-soft)] border border-[var(--bk-status-warning-soft)]">
                                  <ShieldAlert className="w-3 h-3 text-[var(--bk-status-warning)]" />
                                  <span className="text-[9px] font-black uppercase text-[var(--bk-status-warning)]">
                                    阻塞原因: {action.blockedBy}
                                  </span>
                                </div>
                              )}
                            </div>
                          </div>

                          <Button 
                            variant={!action.allowed || diagnosisActionBlocked ? "secondary" : (action.action.includes('clear') || action.action.includes('adopt') ? "bento-destructive" : "bento-primary")}
                            size="lg"
                            className="rounded-2xl h-16 px-8 font-black shadow-lg shrink-0"
                            onClick={() => handleExecuteAction(action)}
                            disabled={executingActionId !== null || diagnosisActionBlocked || !action.allowed}
                          >
                            {executingActionId === action.action ? (
                              <RefreshCw className="w-5 h-5 mr-2 animate-spin" />
                            ) : (
                              <Zap className="w-5 h-5 mr-2" />
                            )}
                            执行此动作
                          </Button>
                        </div>
                      </div>
                    ))
                  )}
                </div>
              </CardContent>
            </Card>

            <div className="flex justify-start">
              <Button variant="bento-ghost" onClick={() => setStep('diagnose')} className="rounded-xl font-black">
                <ArrowLeft className="w-4 h-4 mr-2" />
                回看诊断报告
              </Button>
            </div>
          </div>
        )}

        {step === 'verify' && verificationResult && (
          <Card tone="bento" className="rounded-[40px] border-4 border-[var(--bk-status-success-soft)] bg-[var(--bk-surface-faint)] overflow-hidden">
            <div className="p-12 text-center space-y-8">
              <div className="mx-auto w-24 h-24 rounded-full bg-[var(--bk-status-success-soft)] flex items-center justify-center border-4 border-[var(--bk-status-success)]">
                <CheckCircle2 className="w-12 h-12 text-[var(--bk-status-success)]" />
              </div>
              
              <div className="space-y-3">
                <h2 className="text-3xl font-black tracking-tighter">修复动作已成功执行</h2>
                <p className="text-[var(--bk-text-muted)] font-medium max-w-md mx-auto leading-relaxed">
                  系统已根据指令更新了本地状态 facts。建议此时前往<b>监控台</b>或<b>账户概览</b>确认最新的对账状态。
                </p>
              </div>

              <div className="bg-[var(--bk-surface)] rounded-3xl p-8 border border-[var(--bk-border)] text-left max-w-2xl mx-auto shadow-inner">
                <div className="flex items-center gap-2 mb-4">
                  <div className="w-2 h-2 rounded-full bg-[var(--bk-status-success)] animate-pulse" />
                  <h4 className="text-xs font-black uppercase tracking-widest text-[var(--bk-text-muted)]">执行反馈摘要</h4>
                </div>
                <div className="grid grid-cols-2 gap-y-4 gap-x-8">
                  <div className="space-y-1">
                    <p className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-50">动作类型</p>
                    <p className="font-mono text-xs font-bold">{verificationResult.actionType || "N/A"}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-50">变更对象</p>
                    <p className="font-mono text-xs font-bold">{verificationResult.targetSymbol || selectedAccountId}</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-50">事实对齐结果</p>
                    <p className="font-mono text-xs font-bold text-[var(--bk-status-success)]">SUCCESS (ALIGNED)</p>
                  </div>
                  <div className="space-y-1">
                    <p className="text-[9px] font-black uppercase text-[var(--bk-text-muted)] opacity-50">流水 ID</p>
                    <p className="font-mono text-xs font-bold">{verificationResult.traceId || "N/A"}</p>
                  </div>
                </div>
              </div>

              <div className="flex items-center justify-center gap-4 pt-4">
                <Button 
                  variant="bento-ghost" 
                  size="lg" 
                  onClick={() => setStep('diagnose')}
                  className="rounded-2xl px-8 font-black"
                >
                  继续诊断其它项
                </Button>
                <Button 
                  variant="bento-primary" 
                  size="lg" 
                  onClick={() => setSidebarTab('account')}
                  className="rounded-2xl px-12 font-black shadow-xl"
                >
                  返回账户页
                </Button>
              </div>
            </div>
          </Card>
        )}
      </div>
    </div>
  );
}
