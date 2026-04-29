import { readStoredAuthSession } from './auth';

export const API_BASE = (((import.meta as any).env.VITE_API_BASE as string | undefined) ?? "").replace(/\/$/, "");

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const authSession = readStoredAuthSession();
  const headers = new Headers(init?.headers ?? {});
  if (authSession?.token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${authSession.token}`);
  }
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers });
  if (!response.ok) {
    let message = `HTTP ${response.status} for ${path}`;
    try {
      const payload = (await response.json()) as { error?: string; message?: string } | null;
      if (payload?.error) {
        message = payload.error;
      } else if (payload?.message) {
        message = payload.message;
      }
    } catch {
      // ignore body parsing and fall back to status text
    }
    const error = new Error(message) as Error & { status?: number };
    error.status = response.status;
    throw error;
  }
  return (await response.json()) as T;
}

import { RemoteFillsResponse, ManualFillSyncResponse } from '../types/domain';

export async function fetchRemoteFills(orderId: string): Promise<RemoteFillsResponse> {
  return fetchJSON<RemoteFillsResponse>(`/api/v1/orders/${orderId}/remote-fills`, {
    method: 'GET',
  });
}

export async function manualSyncFills(orderId: string, req: { confirm: boolean; reason: string; dryRun: boolean }): Promise<ManualFillSyncResponse> {
  return fetchJSON<ManualFillSyncResponse>(`/api/v1/orders/${orderId}/sync-fills`, {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify(req),
  });
}
