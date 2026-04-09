import { useEffect, useRef } from 'react';
import { Time, CandlestickSeries, ColorType, CrosshairMode, LineStyle, createChart, createSeriesMarkers } from 'lightweight-charts';
import { ChartCandle, ChartAnnotation, MarkerDetail } from '../../types/domain';
import { markerPosition, markerColor, markerShape, markerText, findNearestAnnotation, toMarkerDetail } from '../../utils/derivation';

export function TradingChart(props: {
  candles: ChartCandle[];
  annotations: ChartAnnotation[];
  focusTime?: string;
  focusNonce: number;
  selectedAnnotationIds: string[];
  onSelectAnnotation: (annotation: ChartAnnotation) => void;
  onHoverMarker: (detail: MarkerDetail | null) => void;
}) {
  const containerRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    if (!containerRef.current || props.candles.length === 0) {
      return;
    }

    const chart = createChart(containerRef.current, {
      autoSize: true,
      height: 360,
      layout: {
        background: { type: ColorType.Solid, color: "rgba(255, 251, 242, 0.24)" },
        textColor: "#4f585d",
      },
      grid: {
        vertLines: { color: "rgba(216, 207, 186, 0.35)", style: LineStyle.Dotted },
        horzLines: { color: "rgba(216, 207, 186, 0.35)", style: LineStyle.Dotted },
      },
      crosshair: {
        mode: CrosshairMode.Normal,
      },
      rightPriceScale: {
        borderColor: "rgba(216, 207, 186, 0.9)",
      },
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

    series.setData(
      props.candles.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
        open: item.open,
        high: item.high,
        low: item.low,
        close: item.close,
      }))
    );

    const markers = createSeriesMarkers(
      series,
      props.annotations.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
        position: markerPosition(item.type),
        color: markerColor(item, props.selectedAnnotationIds.includes(item.id)),
        shape: markerShape(item.type),
        text: markerText(item, props.selectedAnnotationIds.includes(item.id)),
      }))
    );
    markers.setMarkers(
      props.annotations.map((item) => ({
        time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
        position: markerPosition(item.type),
        color: markerColor(item, props.selectedAnnotationIds.includes(item.id)),
        shape: markerShape(item.type),
        text: markerText(item, props.selectedAnnotationIds.includes(item.id)),
      }))
    );

    chart.subscribeCrosshairMove((param) => {
      if (param.time == null) {
        props.onHoverMarker(null);
        return;
      }
      const hoveredTime = Number(param.time);
      if (!Number.isFinite(hoveredTime)) {
        props.onHoverMarker(null);
        return;
      }

      const nearest = findNearestAnnotation(props.annotations, hoveredTime);
      props.onHoverMarker(nearest ? toMarkerDetail(nearest) : null);
    });

    chart.subscribeClick((param) => {
      if (param.time == null) {
        return;
      }
      const clickedTime = Number(param.time);
      if (!Number.isFinite(clickedTime)) {
        return;
      }
      const nearest = findNearestAnnotation(props.annotations, clickedTime);
      if (nearest) {
        props.onSelectAnnotation(nearest);
      }
    });

    if (props.focusTime && props.focusNonce > 0) {
      const focusSeconds = Math.floor(new Date(props.focusTime).getTime() / 1000);
      const firstSeconds = Math.floor(new Date(props.candles[0].time).getTime() / 1000);
      const lastSeconds = Math.floor(new Date(props.candles[props.candles.length - 1].time).getTime() / 1000);
      const span = Math.max(lastSeconds - firstSeconds, 60 * 60);
      const padding = Math.max(Math.floor(span / 6), 30 * 60);
      chart.timeScale().setVisibleRange({
        from: (focusSeconds - padding) as Time,
        to: (focusSeconds + padding) as Time,
      });
    } else {
      chart.timeScale().fitContent();
    }
    return () => {
      props.onHoverMarker(null);
      chart.remove();
    };
  }, [
    props.annotations,
    props.candles,
    props.focusNonce,
    props.focusTime,
    props.onHoverMarker,
    props.onSelectAnnotation,
    props.selectedAnnotationIds,
  ]);

  return <div ref={containerRef} className="tv-chart" />;
}
