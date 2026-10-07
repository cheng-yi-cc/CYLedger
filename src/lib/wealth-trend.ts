import moment from 'moment-timezone';
import type { EChartsOption } from 'echarts';
import type { WealthSnapshot } from '@/models/investment.ts';
import { LedgerDecimal, ledgerMoney } from '@/lib/ledger-display.ts';

export type WealthMetric = 'net' | 'valued';
export interface WealthTrendPoint { at: number; amount: string | null }

export function wealthTrendPoints(rows: WealthSnapshot[], metric: WealthMetric, timeZone: string): WealthTrendPoint[] {
    const points: WealthTrendPoint[] = [];
    const sorted = [...rows].sort((a, b) => a.recordedAt - b.recordedAt);
    for (const row of sorted) {
        const previous = points[points.length - 1];
        if (previous) {
            const nextDay = moment.unix(previous.at).tz(timeZone).startOf('day').add(1, 'day');
            // Missing calendar days remain gaps, including across DST boundaries.
            if (moment.unix(row.recordedAt).tz(timeZone).startOf('day').isAfter(nextDay)) {
                points.push({ at: nextDay.unix(), amount: null });
            }
        }
        points.push({ at: row.recordedAt, amount: row.invalidated || metric === 'net' && (!row.complete || row.netAssets === null)
            ? null : metric === 'net' ? row.netAssets : row.valuedAssets });
    }
    return points;
}

export function wealthTrendChart(points: WealthTrendPoint[], options: {
    from: number; to: number; metric: WealthMetric; timeZone: string; dark: boolean;
}): EChartsOption {
    const { from, to, metric, timeZone, dark } = options;
    const line = dark ? '#b4d4cb' : '#488e80', muted = dark ? '#94a6a4' : '#8b9697';
    const label = metric === 'net' ? '净资产' : '已估值资产';
    const longRange = moment.unix(to).diff(moment.unix(from), 'days') > 370;
    return {
        animation: false,
        grid: { left: 4, right: 12, top: 20, bottom: 12, containLabel: true },
        tooltip: {
            trigger: 'axis', confine: true, renderMode: 'richText', padding: [10, 12],
            backgroundColor: dark ? '#293c42' : '#ffffff', borderColor: dark ? '#425754' : '#e1ebe7', borderWidth: 1,
            textStyle: { color: dark ? '#e3ece9' : '#303637', fontSize: 12, lineHeight: 22 },
            axisPointer: { type: 'line', lineStyle: { color: line, width: 1, type: 'dashed', opacity: .65 } },
            formatter: parameters => {
                const item = Array.isArray(parameters) ? parameters[0] : parameters;
                const point = item && points[item.dataIndex];
                if (!point) return '';
                return `${moment.unix(point.at).tz(timeZone).format('YYYY年M月D日 HH:mm')}\n${label}  ${ledgerMoney(point.amount)}`;
            }
        },
        xAxis: {
            type: 'time', min: from * 1000, max: to * 1000, splitNumber: 5,
            axisLine: { lineStyle: { color: dark ? '#425552' : '#dce5e1' } }, axisTick: { show: false }, splitLine: { show: false },
            axisLabel: { color: muted, fontSize: 10, margin: 12, hideOverlap: true,
                formatter: (value: number) => moment(value).tz(timeZone).format(longRange ? 'YY/M' : 'M/D') }
        },
        yAxis: {
            type: 'value', scale: true, splitNumber: 3, axisLine: { show: false }, axisTick: { show: false },
            axisLabel: { color: muted, fontSize: 10, margin: 10,
                formatter: (value: number) => Math.abs(value) >= 1e8 ? `${+(value / 1e8).toFixed(1)}亿` : Math.abs(value) >= 1e6 ? `${+(value / 1e4).toFixed(1)}万` : value.toLocaleString('zh-CN', { maximumFractionDigits: 2 }) },
            splitLine: { lineStyle: { color: dark ? '#d0e4de0c' : '#738c8310', type: 'dashed' } }
        },
        series: [{
            type: 'line', name: label, smooth: .16, smoothMonotone: 'x', connectNulls: false,
            showSymbol: true, symbol: 'circle', symbolSize: (_value, params) => {
                const index = params.dataIndex;
                return (!points[index - 1] || points[index - 1]?.amount === null) && (!points[index + 1] || points[index + 1]?.amount === null) ? 6 : 0;
            },
            lineStyle: { color: line, width: 2.4, cap: 'round', join: 'round' },
            itemStyle: { color: line, borderColor: dark ? '#202e35' : '#fff', borderWidth: 2 },
            emphasis: { scale: 2, itemStyle: { color: line, borderWidth: 2 } },
            areaStyle: { color: { type: 'linear', x: 0, y: 0, x2: 0, y2: 1,
                colorStops: [{ offset: 0, color: dark ? '#6db5a030' : '#4b9e8733' }, { offset: 1, color: dark ? '#6db5a001' : '#4b9e8701' }] } },
            data: points.map(point => [point.at * 1000, point.amount === null ? null : new LedgerDecimal(point.amount).toNumber()])
        }]
    };
}
