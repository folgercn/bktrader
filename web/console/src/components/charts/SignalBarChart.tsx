import { useEffect, useRef } from 'react';
import { Time, CandlestickSeries, ColorType, CrosshairMode, LineStyle, createChart } from 'lightweight-charts';
import { SignalBarCandle } from '../../types/domain';
import { applyDefaultChartWindow } from '../../utils/derivation';

export function SignalBarChart(props: { candles: SignalBarCandle[] }) {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!containerRef.current || props.candles.length === 0) {
      return;
    }

    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 260,
      layout: {
        background: { type: ColorType.Solid, color: "rgba(255, 251, 242, 0.16)" },
        textColor: "#4f585d",
      },
      grid: {
        vertLines: { color: "rgba(216, 207, 186, 0.25)", style: LineStyle.Dotted },
        horzLines: { color: "rgba(216, 207, 186, 0.25)", style: LineStyle.Dotted },
      },
      crosshair: { mode: CrosshairMode.Normal },
      rightPriceScale: { borderColor: "rgba(216, 207, 186, 0.9)" },
      timeScale: {
        borderColor: "rgba(216, 207, 186, 0.9)",
        timeVisible: true,
        secondsVisible: false,
      },
    });

    const series = chart.addSeries(CandlestickSeries, {
      upColor: "#0e6d60",
      downColor: "#b04a37",
      wickUpColor: "#0e6d60",
      wickDownColor: "#b04a37",
      borderVisible: false,
      priceLineVisible: true,
    });

    const mappedData = props.candles.map((item) => ({
      time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
      open: item.open,
      high: item.high,
      low: item.low,
      close: item.close,
    }));

    // 1. 去重：如果有重复时间戳，保留最后出现的（通常是最新更新的）
    const uniqueMap = new Map<number, typeof mappedData[0]>();
    mappedData.forEach((item) => {
      uniqueMap.set(item.time as number, item);
    });

    // 2. 排序：确保严格按时间升序
    const sortedData = Array.from(uniqueMap.values()).sort(
      (a, b) => (a.time as number) - (b.time as number)
    );

    series.setData(sortedData);

    applyDefaultChartWindow(chart, props.candles.length, 90);
    return () => chart.remove();
  }, [props.candles]);

  return <div ref={containerRef} className="tv-chart" />;
}
