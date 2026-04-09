import { AuthSession } from '../types/domain';

export function readStoredAuthSession(): AuthSession | null {
  if (typeof window === "undefined") {
    return null;
  }
  const raw = window.localStorage.getItem("bktrader-console-auth");
  if (!raw) {
    return null;
  }
  try {
    return JSON.parse(raw) as AuthSession;
  } catch {
    return null;
  }
}

export function writeStoredAuthSession(session: AuthSession | null) {
  if (typeof window === "undefined") {
    return;
  }
  if (!session) {
    window.localStorage.removeItem("bktrader-console-auth");
    return;
  }
  window.localStorage.setItem("bktrader-console-auth", JSON.stringify(session));
}
