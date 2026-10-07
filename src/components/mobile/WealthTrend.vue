<template>
    <div class="wealth-trend">
        <div class="wealth-heading">
            <h2 v-if="!expanded">资产走势</h2>
            <label class="wealth-metric"><span class="wealth-dot" /><select :value="metric" aria-label="资产走势指标" @change="emit('update:metric', ($event.target as HTMLSelectElement).value as WealthMetric)"><option value="net">净资产</option><option value="valued">已估值资产</option></select><f7-icon f7="chevron_down" /></label>
            <button v-if="!expanded" class="wealth-expand" aria-label="放大资产走势" @click="emit('expand')"><f7-icon f7="arrow_up_left_arrow_down_right" /></button>
        </div>
        <p v-if="error" class="wealth-empty">{{ error }}</p>
        <p v-else-if="!rows.length" class="wealth-empty">此期间还没有资产快照<span>记录历史估值后，走势会显示在这里</span></p>
        <template v-else>
            <div class="wealth-reading"><strong>{{ ledgerMoney(latest?.amount, false) }}<small v-if="latest?.amount !== null">元</small></strong><span>{{ latest ? moment.unix(latest.at).tz(timeZone).format('M月D日 HH:mm') : '' }}</span></div>
            <StatisticsChart v-if="hasValues" :option="option" :height="expanded ? 340 : 196" />
            <p v-else class="wealth-empty">此期间暂无有效估值<span>缺失的报价或汇率不会按零计算</span></p>
            <div class="wealth-footer"><span>全部账户 · 人民币</span><span v-if="hasGaps">缺失处保留断点</span><span v-else-if="points.length === 1">仅有一个历史记录点</span><span v-else>触摸曲线查看金额</span></div>
        </template>
    </div>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import moment from 'moment-timezone';
import StatisticsChart from './StatisticsChart.vue';
import { useEnvironmentsStore } from '@/stores/environment.ts';
import type { WealthSnapshot } from '@/models/investment.ts';
import { wealthTrendPoints, wealthTrendChart, type WealthMetric } from '@/lib/wealth-trend.ts';
import { ledgerMoney } from '@/lib/ledger-display.ts';
const props = defineProps<{ rows: WealthSnapshot[]; metric: WealthMetric; from: number; to: number; timeZone: string; error?: string; expanded?: boolean }>();
const emit = defineEmits<{ 'update:metric': [value: WealthMetric]; expand: [] }>();
const environment = useEnvironmentsStore();
const points = computed(() => wealthTrendPoints(props.rows, props.metric, props.timeZone));
const latest = computed(() => points.value[points.value.length - 1]);
const hasValues = computed(() => points.value.some(point => point.amount !== null));
const hasGaps = computed(() => points.value.some(point => point.amount === null));
const option = computed(() => wealthTrendChart(points.value, { from: props.from, to: Math.max(props.from + 1, Math.min(props.to, moment().tz(props.timeZone).endOf('day').unix())), metric: props.metric, timeZone: props.timeZone, dark: !!environment.framework7DarkMode }));
</script>
<style scoped>
.wealth-heading{display:flex;align-items:center;gap:9px;min-height:34px}.wealth-heading h2{font-size:16px;font-weight:600;flex:1;margin:0}
.wealth-metric{display:flex;align-items:center;gap:5px;position:relative;font-size:11px;color:var(--cy-muted)}.wealth-dot{width:5px;height:5px;border-radius:50%;background:var(--cy-accent)}
.wealth-metric select{appearance:none;border:0;background:transparent;color:inherit;font:inherit;padding:9px 17px 9px 2px;min-height:36px;max-width:115px}.wealth-metric>.icon{position:absolute;right:0;pointer-events:none;font-size:9px}
.wealth-expand{border:0;background:none;color:var(--cy-muted);padding:9px 0 9px 6px;min-width:30px;min-height:36px}.wealth-expand .icon{font-size:15px}
.wealth-reading{display:flex;justify-content:space-between;align-items:baseline;gap:10px;margin:9px 2px 0}.wealth-reading strong{font-size:21px;font-weight:500;letter-spacing:-.5px;overflow-wrap:anywhere;font-variant-numeric:tabular-nums}.wealth-reading strong small{font-size:10px;font-weight:400;color:var(--cy-muted);margin-left:5px}.wealth-reading>span{font-size:10px;color:var(--cy-muted);white-space:nowrap}
.wealth-footer{display:flex;justify-content:space-between;gap:8px;color:var(--cy-muted);font-size:9px;line-height:1.7;margin:10px 2px 0;flex-wrap:wrap}
.wealth-empty{text-align:center;font-size:13px;color:var(--cy-muted);line-height:1.8;padding:45px 10px}.wealth-empty span{display:block;font-size:11px;margin-top:7px}
</style>
