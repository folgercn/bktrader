import { readStoredAuthSession } from './auth';

export const API_BASE = ((import.meta.env.VITE_API_BASE as string | undefined) ?? "").replace(/\/$/, "");

export async function fetchJSON<T>(path: string, init?: RequestInit): Promise<T> {
  const authSession = readStoredAuthSession();
  const headers = new Headers(init?.headers ?? {});
  if (authSession?.token && !headers.has("Authorization")) {
    headers.set("Authorization", `Bearer ${authSession.token}`);
  }
  
  const response = await fetch(`${API_BASE}${path}`, { ...init, headers });
  const contentType = response.headers.get("content-type") || "";
  const isJSON = contentType.includes("application/json");

  if (!response.ok) {
    let message = `HTTP ${response.status} for ${path}`;
    if (isJSON) {
      try {
        const payload = (await response.json()) as { error?: string; message?: string } | null;
        if (payload?.error) message = payload.error;
        else if (payload?.message) message = payload.message;
      } catch {
        // ignore body parsing error on non-ok response
      }
    }
    const error = new Error(message) as Error & { status?: number };
    error.status = response.status;
    throw error;
  }

  const text = await response.text();
  if (text.trim().length === 0) {
    return null as unknown as T;
  }

  if (!isJSON) {
     if (text.trim().toLowerCase().startsWith("<!doctype html") || text.trim().toLowerCase().startsWith("<html")) {
        throw new Error(`API 返回由于代理或后端配置错误导致的 HTML 页面而非 JSON 数据。请检查后端状态。`);
     }
  }

  try {
    return JSON.parse(text) as T;
  } catch (e) {
    throw new Error(`Failed to parse JSON response from ${path}: ${e instanceof Error ? e.message : "Malformed JSON"}`);
  }
}

