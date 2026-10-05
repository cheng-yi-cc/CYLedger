import type { EChartsOption } from 'echarts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import type { ChartBucket, ReportGroup, StatisticsMetric } from '@/lib/statistics-report.ts';
export const chartColors = ['#ff666b', '#55bda5', '#edb54d', '#7096de', '#b08acf', '#dd8b69', '#64b9ce', '#9eaf68', '#d679a4', '#8495a4'];
export const metricLabels = { expense: '支出', income: '收入', balance: '结余' };
export const metricColors = { expense: '#ff626a', income: '#48b69d', balance: '#80a6cd' };
export function cashChart(buckets: ChartBucket[], metrics: StatisticsMetric[], kind: 'bar' | 'line', labels = false): EChartsOption {
    return { grid: { left: 42, right: 15, top: labels ? 40 : 28, bottom: buckets.length > 62 ? 58 : 28 },
        tooltip: { trigger: 'axis', confine: true },
        legend: { show: metrics.length > 1, top: 0, textStyle: { color: '#89979d', fontSize: 10 } },
        xAxis: { type: 'category', data: buckets.map(b => b.label), axisLabel: { fontSize: 10, color: '#89979d' }, axisTick: { show: false }, axisLine: { lineStyle: { color: '#71808755' } } },
        yAxis: { type: 'value', splitNumber: 3, axisLabel: { fontSize: 10, color: '#89979d' }, splitLine: { lineStyle: { color: '#71808722' } } },
        dataZoom: buckets.length > 62 ? [{ type: 'slider', height: 16, bottom: 8, startValue: Math.max(0, buckets.length - 31), endValue: buckets.length - 1 }] : [],
        series: metrics.map(metric => ({ name: metricLabels[metric], type: kind, data: buckets.map(b => b.complete ? new LedgerDecimal(b[metric]).toNumber() : null), itemStyle: { color: metricColors[metric], borderRadius: kind === 'bar' ? [3, 3, 0, 0] : 0 }, barMaxWidth: 18, connectNulls: false, symbolSize: 4, label: { show: labels, position: 'top', fontSize: 9, color: metricColors[metric] } })) };
}
export function shareChart(groups: ReportGroup[], metric: StatisticsMetric, labels: boolean): EChartsOption {
    const total = groups.reduce((sum, group) => sum.plus(group[metric]), new LedgerDecimal(0));
    const valid = groups.every(group => group.complete && new LedgerDecimal(group[metric]).gte(0)) && total.gt(0);
    return { color: chartColors, tooltip: { trigger: 'item', confine: true }, series: [{ type: 'pie', radius: ['40%', '62%'], center: ['50%', '50%'], avoidLabelOverlap: true,
        label: { show: true, fontSize: 10, color: '#89979d', formatter: labels ? '{b}\n{c}' : '{b}\n{d}%' }, labelLine: { length: 9, length2: 7 },
        data: valid ? groups.filter(group => new LedgerDecimal(group[metric]).gt(0)).map(group => ({ name: group.name, value: new LedgerDecimal(group[metric]).toNumber(), id: group.id })) : [] }] };
}
export function flowChart(groups: ReportGroup[]): EChartsOption {
    const income = groups.filter(g => new LedgerDecimal(g.income).gt(0)), expense = groups.filter(g => new LedgerDecimal(g.expense).gt(0));
    if (groups.some(g => !g.complete || new LedgerDecimal(g.income).lt(0) || new LedgerDecimal(g.expense).lt(0))) return { series: [] };
    const received = income.reduce((s, g) => s.plus(g.income), new LedgerDecimal(0)), spent = expense.reduce((s, g) => s.plus(g.expense), new LedgerDecimal(0));
    const data = [{ name: '收支', depth: 1, itemStyle: { color: '#9babaa' } }, ...income.map(g => ({ name: `收入·${g.name}`, depth: 0, itemStyle: { color: '#48b69d' } })), ...expense.map(g => ({ name: `支出·${g.name}`, depth: 2, itemStyle: { color: '#ff626a' } }))];
    const links = [...income.map(g => ({ source: `收入·${g.name}`, target: '收支', value: new LedgerDecimal(g.income).toNumber() })), ...expense.map(g => ({ source: '收支', target: `支出·${g.name}`, value: new LedgerDecimal(g.expense).toNumber() }))];
    if (received.lt(spent)) { data.push({ name: '超支', depth: 0, itemStyle: { color: '#ff626a' } }); links.push({ source: '超支', target: '收支', value: spent.minus(received).toNumber() }); }
    if (received.gt(spent)) { data.push({ name: '结余', depth: 2, itemStyle: { color: '#48b69d' } }); links.push({ source: '收支', target: '结余', value: received.minus(spent).toNumber() }); }
    return { tooltip: { trigger: 'item', confine: true }, series: [{ type: 'sankey', left: 10, right: 115, top: 20, bottom: 10, data: links.length ? data : [], links, nodeWidth: 7, nodeGap: 15, label: { color: '#89979d', fontSize: 10, width: 108, overflow: 'truncate' }, lineStyle: { color: 'gradient', opacity: .3 }, layoutIterations: 24 }] };
}
