import { describe, expect, it, vi } from "vitest";
import {
  fetchDashboardConfigItem,
  fetchDashboardConfigList,
  isUnauthorizedDashboardError,
} from "./dashboardConfigFetch";

describe("dashboard config fetch helpers", () => {
  it("returns fallback for non-auth config failures", async () => {
    const warn = vi.fn();

    await expect(
      fetchDashboardConfigList("launch-templates", async () => {
        throw Object.assign(new Error("server unavailable"), { status: 500 });
      }, warn)
    ).resolves.toEqual([]);

    expect(warn).toHaveBeenCalledTimes(1);
  });

  it("rethrows 401 so auth expiry still logs out", async () => {
    const warn = vi.fn();
    const unauthorized = Object.assign(new Error("unauthorized"), { status: 401 });

    await expect(
      fetchDashboardConfigItem("signal-sources", async () => {
        throw unauthorized;
      }, { sources: [], notes: [] }, warn)
    ).rejects.toBe(unauthorized);

    expect(isUnauthorizedDashboardError(unauthorized)).toBe(true);
    expect(warn).not.toHaveBeenCalled();
  });
});
