import { describe, expect, it } from "vitest";
import type { LiveSession } from "../types/domain";
import { mergeLiveSessionDetail, mergeLiveSessionSnapshot } from "./liveSessionDetail";

function liveSession(state: Record<string, unknown>): LiveSession {
  return {
    id: "live-session-1",
    alias: "",
    accountId: "account-1",
    strategyId: "strategy-1",
    status: "RUNNING",
    state,
    createdAt: "2026-05-18T07:00:00Z",
  };
}

describe("live session detail merging", () => {
  it("preserves loaded source-state bars across summary refreshes", () => {
    const sourceStates = {
      "binance-kline|signal|ETHUSDT|1h": {
        streamType: "signal_bar",
        symbol: "ETHUSDT",
        timeframe: "1h",
        bars: [
          {
            barStart: "1779080400000",
            open: "2122.07",
            high: "2123.59",
            low: "2116.65",
            close: "2116.67",
          },
        ],
      },
    };

    const withDetail = mergeLiveSessionDetail(
      [liveSession({ status: "summary" })],
      liveSession({ lastStrategyEvaluationSourceStates: sourceStates }),
      ["lastStrategyEvaluationSourceStates"]
    );
    const afterSummaryRefresh = mergeLiveSessionSnapshot(withDetail, [liveSession({ status: "summary-refresh" })]);

    expect(afterSummaryRefresh[0]?.state?.lastStrategyEvaluationSourceStates).toEqual(sourceStates);
  });
});
