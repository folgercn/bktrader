import { Time } from 'lightweight-charts';
import { SignalBarCandle } from '../types/domain';

/**
 * 规整图表 K 线数据：
 * 1. 转换时间格式为 Unix 秒级时间戳
 * 2. 去重：如果存在重复时间点，保留最后一条（通常是最新更新的）
 * 3. 排序：确保严格按时间升序排列
 */
export function normalizeChartData(candles: SignalBarCandle[]) {
  if (!candles || candles.length === 0) return [];

  const mappedData = candles.map((item) => ({
    time: Math.floor(new Date(item.time).getTime() / 1000) as Time,
    open: item.open,
    high: item.high,
    low: item.low,
    close: item.close,
  }));

  const uniqueMap = new Map<number, typeof mappedData[0]>();
  mappedData.forEach((item) => {
    uniqueMap.set(item.time as number, item);
  });

  return Array.from(uniqueMap.values()).sort(
    (a, b) => (a.time as number) - (b.time as number)
  );
}
