type WarnFn = (message?: unknown, ...optionalParams: unknown[]) => void;

export function isUnauthorizedDashboardError(err: unknown): boolean {
  return Boolean(typeof err === "object" && err && "status" in err && (err as { status?: number }).status === 401);
}

export async function fetchDashboardConfigItem<T>(
  label: string,
  load: () => Promise<T>,
  fallback: T,
  warn: WarnFn = console.warn
): Promise<T> {
  try {
    return await load();
  } catch (err) {
    if (isUnauthorizedDashboardError(err)) {
      throw err;
    }
    warn(`Failed to load dashboard config item: ${label}`, err);
    return fallback;
  }
}

export async function fetchDashboardConfigList<T>(
  label: string,
  load: () => Promise<T[]>,
  warn?: WarnFn
): Promise<T[]> {
  return fetchDashboardConfigItem(label, load, [], warn);
}
