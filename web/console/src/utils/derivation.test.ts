import { describe, expect, it } from "vitest";
import { deriveRuntimeMarketSnapshot, deriveSessionMarkers, deriveSignalMonitorDecorations, markerText, mergeLivePriceIntoSignalBars } from "./derivation";
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

describe("live monitor candles", () => {
  it("updates the active bar with the latest runtime trade price", () => {
    const candles: SignalBarCandle[] = [
      { ...candle("2026-04-24T00:00:00Z"), high: 101, low: 99, close: 100, timeframe: "5" },
    ];

    const merged = mergeLivePriceIntoSignalBars(candles, 105, "5", "2026-04-24T00:03:10Z");

    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({
      time: "2026-04-24T00:00:00Z",
      high: 105,
      low: 99,
      close: 105,
      isClosed: false,
    });
  });

  it("appends a current bar when REST candles are behind runtime trade data", () => {
    const candles: SignalBarCandle[] = [
      { ...candle("2026-04-24T00:00:00Z"), close: 76000, timeframe: "5" },
    ];

    const merged = mergeLivePriceIntoSignalBars(candles, 77100, "5", "2026-04-24T00:05:02Z");

    expect(merged).toHaveLength(2);
    expect(merged[1]).toMatchObject({
      time: "2026-04-24T00:05:00.000Z",
      open: 76000,
      high: 77100,
      low: 76000,
      close: 77100,
      isClosed: false,
    });
  });

  it("keeps trade price paired with its own source timestamp when order book is newer", () => {
    const market = deriveRuntimeMarketSnapshot(
      {
        trade: {
          streamType: "trade_tick",
          symbol: "BTCUSDT",
          summary: { price: "77100" },
          lastEventAt: "2026-04-24T00:00:10Z",
        },
        book: {
          streamType: "order_book",
          symbol: "BTCUSDT",
          summary: { bestBid: "77090", bestAsk: "77110" },
          lastEventAt: "2026-04-24T00:05:10Z",
        },
      },
      { event: "depth", bestBid: "77090", bestAsk: "77110" },
      "BTCUSDT"
    );
    const candles: SignalBarCandle[] = [
      { ...candle("2026-04-24T00:00:00Z"), close: 76000, timeframe: "5" },
    ];

    const merged = mergeLivePriceIntoSignalBars(candles, market.tradePrice, "5", market.tradePriceAt);

    expect(market).toMatchObject({
      tradePrice: 77100,
      tradePriceAt: "2026-04-24T00:00:10Z",
      bestBid: 77090,
      bestBidAt: "2026-04-24T00:05:10Z",
      bestAsk: 77110,
      bestAskAt: "2026-04-24T00:05:10Z",
    });
    expect(merged).toHaveLength(1);
    expect(merged[0]).toMatchObject({
      time: "2026-04-24T00:00:00Z",
      close: 77100,
    });
  });

  it("reads trade price from source summary when last event summary is not a trade", () => {
    const market = deriveRuntimeMarketSnapshot(
      {
        trade: {
          streamType: "trade_tick",
          symbol: "BTCUSDT",
          summary: { price: "77100" },
          lastEventAt: "2026-04-24T00:00:10Z",
        },
      },
      { event: "depth", bestBid: "77090", bestAsk: "77110" },
      "BTCUSDT"
    );

    expect(market.tradePrice).toBe(77100);
    expect(market.tradePriceAt).toBe("2026-04-24T00:00:10Z");
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
