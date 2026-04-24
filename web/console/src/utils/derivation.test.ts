import { describe, expect, it } from "vitest";
import { deriveSessionMarkers, deriveSignalMonitorDecorations, markerText } from "./derivation";
import { ChartAnnotation, Order, Position, SignalBarCandle } from "../types/domain";

describe("monitor chart marker labels", () => {
  it("distinguishes long and short entry/exit order markers", () => {
    const session = { id: "live-1", strategyId: "strategy-1" } as any;
    const orders: Order[] = [
      order("o1", "BUY", 100, false, "2026-04-24T00:00:00Z"),
      order("o2", "SELL", 101, false, "2026-04-24T00:01:00Z"),
      order("o3", "SELL", 102, true, "2026-04-24T00:02:00Z", "SL"),
      order("o4", "BUY", 103, true, "2026-04-24T00:03:00Z", "PT"),
    ];

    const markers = deriveSessionMarkers(session, orders, []);

    expect(markers.map((item) => item.text)).toEqual([
      "开多 100.00",
      "开空 101.00",
      "平多 SL 102.00",
      "平空 TP 103.00",
    ]);
  });

  it("adds direction to breakout and stop monitor decorations", () => {
    const candles: SignalBarCandle[] = [
      candle("2026-04-24T00:00:00Z"),
      candle("2026-04-24T00:01:00Z"),
      candle("2026-04-24T00:02:00Z"),
    ];
    const session = {
      id: "live-1",
      strategyId: "strategy-1",
      state: {
        breakoutHistory: [
          { barTime: "2026-04-24T00:00:00Z", level: 100, side: "BUY" },
          { barTime: "2026-04-24T00:01:00Z", level: 99, side: "SELL" },
        ],
        livePositionState: {
          found: true,
          side: "SHORT",
          stopLoss: 104,
          trailingStopActive: true,
        },
      },
    } as any;
    const position = {
      id: "position-1",
      accountId: "account-1",
      symbol: "BTCUSDT",
      side: "SHORT",
      quantity: 0.01,
      entryPrice: 101,
      markPrice: 100,
      updatedAt: "2026-04-24T00:02:00Z",
    } as Position;

    const { markers } = deriveSignalMonitorDecorations(session, candles, position, [], []);

    expect(markers.map((item) => item.text)).toEqual(["多 BO", "空 BO", "空 TSL"]);
  });

  it("adds direction to annotation marker labels when order side is available", () => {
    const annotation = {
      id: "a1",
      source: "live",
      type: "exit-pt",
      symbol: "BTCUSDT",
      time: "2026-04-24T00:00:00Z",
      price: 100,
      label: "PT",
      metadata: { orderSide: "SELL", reason: "PT" },
    } satisfies ChartAnnotation;

    expect(markerText(annotation)).toBe("平多 TP");
  });
});

function order(id: string, side: string, price: number, reduceOnly: boolean, createdAt: string, reason = ""): Order {
  return {
    id,
    accountId: "account-1",
    symbol: "BTCUSDT",
    side,
    type: "MARKET",
    status: "FILLED",
    quantity: 0.01,
    price,
    reduceOnly,
    metadata: {
      liveSessionId: "live-1",
      reduceOnly,
      executionProposal: {
        role: reduceOnly ? "exit" : "entry",
        reason,
        reduceOnly,
      },
    },
    createdAt,
  };
}

function candle(time: string): SignalBarCandle {
  return {
    time,
    open: 100,
    high: 101,
    low: 99,
    close: 100,
    timeframe: "1m",
    isClosed: true,
  };
}
