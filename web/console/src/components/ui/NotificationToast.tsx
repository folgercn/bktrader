import React, { useEffect } from 'react';
import { useUIStore } from '../../store/useUIStore';
import { AlertCircle, CheckCircle, Info, X } from 'lucide-react';

/**
 * NotificationToast - 全局玻璃拟态通知组件
 * 遵循 Frontend-Design-System-Skill 规范
 */
export const NotificationToast: React.FC = () => {
  const notification = useUIStore(s => s.notification);
  const setNotification = useUIStore(s => s.setNotification);

  // 自动消失逻辑：5秒后自动清除通知
  useEffect(() => {
    if (notification) {
      const timer = setTimeout(() => {
        setNotification(null);
      }, 5000);
      return () => clearTimeout(timer);
    }
  }, [notification, setNotification]);

  if (!notification) return null;

  const { type, message } = notification;

  // 根据类型配置图标与配色
  const iconMap = {
    success: <CheckCircle className="text-emerald-400" size={20} />,
    error: <AlertCircle className="text-rose-400" size={20} />,
    info: <Info className="text-blue-400" size={20} />,
  };

  const styleMap = {
    success: 'border-emerald-500/20 bg-emerald-500/5 shadow-emerald-500/10',
    error: 'border-rose-500/20 bg-rose-500/5 shadow-rose-500/10',
    info: 'border-blue-500/20 bg-blue-500/5 shadow-blue-500/10',
  };

  const labelMap = {
    success: '操作成功 SUCCESS',
    error: '系统异常 ERROR',
    info: '系统提示 INFO',
  };

  return (
    <div className="fixed top-8 left-1/2 -translate-x-1/2 z-[10000] animate-in fade-in slide-in-from-top-6 duration-500">
      <div className={`
        flex items-start gap-4 p-4 pr-12 rounded-2xl border min-w-[360px] max-w-[520px]
        bg-zinc-900/60 backdrop-blur-2xl shadow-2xl
        ${styleMap[type]}
      `}>
        {/* 图标区域 */}
        <div className="mt-0.5 shrink-0">
          {iconMap[type]}
        </div>

        {/* 文字内容区域 */}
        <div className="space-y-1">
          <p className="text-[10px] font-bold uppercase tracking-[0.2em] opacity-50 mb-1">
            {labelMap[type]}
          </p>
          <p className="text-sm font-medium text-zinc-100 leading-relaxed font-sans">
            {message}
          </p>
        </div>

        {/* 手动关闭按钮 */}
        <button
          onClick={() => setNotification(null)}
          className="absolute right-3 top-3 p-1.5 rounded-xl text-white/10 hover:text-white/60 hover:bg-white/5 transition-all duration-300"
          aria-label="关闭通知"
        >
          <X size={16} />
        </button>

        {/* 底部进度条装饰 (可选) */}
        <div className="absolute bottom-0 left-0 right-0 h-[2px] overflow-hidden rounded-b-2xl">
          <div className={`h-full opacity-30 animate-progress-shrink origin-left ${
            type === 'success' ? 'bg-emerald-500' : type === 'error' ? 'bg-rose-500' : 'bg-blue-500'
          }`} style={{ animationDuration: '5000ms' }} />
        </div>
      </div>
    </div>
  );
};
