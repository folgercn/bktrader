import React from "react";

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
          "w-[min(960px,calc(100vw-2rem))] max-w-[min(960px,calc(100vw-2rem))] rounded-[30px] border-2 border-[var(--bk-border-strong)] bg-[var(--bk-surface-strong)] p-0 shadow-[var(--bk-shadow-card)]",
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
                className="h-8 rounded-xl px-3 text-xs font-bold"
                onClick={() => onOpenChange(false)}
              >
                关闭
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
    <div className="rounded-2xl border border-[var(--bk-border-soft)] bg-[var(--bk-surface-overlay)] px-4 py-3">
      <div className="flex flex-wrap items-center gap-x-3 gap-y-2 text-xs">{children}</div>
    </div>
  );
}

export function ModalMetaItem({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <div className="flex items-center gap-2">
      <span className="font-black uppercase tracking-wide text-[var(--bk-text-secondary)]">{label}</span>
      <span className="font-mono text-[var(--bk-text-muted)]">{value}</span>
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
        "grid gap-4",
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
      <Label className="ml-0.5 text-[11px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]">
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
    <label className="flex min-h-10 items-center justify-between rounded-2xl border border-[var(--bk-border)] bg-[var(--bk-surface-overlay)] px-3 py-2">
      <span className="text-[11px] font-black uppercase tracking-wide text-[var(--bk-text-secondary)]">
        {label}
      </span>
      <input
        type="checkbox"
        checked={checked}
        onChange={(event) => onChange(event.target.checked)}
        className="size-4 rounded border-[var(--bk-border-strong)] accent-[var(--bk-text-primary)]"
      />
    </label>
  );
}

export function ModalActions({ children }: { children: React.ReactNode }) {
  return (
    <div className="flex flex-col gap-2 border-t border-[var(--bk-border-soft)] pt-4 sm:flex-row sm:justify-end">
      {children}
    </div>
  );
}
