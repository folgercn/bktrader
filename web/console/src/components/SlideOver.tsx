import React, { useEffect, useState } from 'react';

export interface SlideOverProps {
  isOpen: boolean;
  onClose: () => void;
  title: React.ReactNode;
  subtitle?: React.ReactNode;
  children: React.ReactNode;
  widthClass?: string;
}

export function SlideOver({
  isOpen,
  onClose,
  title,
  subtitle,
  children,
  widthClass = "w-full max-w-md",
}: SlideOverProps) {
  // Use state to manage delayed unmount for exit animation
  const [shouldRender, setRender] = useState(isOpen);

  useEffect(() => {
    if (isOpen) setRender(true);
  }, [isOpen]);

  const handleAnimationEnd = () => {
    if (!isOpen) setRender(false);
  };

  if (!shouldRender) return null;

  return (
    <div className="fixed inset-0 z-50 overflow-hidden" aria-labelledby="slide-over-title" role="dialog" aria-modal="true">
      {/* Background backdrop */}
      <div 
        className={`absolute inset-0 bg-black/40 backdrop-blur-sm transition-opacity duration-500 ease-in-out ${isOpen ? 'opacity-100' : 'opacity-0'}`} 
        onClick={onClose} 
      />

      <div className="absolute inset-y-0 right-0 flex max-w-full pl-10">
        <div 
          className={`${widthClass} h-full transform transition-transform duration-500 ease-in-out ${isOpen ? 'translate-x-0' : 'translate-x-full'}`}
          onTransitionEnd={handleAnimationEnd}
        >
          <div className="flex h-full flex-col bg-zinc-900/80 backdrop-blur-2xl shadow-2xl shadow-black/50 border-l border-white/5 overflow-y-auto">
            {/* Header */}
            <div className="px-6 py-6 border-b border-white/5 flex items-start justify-between">
              <div>
                <h2 className="text-lg font-medium text-zinc-100" id="slide-over-title">{title}</h2>
                {subtitle && <p className="mt-1 text-sm text-zinc-400">{subtitle}</p>}
              </div>
              <button
                type="button"
                className="ml-3 flex h-7 items-center justify-center rounded-md text-zinc-400 hover:text-zinc-200 hover:bg-white/10 transition-colors px-2"
                onClick={onClose}
              >
                <span className="sr-only">Close panel</span>
                关闭
              </button>
            </div>
            
            {/* Content */}
            <div className="relative flex-1 px-6 py-6">
              {children}
            </div>
          </div>
        </div>
      </div>
    </div>
  );
}
