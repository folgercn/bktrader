import { getNumber } from './derivation';

export function formatMoney(value?: number) {
  if (value == null) {
    return "--";
  }
  return new Intl.NumberFormat("en-US", {
    style: "currency",
    currency: "USD",
    maximumFractionDigits: 2,
  }).format(value);
}

export function formatSigned(value?: number) {
  if (value == null) {
    return "--";
  }
  const prefix = value > 0 ? "+" : "";
  return `${prefix}${formatMoney(value)}`;
}

export function formatPercent(value?: unknown) {
  const number = typeof value === "number" ? value : Number(value);
  if (!Number.isFinite(number)) {
    return "--";
  }
  return `${number >= 0 ? "+" : ""}${(number * 100).toFixed(2)}%`;
}

export function formatNumber(value?: number, digits = 2) {
  if (value == null) {
    return "--";
  }
  return value.toFixed(digits);
}

export function formatMaybeNumber(value: unknown) {
  const number = getNumber(value);
  if (number == null) {
    return "--";
  }
  return number.toFixed(2);
}

export function formatTime(value: string) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "--";
  }
  return parsed.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    fractionalSecondDigits: 3,
    hour12: false,
  } as any);
}

export function formatShortTime(value: Date) {
  return value.toLocaleString("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
  });
}

export function formatFullLogTime(value: number | string | Date) {
  const parsed = new Date(value);
  if (Number.isNaN(parsed.getTime())) {
    return "--";
  }

  const formatter = new Intl.DateTimeFormat("zh-CN", {
    month: "2-digit",
    day: "2-digit",
    hour: "2-digit",
    minute: "2-digit",
    second: "2-digit",
    hour12: false,
  });

  const parts = formatter.formatToParts(parsed);
  const find = (type: string) => parts.find((p) => p.type === type)?.value || "";
  
  // Format as: MM-DD HH:mm:ss
  const dateStr = `${find("month")}-${find("day")} ${find("hour")}:${find("minute")}:${find("second")}`;
  const ms = String(parsed.getMilliseconds()).padStart(3, "0");

  return `${dateStr}.${ms}`;
}

export function shrink(value: unknown) {
  const text = String(value ?? "").trim();
  if (text === "") {
    return "--";
  }
  return text.length > 16 ? `${text.slice(0, 8)}...${text.slice(-4)}` : text;
}

