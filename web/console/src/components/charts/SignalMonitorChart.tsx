import { useEffect, useRef } from 'react';
import { Time, CandlestickSeries, ColorType, CrosshairMode, LineStyle, createChart, createSeriesMarkers } from 'lightweight-charts';
import { SignalBarCandle, SessionMarker } from '../../types/domain';
import { applyDefaultChartWindow } from '../../utils/derivation';

export function SignalMonitorChart(props: { candles: SignalBarCandle[]; markers: SessionMarker[] }) {
  const containerRef = useRef<HTMLDivElement | null>(null);
  const chartRef = useRef<ReturnType<typeof createChart> | null>(null);
  const seriesRef = useRef<ReturnType<any> | null>(null);
  const markersRef = useRef<ReturnType<any> | null>(null);
  const isInitialRender = useRef(true);

  useEffect(() => {
    if (!containerRef.current) return;

    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 360,
      layout: {
        background: { type: ColorType.Solid, color: "rgba(255, 251, 242, 0.20)" },
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
        barSpacing: 10,
        minBarSpacing: 4,
        rightOffset: 6,
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

    chartRef.current = chart;
    seriesRef.current = series;
    markersRef.current = createSeriesMarkers(series, []);

    return () => {
      chart.remove();
      chartRef.current = null;
      seriesRef.current = null;
      markersRef.current = null;
    };
  }, []);

  useEffect(() => {
    const chart = chartRef.current;
    const series = seriesRef.current;
    const markers = markersRef.current;
    if (!chart || !series || !markers || props.candles.length === 0) return;

    series.setData(
      props.candles.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
      }))
    );

    markers.setMarkers(
      props.markers.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
        position: item.position,
        color: item.color,
        shape: item.shape,
        text: item.text,
      }))
    );

    if (isInitialRender.current) {
      applyDefaultChartWindow(chart, props.candles.length, 90);
      isInitialRender.current = false;
    }
  }, [props.candles, props.markers]);

  return <div ref={containerRef} className="tv-chart" />;
}
