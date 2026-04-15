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

  // Snapshot logic for autoRefresh
  const [snapshot, setSnapshot] = useState<ConsoleLogEvent[]>([]);

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
    <div className="flex flex-col h-full bg-zinc-950/20">
      {/* Top Filter Bar */}
      <header className="p-4 border-b border-white/5 bg-zinc-900/40 backdrop-blur-xl shrink-0 flex items-center justify-between gap-4 z-20">
        <div className="flex items-center gap-3">
          <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800/50 rounded-lg border border-white/5">
            <Filter size={14} className="text-zinc-500" />
            <select 
              className="bg-transparent text-xs text-zinc-300 outline-none"
              value={logType}
              onChange={(e) => setLogType(e.target.value as LogSource | "all")}
            >
              <option value="all">所有来源</option>
              <option value="alert">异常告警</option>
              <option value="notification">通知</option>
              <option value="timeline">执行时间线</option>
              <option value="system">系统日志</option>
            </select>
          </div>

          <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800/50 rounded-lg border border-white/5">
            <Zap size={14} className="text-zinc-500" />
            <select 
              className="bg-transparent text-xs text-zinc-300 outline-none"
              value={levelFilter}
              onChange={(e) => setLevelFilter(e.target.value as LogLevel | "all")}
            >
              <option value="all">所有等级</option>
              <option value="critical">严重 (Critical)</option>
              <option value="warning">警告 (Warning)</option>
              <option value="info">信息 (Info)</option>
            </select>
          </div>

          <div className="flex items-center gap-2 px-3 py-1.5 bg-zinc-800/50 rounded-lg border border-white/5 w-64">
            <Search size={14} className="text-zinc-500" />
            <input 
              type="text"
              placeholder="搜索日志内容..."
              className="bg-transparent text-xs text-zinc-300 outline-none w-full"
              value={searchQuery}
              onChange={(e) => setSearchQuery(e.target.value)}
            />
          </div>
        </div>

        <div className="flex items-center gap-2">
          <button 
            onClick={() => setAutoRefresh(!autoRefresh)}
            className={`flex items-center gap-2 px-4 py-1.5 rounded-lg text-xs font-medium transition-all ${
              autoRefresh 
                ? 'bg-emerald-500/10 text-emerald-400 border border-emerald-500/20' 
                : 'bg-zinc-800 text-zinc-400 border border-white/5'
            }`}
          >
            {autoRefresh ? <Pause size={14} /> : <Play size={14} />}
            {autoRefresh ? '自动刷新中' : '已暂停刷新'}
          </button>
          
          <button 
             onClick={() => setSnapshot(processedEvents)}
             className="p-1.5 text-zinc-400 hover:text-zinc-200 transition-colors"
             title="手动刷新"
          >
            <RefreshCcw size={16} />
          </button>
        </div>
      </header>

      {/* Main Content Areas */}
      <div className="flex-1 min-h-0 relative">
        <div className="absolute inset-0 overflow-y-auto p-4 flex flex-col gap-2">
          {filteredEvents.length === 0 ? (
            <div className="flex flex-col items-center justify-center h-64 text-zinc-600 gap-3">
              <Terminal size={48} className="opacity-20" />
              <p className="text-sm">没有匹配的数据项</p>
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
  const levelStyles = {
    critical: "border-rose-500/20 bg-rose-500/5 text-rose-400",
    warning: "border-amber-500/20 bg-amber-500/5 text-amber-400",
    info: "border-zinc-500/20 bg-zinc-500/5 text-zinc-400",
    debug: "border-zinc-700/20 bg-zinc-700/5 text-zinc-500",
  };

  const sourceIcons = {
    alert: <ShieldAlert size={14} />,
    notification: <Bell size={14} />,
    timeline: <Activity size={14} />,
    system: <Terminal size={14} />,
  };

  const sourceLabels = {
    alert: "告警",
    notification: "通知",
    timeline: "时间线",
    system: "系统",
  };

  return (
    <div className={`group flex items-start gap-4 p-3 rounded-xl border backdrop-blur-md transition-all hover:bg-white/5 ${levelStyles[log.level]}`}>
      <div className="shrink-0 mt-0.5">
        {sourceIcons[log.source]}
      </div>
      
      <div className="flex-1 min-w-0">
        <div className="flex items-center justify-between gap-4 mb-1">
          <div className="flex items-center gap-2">
            <span className="text-[10px] font-bold uppercase tracking-wider opacity-60">
              {sourceLabels[log.source]}
            </span>
            <h4 className="text-sm font-semibold truncate text-zinc-200">
              {log.title}
            </h4>
          </div>
          <span className="shrink-0 text-[11px] font-mono text-zinc-500">
            {formatFullLogTime(log.eventTime)}
          </span>
        </div>
        
        <p className="text-xs leading-relaxed text-zinc-400 break-words line-clamp-2 group-hover:line-clamp-none">
          {log.message}
        </p>

        {log.metadata && (
          <div className="mt-2 text-[10px] font-mono p-1.5 bg-black/20 rounded-md border border-white/5 text-zinc-600 hidden group-hover:block max-h-32 overflow-y-auto">
             {JSON.stringify(log.metadata, null, 2)}
          </div>
        )}
      </div>
    </div>
  );
}
