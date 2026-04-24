import { describe, expect, it } from "vitest";
import { normalizeChartData, normalizeLineSeriesRange } from "./chart";

describe("chart data normalization", () => {
  it("deduplicates candles by second and sorts ascending", () => {
    const result = normalizeChartData([
      { time: "2026-04-24T10:00:01.000Z", open: 1, high: 2, low: 1, close: 2, timeframe: "1m", isClosed: true },
      { time: "2026-04-24T10:00:00.000Z", open: 3, high: 4, low: 3, close: 4, timeframe: "1m", isClosed: true },
      { time: "2026-04-24T10:00:01.500Z", open: 5, high: 6, low: 5, close: 6, timeframe: "1m", isClosed: false },
    ]);

    expect(result).toEqual([
      { time: 1777024800, open: 3, high: 4, low: 3, close: 4 },
      { time: 1777024801, open: 5, high: 6, low: 5, close: 6 },
    ]);
  });

  it("drops zero-width line ranges before lightweight-charts setData", () => {
    expect(
      normalizeLineSeriesRange("2026-04-24T10:00:00.000Z", "2026-04-24T10:00:00.500Z", 100)
    ).toEqual([]);
  });

  it("keeps strictly ascending line ranges", () => {
    expect(
      normalizeLineSeriesRange("2026-04-24T10:00:00.000Z", "2026-04-24T10:00:01.000Z", 100)
    ).toEqual([
      { time: 1777024800, value: 100 },
      { time: 1777024801, value: 100 },
    ]);
  });
});
