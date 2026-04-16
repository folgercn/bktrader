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
    <div className="flex flex-col h-full bg-[#f3f0e7]">
      {/* 顶部过滤器 - 采用熟悉的奶油色面板风格 */}
      <header className="px-6 py-4 border-b border-[#d8cfba] bg-[var(--panel)] backdrop-blur-md shrink-0 flex items-center justify-between gap-4 z-20 shadow-sm">
        <div className="flex items-center gap-4">
          <div className="flex items-center gap-2">
             <Filter size={14} className="text-[#687177]" />
             <Select value={logType} onValueChange={(val) => setLogType(val as any)}>
               <SelectTrigger className="w-32 h-8 text-[11px] border-[#d8cfba] bg-white/50 text-[#1f2328]">
                 <SelectValue placeholder="来源" />
               </SelectTrigger>
               <SelectContent className="bg-[#fffbf2] border-[#d8cfba]">
                 <SelectItem value="all">所有来源</SelectItem>
                 <SelectItem value="alert">异常告警</SelectItem>
                 <SelectItem value="notification">通知</SelectItem>
                 <SelectItem value="timeline">执行时间线</SelectItem>
                 <SelectItem value="system">系统日志</SelectItem>
               </SelectContent>
             </Select>
          </div>

          <div className="flex items-center gap-2">
             <Zap size={14} className="text-[#687177]" />
             <Select value={levelFilter} onValueChange={(val) => setLevelFilter(val as any)}>
               <SelectTrigger className="w-32 h-8 text-[11px] border-[#d8cfba] bg-white/50 text-[#1f2328]">
                 <SelectValue placeholder="等级" />
               </SelectTrigger>
               <SelectContent className="bg-[#fffbf2] border-[#d8cfba]">
                 <SelectItem value="all">所有等级</SelectItem>
                 <SelectItem value="critical">严重 (Critical)</SelectItem>
                 <SelectItem value="warning">警告 (Warning)</SelectItem>
                 <SelectItem value="info">信息 (Info)</SelectItem>
               </SelectContent>
             </Select>
          </div>

          <div className="flex items-center gap-2 w-64">
            <Search size={14} className="text-[#687177]" />
            <Input 
              placeholder="搜索日志..." 
              className="h-8 text-[11px] border-[#d8cfba] bg-white/50 text-[#1f2328] focus-visible:ring-[#0e6d60]"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <Button 
            variant="outline"
            size="sm"
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`h-8 text-[11px] gap-2 border-[#d8cfba] transition-all ${
              autoRefresh 
                ? 'bg-[#d9eee8] text-[#0e6d60] hover:bg-[#c9ded8]' 
                : 'bg-white/50 text-[#687177]'
            }`}
          >
            {autoRefresh ? <Pause size={12} /> : <Play size={12} />}
            {autoRefresh ? '自动刷新' : '已暂停'}
          </Button>
          
          <Button
             variant="ghost"
             size="icon"
             className="h-8 w-8 text-[#687177] hover:bg-white/50"
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
            <div className="flex flex-col items-center justify-center h-96 text-[#687177] gap-3 opacity-40">
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
    critical: "border-rose-200 bg-rose-50 shadow-sm",
    warning: "border-amber-200 bg-amber-50 shadow-sm",
    info: "border-[#d8cfba] bg-[#fff8ea] shadow-sm",
    debug: "border-zinc-200 bg-zinc-50",
  };

  const textColors = {
    critical: "text-rose-900",
    warning: "text-amber-900",
    info: "text-[#1f2328]",
    debug: "text-[#687177]",
  };

  const sourceIcons = {
    alert: <ShieldAlert size={16} className="text-rose-600" />,
    notification: <Bell size={16} className="text-[#0e6d60]" />,
    timeline: <Activity size={16} className="text-emerald-600" />,
    system: <Terminal size={16} className="text-zinc-600" />,
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
           <div className="p-2 rounded-xl bg-white/80 border border-inherit shadow-inner">
             {sourceIcons[log.source]}
           </div>
           <div>
              <div className="flex items-center gap-2 mb-0.5">
                <span className="text-[9px] font-bold uppercase tracking-widest text-[#687177]">
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
        <span className="shrink-0 text-[10px] font-mono font-bold text-[#687177] opacity-60">
          {formatFullLogTime(log.eventTime)}
        </span>
      </div>
      
      <div className="pl-[44px]">
        <p className={`text-[11px] leading-relaxed break-words line-clamp-2 group-hover:line-clamp-none ${textColors[log.level]} opacity-90`}>
          {log.message}
        </p>

        {log.metadata && (
          <div className="mt-3 text-[10px] font-mono p-3 bg-black/5 rounded-xl border border-black/5 text-zinc-700 hidden group-hover:block max-h-48 overflow-y-auto shadow-inner">
             {JSON.stringify(log.metadata, null, 2)}
          </div>
        )}
      </div>
    </div>
  );
}
