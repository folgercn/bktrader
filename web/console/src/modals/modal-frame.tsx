import React from "react";
import { LucideIcon } from "lucide-react";


import {
  Dialog,
  DialogContent,
  DialogDescription,
  DialogHeader,
  DialogTitle,
} from "../components/ui/dialog";
import { Button } from "../components/ui/button";
import { Label } from "../components/ui/label";
import { cn } from "../lib/utils";
import { X } from "lucide-react";
import { Input } from "../components/ui/input";
import {
  Select,
  SelectContent,
  SelectItem,
  SelectTrigger,
  SelectValue,
} from "../components/ui/select";


export function SettingsModalFrame({
  open,
  onOpenChange,
  kicker,
  title,
  description,
  children,
  className,
  contentClassName,
  showClose = true,
}: {
  open: boolean;
  onOpenChange: (open: boolean) => void;
  kicker: string;
  title: string;
  description?: string;
  children: React.ReactNode;
  className?: string;
  contentClassName?: string;
  showClose?: boolean;
}) {
  return (
    <Dialog open={open} onOpenChange={onOpenChange}>
      <DialogContent
        tone="bento"
        showCloseButton={false}
        className={cn(
          "w-[calc(100vw-2rem)] max-w-5xl overflow-hidden rounded-[30px] border-2 border-[var(--bk-border-strong)] bg-[var(--bk-surface-strong)] p-0 shadow-[var(--bk-shadow-card)]",
          className
        )}



      >
        <DialogHeader className="border-b border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] px-6 py-5">
          <div className="flex items-start justify-between gap-4">
            <div className="space-y-2">
              <p className="text-[11px] font-black uppercase tracking-[0.22em] text-[var(--bk-text-secondary)]">
                {kicker}
              </p>
              <DialogTitle className="text-xl font-black text-[var(--bk-text-primary)]">
                {title}
              </DialogTitle>
              {description ? (
                <DialogDescription className="max-w-2xl text-sm font-medium leading-relaxed text-[var(--bk-text-muted)]">
                  {description}
                </DialogDescription>
              ) : null}
            </div>
            {showClose ? (
              <Button
                type="button"
                variant="bento-ghost"
                className="size-10 shrink-0 rounded-2xl border border-[var(--bk-border-soft)] p-0 text-[var(--bk-text-muted)] hover:border-[var(--bk-border-strong)] hover:bg-[var(--bk-surface-strong)] hover:text-[var(--bk-text-primary)]"
                onClick={() => onOpenChange(false)}
              >
                <X size={20} strokeWidth={2.5} />
              </Button>
            ) : null}

          </div>
        </DialogHeader>
        <div className={cn("space-y-5 px-6 py-6", contentClassName)}>{children}</div>
      </DialogContent>
    </Dialog>
  );
}

export function ModalNotice({
  tone,
  children,
}: {
  tone: "error" | "success" | "info";
  children: React.ReactNode;
}) {
  const toneClassName =
    tone === "error"
      ? "border-[var(--bk-status-danger)]/20 bg-[color:color-mix(in_srgb,var(--bk-status-danger)_10%,transparent)] text-[var(--bk-status-danger)]"
      : tone === "success"
        ? "border-[var(--bk-status-success)]/25 bg-[color:color-mix(in_srgb,var(--bk-status-success)_10%,transparent)] text-[var(--bk-status-success)]"
        : "border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] text-[var(--bk-text-secondary)]";

  return (
    <div className={cn("rounded-2xl border px-4 py-3 text-sm font-medium", toneClassName)}>
      {children}
    </div>
  );
}

export function ModalMetaStrip({ children }: { children: React.ReactNode }) {
  return (
    <div className="rounded-[24px] border border-[var(--bk-border-soft)] bg-gradient-to-br from-[var(--bk-surface-overlay)] to-[var(--bk-surface-strong)] p-1.5">
      <div className="flex flex-wrap items-center gap-1">{children}</div>
    </div>
  );
}

export function ModalMetaItem({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-1 rounded-[18px] bg-[var(--bk-surface-strong)] px-4 py-2.5 shadow-sm">
      <span className="text-[9px] font-black uppercase tracking-[0.2em] text-[var(--bk-text-secondary)]">
        {label}
      </span>
      <span className="font-mono text-xs font-bold text-[var(--bk-text-primary)]">{value}</span>
    </div>
  );
}

export function ModalSectionHeader({ 
  icon: Icon, 
  title, 
  description 
}: { 
  icon?: LucideIcon; 
  title: string; 
  description?: string;
}) {
  return (
    <div className="mb-4 mt-2 flex items-center gap-3 px-1">
      {Icon && (
        <div className="flex size-10 items-center justify-center rounded-2xl bg-gradient-to-tr from-[var(--bk-bg-button-bento)]/10 to-[var(--bk-bg-button-bento)]/5 text-[var(--bk-bg-button-bento)]">
          <Icon size={20} strokeWidth={2.5} />
        </div>
      )}
      <div className="space-y-0.5">
        <h3 className="text-sm font-black uppercase tracking-wider text-[var(--bk-text-primary)]">
          {title}
        </h3>
        {description && (
          <p className="text-[11px] font-medium text-[var(--bk-text-muted)]">{description}</p>
        )}
      </div>
    </div>
  );
}

export function ModalGroup({ children, className }: { children: React.ReactNode; className?: string }) {
  return (
    <div className={cn(
      "rounded-[28px] border border-[var(--bk-border-soft)] bg-gradient-to-b from-[var(--bk-surface-overlay)] to-[var(--bk-surface-strong)]/30 p-5 shadow-[inset_0_1px_2px_rgba(255,255,255,0.05)]",
      className
    )}>
      {children}
    </div>
  );
}

export function ModalFormGrid({
  children,
  columns = "default",
}: {
  children: React.ReactNode;
  columns?: "default" | "wide";
}) {
  return (
    <div
      className={cn(
        "grid gap-x-8 gap-y-4",
        columns === "wide" ? "grid-cols-1 md:grid-cols-3" : "grid-cols-1 md:grid-cols-2"
      )}
    >

      {children}
    </div>
  );
}

export function ModalField({
  label,
  children,
  wide = false,
}: {
  label: string;
  children: React.ReactNode;
  wide?: boolean;
}) {
  return (
    <label className={cn("space-y-1.5", wide && "md:col-span-2")}>
      <Label className="ml-0.5 whitespace-nowrap text-[11px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]">
        {label}
      </Label>
      {children}
    </label>

  );
}

export function ModalCheckboxField({
  label,
  checked,
  onChange,
}: {
  label: string;
  checked: boolean;
  onChange: (checked: boolean) => void;
}) {
  return (
    <div className="flex flex-col space-y-1.5">
      <Label className="ml-0.5 whitespace-nowrap text-[11px] font-black uppercase tracking-wide text-transparent select-none">
        &nbsp;
      </Label>
      <label className="flex h-11 !h-11 items-center justify-between gap-4 rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-4 py-2 hover:bg-[var(--bk-surface)] transition-colors cursor-pointer group">
        <span className="whitespace-nowrap text-[11px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)] group-hover:text-[var(--bk-text-primary)]">
          {label}
        </span>
        <input
          type="checkbox"
          checked={checked}
          onChange={(event) => onChange(event.target.checked)}
          className="size-4 shrink-0 rounded border-[var(--bk-border-strong)] accent-[var(--bk-text-primary)]"
        />
      </label>
    </div>
  );


}

export function ModalActions({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2 border-t border-[var(--bk-border-soft)] pt-4 sm:flex-row sm:justify-end">
      {children}
    </div>
  );
}


export function ModalInput({ className, ...props }: React.ComponentProps<typeof Input>) {

  return (
    <Input
      className={cn(
        "h-11 !h-11 rounded-xl border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-4 transition-all focus:ring-1 focus:ring-[var(--bk-bg-button-bento)]",
        className
      )}
      {...props}
    />
  );
}

export function ModalSelect({
  value,
  onValueChange,
  options,
  placeholder,
  className,
}: {
  value: string;
  onValueChange: (value: string) => void;
  options: Array<{ value: string; label: string }>;
  placeholder?: string;
  className?: string;
}) {
  const EMPTY_SELECT_VALUE = "__empty__";
  const normalizedValue = value === "" ? EMPTY_SELECT_VALUE : value;

  return (
    <Select
      value={normalizedValue}
      onValueChange={(nextValue) =>
        onValueChange(nextValue === EMPTY_SELECT_VALUE ? "" : nextValue ?? "")
      }
    >
      <SelectTrigger
        tone="bento"
        className={cn("h-11 !h-11 w-full rounded-xl px-4", className)}
      >
        <SelectValue placeholder={placeholder} />
      </SelectTrigger>
      <SelectContent tone="bento" className="rounded-xl">
        {options.map((option) => (
          <SelectItem
            key={`${placeholder}-${option.value === "" ? EMPTY_SELECT_VALUE : option.value}`}
            value={option.value === "" ? EMPTY_SELECT_VALUE : option.value}
          >
            {option.label}
          </SelectItem>
        ))}
      </SelectContent>
    </Select>
  );
}
