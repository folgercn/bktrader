import React, { useMemo, useState, useEffect } from 'react';
import { useTradingStore } from '../store/useTradingStore';
import { useUIStore } from '../store/useUIStore';
import { formatFullLogTime, shrink } from '../utils/format';
import { getList } from '../utils/derivation';
import { 
  Terminal, 
  AlertCircle, 
  Info, 
  Bell, 
  FileText, 
  Activity, 
  Zap, 
  Filter, 
  RefreshCcw, 
  Pause, 
  Play,
  ShieldAlert,
  Search
} from 'lucide-react';
import { Card, CardHeader, CardTitle, CardContent } from '../components/ui/card';
import { Badge } from '../components/ui/badge';
import { Input } from '../components/ui/input';
import { Button } from '../components/ui/button';
import { 
  Select, 
  SelectContent, 
  SelectItem, 
  SelectTrigger, 
  SelectValue 
} from '../components/ui/select';
import { Separator } from '../components/ui/separator';

type LogSource = "alert" | "notification" | "timeline" | "system";
type LogLevel = "critical" | "warning" | "info" | "debug";

interface ConsoleLogEvent {
  id: string;
  source: LogSource;
  level: LogLevel;
  title: string;
  message: string;
  eventTime: number;
  metadata?: any;
}

export function LogStage() {
  const [logType, setLogType] = useState<LogSource | "all">("all");
  const [levelFilter, setLevelFilter] = useState<LogLevel | "all">("all");
  const [autoRefresh, setAutoRefresh] = useState(true);
  const [searchQuery, setSearchQuery] = useState("");

  const alerts = useTradingStore(s => s.alerts);
  const notifications = useTradingStore(s => s.notifications);
  const liveSessions = useTradingStore(s => s.liveSessions);
  const signalRuntimeSessions = useTradingStore(s => s.signalRuntimeSessions);
  const systemLogs = useUIStore(s => s.systemLogs);

  const processedEvents = useMemo(() => {
    const events: ConsoleLogEvent[] = [];

    // Alerts
    alerts.forEach(alert => {
      events.push({
        id: `alert-${alert.id}`,
        source: "alert",
        level: (alert.level as LogLevel) || "info",
        title: alert.title,
        message: alert.detail,
        eventTime: Date.parse(alert.eventTime || ""),
        metadata: alert,
      });
    });

    // Notifications
    notifications.forEach(notif => {
      events.push({
        id: `notif-${notif.id}`,
        source: "notification",
        level: (notif.alert?.level as LogLevel) || "info",
        title: notif.alert?.title || "Notification",
        message: notif.alert?.detail || "",
        eventTime: Date.parse(notif.updatedAt || ""),
        metadata: notif,
      });
    });

    // System Logs
    systemLogs.forEach(log => {
      events.push({
        id: `system-${log.id}`,
        source: "system",
        level: log.level === "error" ? "critical" : "info",
        title: "系统日志",
        message: log.message,
        eventTime: Date.parse(log.createdAt),
      });
    });

    // Timelines from LiveSessions
    liveSessions.forEach(session => {
      const timeline = getList(session.state?.timeline);
      timeline.forEach((item, index) => {
        events.push({
          id: `timeline-live-${session.id}-${index}`,
          source: "timeline",
          level: (item.category === "error" || String(item.title).toLowerCase().includes("error")) ? "warning" : "info",
          title: String(item.title || "Timeline Event"),
          message: `${session.accountId} | ${session.strategyId}`,
          eventTime: Date.parse(String(item.time || "")),
          metadata: { ...item, sessionId: session.id },
        });
      });
    });

    // Timelines from SignalRuntimeSessions
    signalRuntimeSessions.forEach(session => {
        const timeline = getList(session.state?.timeline);
        timeline.forEach((item, index) => {
          events.push({
            id: `timeline-runtime-${session.id}-${index}`,
            source: "timeline",
            level: (item.category === "error" || String(item.title).toLowerCase().includes("error")) ? "warning" : "info",
            title: String(item.title || "Runtime Event"),
            message: `${session.accountId} | ${session.strategyId}`,
            eventTime: Date.parse(String(item.time || "")),
            metadata: { ...item, sessionId: session.id },
          });
        });
      });

    return events.sort((a, b) => b.eventTime - a.eventTime);
  }, [alerts, notifications, systemLogs, liveSessions, signalRuntimeSessions]);

  const [snapshot, setSnapshot] = useState<ConsoleLogEvent[]>(() => processedEvents);

  useEffect(() => {
    if (autoRefresh) {
      setSnapshot(processedEvents);
    }
  }, [processedEvents, autoRefresh]);

  const filteredEvents = useMemo(() => {
    return snapshot
      .filter(e => {
        const matchesType = logType === "all" || e.source === logType;
        const matchesLevel = levelFilter === "all" || e.level === levelFilter;
        const matchesSearch = searchQuery === "" || 
          e.title.toLowerCase().includes(searchQuery.toLowerCase()) || 
          e.message.toLowerCase().includes(searchQuery.toLowerCase());
        return matchesType && matchesLevel && matchesSearch;
      })
      .slice(0, 500);
  }, [snapshot, logType, levelFilter, searchQuery]);

  return (
    <div className="flex h-full flex-col bg-[var(--bk-canvas)]">
      {/* 顶部过滤器 - 采用熟悉的奶油色面板风格 */}
      <header className="z-20 flex shrink-0 items-center justify-between gap-4 border-b border-[var(--bk-border)] bg-[var(--bk-surface)] px-6 py-4 shadow-sm backdrop-blur-md">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
             <Filter size={14} className="text-[var(--bk-text-muted)]" />
             <Select value={logType} onValueChange={(val) => setLogType(val as any)}>
               <SelectTrigger tone="bento" className="h-8 w-32 bg-[var(--bk-surface-faint)] text-[11px] text-[var(--bk-text-primary)]">
                 <SelectValue placeholder="来源" />
               </SelectTrigger>
               <SelectContent tone="bento" className="bg-[var(--bk-surface-overlay-strong)]">
                 <SelectItem value="all">所有来源</SelectItem>
                 <SelectItem value="alert">异常告警</SelectItem>
                 <SelectItem value="notification">通知</SelectItem>
                 <SelectItem value="timeline">执行时间线</SelectItem>
                 <SelectItem value="system">系统日志</SelectItem>
               </SelectContent>
             </Select>
          </div>

          <div className="flex items-center gap-2">
             <Zap size={14} className="text-[var(--bk-text-muted)]" />
             <Select value={levelFilter} onValueChange={(val) => setLevelFilter(val as any)}>
               <SelectTrigger tone="bento" className="h-8 w-32 bg-[var(--bk-surface-faint)] text-[11px] text-[var(--bk-text-primary)]">
                 <SelectValue placeholder="等级" />
               </SelectTrigger>
               <SelectContent tone="bento" className="bg-[var(--bk-surface-overlay-strong)]">
                 <SelectItem value="all">所有等级</SelectItem>
                 <SelectItem value="critical">严重 (Critical)</SelectItem>
                 <SelectItem value="warning">警告 (Warning)</SelectItem>
                 <SelectItem value="info">信息 (Info)</SelectItem>
               </SelectContent>
             </Select>
          </div>

          <div className="flex items-center gap-2 w-64">
            <Search size={14} className="text-[var(--bk-text-muted)]" />
            <Input 
              placeholder="搜索日志..." 
              className="h-8 border-[var(--bk-border)] bg-[var(--bk-surface-faint)] text-[11px] text-[var(--bk-text-primary)]"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button 
            variant="bento-outline"
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`h-8 gap-2 text-[11px] transition-all ${
              autoRefresh 
                ? 'border-[var(--bk-border-accent)] bg-[var(--bk-status-success-soft)] text-[var(--bk-status-success)] hover:bg-[color-mix(in_srgb,var(--bk-status-success-soft)_85%,white)]' 
                : 'bg-[var(--bk-surface-faint)] text-[var(--bk-text-muted)]'
            }`}
          >
            {autoRefresh ? <Pause size={12} /> : <Play size={12} />}
            {autoRefresh ? '自动刷新' : '已暂停'}
          </Button>
          
          <Button
             variant="bento-ghost"
             size="icon"
             className="h-8 w-8 text-[var(--bk-text-muted)] hover:bg-[var(--bk-surface-faint)]"
             onClick={() => setSnapshot(processedEvents)}
          >
            <RefreshCcw size={14} />
          </Button>
        </div>
      </header>

      {/* 主体日志流 - 奶油色 Pod 风格 */}
      <div className="flex-1 min-h-0 relative">
        <div className="absolute inset-0 overflow-y-auto p-6 flex flex-col gap-3">
          {filteredEvents.length === 0 ? (
            <div className="flex h-96 flex-col items-center justify-center gap-3 text-[var(--bk-text-muted)] opacity-40">
              <Terminal size={64} />
              <p className="text-sm font-bold">没有匹配的日志项</p>
            </div>
          ) : (
            filteredEvents.map((log) => (
              <LogEntry key={log.id} log={log} />
            ))
          )}
        </div>
      </div>
    </div>
  );
}

function LogEntry({ log }: { log: ConsoleLogEvent }) {
  const levelShadows = {
    critical: "border-[var(--bk-status-danger)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_8%,transparent)] shadow-sm",
    warning: "border-[var(--bk-status-warning)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-warning)_10%,transparent)] shadow-sm",
    info: "border-[var(--bk-border)] bg-[var(--bk-surface-strong)] shadow-sm",
    debug: "border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)]",
  };

  const textColors = {
    critical: "text-[var(--bk-status-danger)]",
    warning: "text-[var(--bk-status-warning)]",
    info: "text-[var(--bk-text-primary)]",
    debug: "text-[var(--bk-text-muted)]",
  };

  const sourceIcons = {
    alert: <ShieldAlert size={16} className="text-[var(--bk-status-danger)]" />,
    notification: <Bell size={16} className="text-[var(--bk-status-success)]" />,
    timeline: <Activity size={16} className="text-[var(--bk-status-success)]" />,
    system: <Terminal size={16} className="text-[var(--bk-text-secondary)]" />,
  };

  const sourceLabels = {
    alert: "告警",
    notification: "通知",
    timeline: "执行时间线",
    system: "系统",
  };

  return (
    <div className={`group flex flex-col gap-2 p-4 rounded-[20px] border transition-all hover:translate-x-1 ${levelShadows[log.level]}`}>
      <div className="flex items-start justify-between gap-4">
        <div className="flex items-center gap-3">
           <div className="rounded-xl border border-inherit bg-[var(--bk-surface)]/85 p-2 shadow-inner">
             {sourceIcons[log.source]}
           </div>
           <div>
              <div className="flex items-center gap-2 mb-0.5">
                <span className="text-[9px] font-bold uppercase tracking-widest text-[var(--bk-text-muted)]">
                  {sourceLabels[log.source]}
                </span>
                <Badge variant="outline" className={`text-[8px] h-4 border-inherit px-1 py-0 font-mono ${textColors[log.level]}`}>
                  {log.level.toUpperCase()}
                </Badge>
              </div>
              <h4 className={`text-sm font-bold truncate ${textColors[log.level]}`}>
                {log.title}
              </h4>
           </div>
        </div>
        <span className="shrink-0 text-[10px] font-mono font-bold text-[var(--bk-text-muted)] opacity-60">
          {formatFullLogTime(log.eventTime)}
        </span>
      </div>
      
      <div className="pl-[44px]">
        <p className={`text-[11px] leading-relaxed break-words line-clamp-2 group-hover:line-clamp-none ${textColors[log.level]} opacity-90`}>
          {log.message}
        </p>

        {log.metadata && (
          <div className="mt-3 hidden max-h-48 overflow-y-auto rounded-xl border border-[var(--bk-border-soft)] bg-[color:color-mix(in_srgb,var(--bk-text-primary)_4%,transparent)] p-3 font-mono text-[10px] text-[var(--bk-text-secondary)] shadow-inner group-hover:block">
             {JSON.stringify(log.metadata, null, 2)}
          </div>
        )}
      </div>
    </div>
  );
}
