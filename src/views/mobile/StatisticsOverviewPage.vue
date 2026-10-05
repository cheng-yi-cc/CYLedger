<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="refresh">
        <f7-navbar title="统计"><f7-nav-right><f7-link href="/statistic/transaction">详细报表</f7-link></f7-nav-right></f7-navbar>
        <main class="cy-page-body">
            <details class="cy-stat-filters"><summary>账本与筛选<span>{{ books.scopeName }}</span></summary><BookScope /><LedgerFilters /></details>
            <nav class="cy-segments cy-periods" aria-label="统计周期"><button v-for="item in periods" :key="item.value" :aria-pressed="period === item.value" @click="period = item.value; refresh()">{{ item.name }}</button></nav>
            <div class="cy-period-inputs"><template v-if="period !== 'custom'"><button class="cy-icon-button" aria-label="上一周期" @click="shift(-1)">‹</button><input v-if="period === 'month'" v-model="month" type="month" aria-label="统计月份" @change="refresh" /><input v-else-if="period === 'day'" v-model="day" type="date" aria-label="统计日期" @change="refresh" /><input v-else v-model="year" type="number" min="1900" max="9999" aria-label="统计年份" @change="refresh" /><button class="cy-icon-button" aria-label="下一周期" @click="shift(1)">›</button></template><template v-else><label>起始<input v-model="start" type="date" aria-label="起始日期" @change="refresh" /></label><label>结束<input v-model="end" type="date" aria-label="结束日期" @change="refresh" /></label></template></div>
            <p v-if="rangeError || error" class="cy-message" role="alert">{{ rangeError || error }}</p>
            <template v-else>
                <section class="cy-panel cy-three-stats"><div><p class="cy-muted">{{ periodName }}支出</p><strong class="cy-expense">{{ display(totals.expense) }}</strong></div><div><p class="cy-muted">{{ periodName }}收入</p><strong class="cy-income">{{ display(totals.income) }}</strong></div><div><p class="cy-muted">{{ periodName }}结余</p><strong>{{ display(totals.balance) }}</strong></div></section>
                <p v-if="!totals.complete && !loading" class="cy-message">部分外币流水缺少已确认的历史汇率，以下合计暂不包含这些金额。</p>
<section class="cy-panel cy-share-panel">
 <div class="cy-section-head"><h2>{{ categoryType===3?'支出':'收入' }}占比</h2><div class="cy-segments"><button :aria-pressed="categoryType===3" @click="categoryType=3">支出</button><button :aria-pressed="categoryType===2" @click="categoryType=2">收入</button></div></div>
 <p v-if="loading" class="cy-empty" role="status">正在加载统计…</p>
 <template v-else>
  <div class="cy-donut-wrap"><svg viewBox="0 0 200 200" role="img" :aria-label="categoryType===3?'支出分类占比':'收入分类占比'">
   <circle cx="100" cy="100" r="72" fill="none" stroke="var(--cy-soft)" stroke-width="27" />
   <circle v-for="(item,index) in breakdown.canShare?breakdown.items:[]" :key="item.id" cx="100" cy="100" r="72" fill="none" :stroke="categoryColor(index)" stroke-width="27" pathLength="100" :stroke-dasharray="`${item.share} ${100-item.share}`" :stroke-dashoffset="-item.offset" transform="rotate(-90 100 100)"><title>{{ item.name }} {{ item.percent }}%</title></circle>
  </svg><div class="cy-donut-total"><span>{{ categoryType===3?'总支出':'总收入' }}</span><strong>{{ ledgerMoney(breakdown.total,false) }}</strong><small>元</small></div></div>
  <p v-if="!breakdown.items.length" class="cy-muted cy-chart-empty">这个周期暂无{{ categoryType===3?'支出':'收入' }}记录。</p>
  <p v-else-if="!breakdown.canShare" class="cy-muted">包含净退款，仅展示分类金额，不绘制占比。</p>
  <label class="cy-category-level"><input v-model="primary" type="checkbox" />按一级分类汇总</label>
 </template>
</section>
<section class="cy-panel cy-ranking">
 <div class="cy-section-head"><h2>{{ categoryType===3?'支出':'收入' }}数据</h2><f7-link :href="detailsLink({type:String(categoryType),title:categoryType===3?'支出明细':'收入明细'})">查看账单 ›</f7-link></div>
 <p v-if="!loading && !breakdown.items.length" class="cy-muted">暂无分类数据</p>
 <f7-link v-for="(item,index) in breakdown.items" :key="item.id" class="cy-rank-row" :href="detailsLink({categoryId:item.id,primary:String(primary),type:String(categoryType),title:item.name})">
  <div class="cy-rank-title"><i :style="{background:categoryColor(index)}" /><span>{{ item.name }}<small>{{ item.count }} 笔<template v-if="item.percent!==null"> · {{ item.percent }}%</template></small></span><strong :class="categoryType===3?'cy-expense':'cy-income'">{{ ledgerMoney(item.amount,false) }}</strong></div>
  <span class="cy-rank-track"><i :style="{width:`${item.width}%`,background:categoryColor(index)}" /></span>
 </f7-link>
</section>
<section class="cy-panel cy-daily-report"><div class="cy-section-head"><h2>报表统计</h2><span class="cy-muted">按日期</span></div>
 <div class="cy-report-row cy-report-head"><span>日期</span><span>收入</span><span>支出</span><span>结余</span></div>
 <p v-if="!loading && !dailyRows.length" class="cy-empty">这个周期暂无账单。</p>
 <f7-link v-for="row in shownDays" :key="row.day" class="cy-report-row" :href="dayDetailsLink(row.day)"><span>{{ period==='year'||period==='custom'?row.day:row.day.slice(5) }}</span><span class="cy-income">{{ ledgerMoney(row.income,false) }}</span><span class="cy-expense">{{ ledgerMoney(row.expense,false) }}</span><span :class="new LedgerDecimal(row.balance).isNegative()?'cy-expense':'cy-income'">{{ ledgerMoney(row.balance,false) }}{{ row.complete?'':'*' }}</span></f7-link>
 <button v-if="dailyRows.length>10" class="cy-report-expand" @click="showAllDays=!showAllDays">{{ showAllDays?'收起':'展开全部 '+dailyRows.length+' 天' }}</button>
</section>



                <details class="cy-secondary-chart"><summary>收支趋势</summary>                <section class="cy-panel"><div class="cy-section-head"><h2>收支统计</h2><div class="cy-segments"><button v-for="item in metrics" :key="item.value" :aria-pressed="metric === item.value" @click="metric = item.value">{{ item.name }}</button></div></div><p v-if="loading" class="cy-empty" role="status">正在加载统计…</p><template v-else><div class="cy-chart-caption"><span>{{ metricName }} · 人民币</span><span>{{ selectedBucket ? selectedBucket.label : '点击柱状图查看金额' }}</span></div><svg class="cy-cash-chart" viewBox="0 0 320 160" role="group" :aria-label="`${metricName}趋势，共${buckets.length}个统计区间`"><line x1="2" :y1="barBaseline" x2="318" :y2="barBaseline" stroke="var(--cy-line)" /><g v-for="(bucket,index) in buckets" :key="bucket.key" tabindex="0" role="button" :aria-label="`${bucket.label} ${metricName} ${ledgerMoney(bucket.value)}`" @click="selectedKey = bucket.key" @keydown.enter="selectedKey = bucket.key" @keydown.space.prevent="selectedKey = bucket.key"><title>{{ bucket.label }} {{ ledgerMoney(bucket.value) }}</title><rect :x="index * barWidth + 2" y="0" :width="barWidth" height="145" fill="transparent" /><rect :x="index * barWidth + 3" :y="bucket.negative ? barBaseline : barBaseline - bucket.height" :width="Math.max(2,barWidth - 4)" :height="bucket.height" rx="2" :fill="metric === 'expense' || bucket.negative ? 'var(--cy-expense)' : 'var(--cy-accent)'" /><text v-if="index % Math.max(1,Math.ceil(buckets.length / 7)) === 0" :x="index * barWidth + barWidth / 2" y="158" text-anchor="middle">{{ bucket.short }}</text></g></svg><p class="cy-chart-value">{{ selectedBucket ? `${selectedBucket.label} · ${ledgerMoney(selectedBucket.value)}` : `${metricName}合计 ${display(totals[metric])} 元` }}</p><f7-link v-if="selectedBucket" :href="bucketLink">查看这段时间的流水 ›</f7-link><p v-if="!entries.length" class="cy-muted">该周期暂无账单。</p></template></section></details><details class="cy-secondary-chart"><summary>净资产走势</summary>                <section class="cy-panel"><div class="cy-section-head"><h2>全部账户净资产走势</h2><f7-link href="/investments">查看资产</f7-link></div><p v-if="historyError" class="cy-message" role="alert">{{ historyError }}</p><p v-else-if="historyLoading" class="cy-empty">正在加载走势…</p><p v-else-if="!historyPoints.length" class="cy-empty">这个周期尚无完整的资产快照。</p><template v-else><div class="cy-chart-caption"><span>净资产 · 人民币</span><span>最新 {{ ledgerMoney(historyPoints[historyPoints.length - 1]?.netAssets) }}</span></div><svg class="cy-history-chart" viewBox="0 0 320 150" role="img" aria-label="所选周期的已记录净资产走势"><line x1="4" y1="125" x2="316" y2="125" stroke="var(--cy-line)" /><polyline v-for="(line,index) in historyLines" :key="index" :points="line" fill="none" stroke="var(--cy-accent)" stroke-width="2.5" stroke-linejoin="round" /><circle v-for="point in historyCoordinates" :key="point.id" :cx="point.x" :cy="point.y" r="3" fill="var(--cy-accent)"><title>{{ moment.unix(point.recordedAt).tz(scope.timeZone).format('MM-DD HH:mm') }} {{ ledgerMoney(point.netAssets) }}</title></circle><text x="4" y="146">{{ range?.from.format('MM-DD') }}</text><text x="316" y="146" text-anchor="end">{{ range?.to.format('MM-DD') }}</text></svg></template><p class="cy-muted">资产为全部账户的净值，不随账本筛选改变。仅显示已记录的快照，缺失估值处断开。</p></section></details><p class="cy-muted">账户互转、余额调整和投资结算不计入生活收支。退款按原分类冲减。</p>
            </template>
        </main><template #fixed><LedgerNavigation active="statistics" /></template>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { storeToRefs } from 'pinia';
import BookScope from '@/components/mobile/BookScope.vue';
import LedgerFilters from '@/components/mobile/LedgerFilters.vue';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { categoryBreakdown, dailyLedgerTotals } from '@/lib/ledger-statistics.ts';
import moment from 'moment-timezone';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import { LedgerDecimal, ledgerMoney, ledgerTotals, useMobileLedger } from '@/lib/mobile-ledger.ts';
import { investments, investmentError } from '@/lib/investments.ts';
import type { WealthSnapshot } from '@/models/investment.ts';
type Period = 'day' | 'month' | 'year' | 'custom';
type Metric = 'income' | 'expense' | 'balance';
const periods: {value: Period; name: string}[] = [{value:'day',name:'日常'},{value:'month',name:'月统计'},{value:'year',name:'年统计'},{value:'custom',name:'自定义'}];
const metrics: {value: Metric; name: string}[] = [{value:'expense',name:'支出'},{value:'income',name:'收入'},{value:'balance',name:'结余'}];
const period = ref<Period>('month'), metric = ref<Metric>('expense'), primary = ref(true);
const books = useBooksStore(), scope = useLedgerScopeStore();
const { month, selectedDay: day } = storeToRefs(scope);
const year = ref(moment().tz(scope.timeZone).format('YYYY'));
function date(value: string, format: string): moment.Moment { return moment.tz(value,format,true,scope.timeZone); }
const start = ref(moment().startOf('month').format('YYYY-MM-DD')), end = ref(moment().format('YYYY-MM-DD'));
const { entries, loading, error, load } = useMobileLedger();
const history = ref<WealthSnapshot[]>([]), historyError = ref(''), rangeError = ref(''), selectedKey = ref('');
const historyLoading = ref(true);
let requestNumber = 0;
const periodName = computed(() => ({day:'当日',month:'月',year:'年',custom:'区间'}[period.value]));
const metricName = computed(() => metrics.find(item => item.value === metric.value)!.name);
const range = computed(() => {
    const value = period.value === 'day' ? day.value : period.value === 'month' ? month.value : year.value;
    const format = period.value === 'day' ? 'YYYY-MM-DD' : period.value === 'month' ? 'YYYY-MM' : 'YYYY';
    const from = period.value === 'custom' ? date(start.value,'YYYY-MM-DD') : date(value,format).startOf(period.value);
    const to = period.value === 'custom' ? date(end.value,'YYYY-MM-DD').endOf('day') : from.clone().endOf(period.value);
    return from.isValid() && to.isValid() && from.isSameOrBefore(to) ? {from,to} : null;
});
const totals = computed(() => ledgerTotals(entries.value));
const monthly = computed(() => period.value === 'year' || (range.value?.to.diff(range.value.from,'days') || 0) > 62);
const rawBuckets = computed(() => {
    if (!range.value) return [];
    const unit = monthly.value ? 'month' : 'day';
    const cursor = range.value.from.clone().startOf(unit), items = [];
    while (cursor.isSameOrBefore(range.value.to)) {
        const key = cursor.format(monthly.value ? 'YYYY-MM' : 'YYYY-MM-DD');
        const aggregate = ledgerTotals(entries.value.filter(entry => entry.day.startsWith(key)));
        items.push({key,label:cursor.format(monthly.value ? 'YYYY年M月' : 'M月D日'),short:cursor.format(monthly.value ? 'M月' : 'D'),value:aggregate[metric.value]}); cursor.add(1,unit);
    }
    return items;
});
const barWidth = computed(() => 316 / Math.max(1,rawBuckets.value.length));
const hasNegative = computed(() => rawBuckets.value.some(item => new LedgerDecimal(item.value).isNegative()));
const barBaseline = computed(() => hasNegative.value ? 72 : 137);
const buckets = computed(() => {
    const maximum = rawBuckets.value.reduce((max,item) => LedgerDecimal.max(max,new LedgerDecimal(item.value).abs()),new LedgerDecimal(1));
    return rawBuckets.value.map(item => ({...item,negative:new LedgerDecimal(item.value).isNegative(),height:new LedgerDecimal(item.value).abs().div(maximum).mul(hasNegative.value ? 61 : 124).toNumber()}));
});
const selectedBucket = computed(() => buckets.value.find(item => item.key === selectedKey.value));
const periodHistory = computed(() => history.value.filter(item => range.value && item.recordedAt >= range.value.from.unix() && item.recordedAt <= range.value.to.unix()).sort((a,b) => a.recordedAt - b.recordedAt));
const historyPoints = computed(() => periodHistory.value.filter(item => item.complete && !item.invalidated && item.netAssets !== null));
const historyCoordinates = computed(() => {
    if (!historyPoints.value.length || !range.value) return [];
    const values = historyPoints.value.map(item => new LedgerDecimal(item.netAssets!));
    const minimum = LedgerDecimal.min(...values), maximum = LedgerDecimal.max(...values), span = maximum.minus(minimum);
    return historyPoints.value.map(item => ({...item,x:4 + (item.recordedAt - range.value!.from.unix()) / Math.max(1,range.value!.to.unix() - range.value!.from.unix()) * 312,y:span.isZero() ? 65 : 120 - new LedgerDecimal(item.netAssets!).minus(minimum).div(span).mul(105).toNumber()}));
});
const historyLines = computed(() => {
    const lines: string[] = []; let line: string[] = [];
    const points = new Map(historyCoordinates.value.map(point => [point.id,point]));
    for (const item of periodHistory.value) { const point = points.get(item.id); if (point) line.push(`${point.x},${point.y}`); else { if (line.length) lines.push(line.join(' ')); line = []; } }
    if (line.length) lines.push(line.join(' ')); return lines;
});
const categoryType=ref(3), showAllDays=ref(false);
const breakdown=computed(()=>categoryBreakdown(entries.value,primary.value,categoryType.value));
const dailyRows=computed(()=>dailyLedgerTotals(entries.value));
const shownDays=computed(()=>showAllDays.value?dailyRows.value:dailyRows.value.slice(0,10));
const categoryColors=['#439c95','#8ab8c2','#91ac7f','#d5ab70','#a799ba','#ca9297','#8c9ca8','#b8bb7f'];
function categoryColor(index:number):string{return categoryColors[index%categoryColors.length]!;}
function dayDetailsLink(day:string):string{const from=date(day,'YYYY-MM-DD');return detailsLink({title:day},from.unix(),from.clone().endOf('day').unix());}
function detailsLink(extra: Record<string,string> = {}, from = range.value?.from.unix(), to = range.value?.to.unix()): string {
    return '/ledger/details?' + new URLSearchParams({start:String(from),end:String(to),...extra}).toString();
}
const bucketLink = computed(() => {
    if (!selectedBucket.value) return '';
    const from = date(selectedBucket.value.key, monthly.value ? 'YYYY-MM' : 'YYYY-MM-DD');
    const to = from.clone().endOf(monthly.value ? 'month' : 'day');
    return detailsLink({title:selectedBucket.value.label},Math.max(from.unix(),range.value!.from.unix()),Math.min(to.unix(),range.value!.to.unix()));
});
watch(() => books.selectedBookIds.join(','), () => { void refresh(); });
watch(() => scope.timeZone, () => { void refresh(); });

function display(value: string): string { return loading.value ? '—' : ledgerMoney(value,false); }
async function refresh(): Promise<void> {
    if (!range.value) { rangeError.value = '请选择有效的日期范围，结束日期不能早于起始日期。'; return; }
    rangeError.value = ''; selectedKey.value = ''; showAllDays.value = false; historyError.value = ''; historyLoading.value = true; const version = ++requestNumber;
    await Promise.all([load(range.value.from.unix(),range.value.to.unix()),investments.history().then(items => { if (version === requestNumber) history.value = items; }).catch(cause => { if (version === requestNumber) { history.value = []; historyError.value = investmentError(cause); } }).finally(() => { if (version === requestNumber) historyLoading.value = false; })]);
}
function shift(direction: number): void { if (!range.value || period.value === 'custom') return; const shifted = range.value.from.clone().add(direction,period.value); if (period.value === 'day') day.value = shifted.format('YYYY-MM-DD'); else if (period.value === 'month') month.value = shifted.format('YYYY-MM'); else year.value = shifted.format('YYYY'); void refresh(); }
</script>
<style scoped>
.cy-periods{justify-content:space-between;margin-bottom:18px}.cy-periods button{flex:1}.cy-period-inputs{display:flex;gap:12px;align-items:center;margin-bottom:20px}.cy-period-inputs input{width:100%;min-width:0;flex:1;border:0;background:transparent;font-size:17px;padding:5px 0}.cy-period-inputs label{min-width:0;flex:1;color:var(--cy-muted);font-size:11px}.cy-period-inputs label input{display:block;color:var(--cy-ink);font-size:14px}.cy-three-stats strong{font-size:clamp(14px,4.3vw,19px)}.cy-section-head{flex-wrap:wrap}.cy-chart-caption{display:flex;justify-content:space-between;gap:8px;font-size:10px;color:var(--cy-muted);margin:4px 0 12px}.cy-cash-chart,.cy-history-chart{display:block;width:100%;overflow:visible}.cy-cash-chart text,.cy-history-chart text{font-size:9px;fill:var(--cy-muted)}.cy-chart-value{font-size:12px;color:var(--cy-muted);margin-top:14px!important}.cy-category-columns{display:grid;grid-template-columns:1fr 1fr;gap:22px}.cy-category-columns h3{font-size:14px;margin-bottom:18px}.cy-category-item{display:block;width:100%;color:var(--cy-ink);margin-bottom:16px;min-width:0}.cy-category-item>div{display:flex;flex-wrap:wrap;justify-content:space-between;gap:4px;font-size:11px;margin-bottom:7px}.cy-category-item strong{font-weight:500;overflow-wrap:anywhere}.cy-category-track{display:block;background:var(--cy-soft);height:5px;border-radius:5px;overflow:hidden}.cy-category-track i{display:block;height:100%;border-radius:5px}
.cy-stat-filters{margin:0 0 12px;font-size:12px}.cy-stat-filters>summary{display:flex;justify-content:space-between;gap:12px;cursor:pointer;padding:8px 0;color:var(--cy-muted);list-style:none}.cy-stat-filters>summary:before{content:'⌄';color:var(--cy-accent)}.cy-stat-filters>summary span{margin-left:auto}.cy-stat-filters[open]>summary{margin-bottom:12px}
.cy-periods{margin-bottom:12px}.cy-period-inputs{margin-bottom:14px}.cy-three-stats{padding:16px!important}.cy-share-panel .cy-section-head{margin-bottom:4px}
.cy-donut-wrap{position:relative;width:200px;height:200px;max-width:100%;margin:4px auto}.cy-donut-wrap svg{display:block;width:100%;height:100%}.cy-donut-total{position:absolute;inset:42px;display:flex;flex-direction:column;align-items:center;justify-content:center;gap:6px;pointer-events:none;min-width:0}.cy-donut-total span{font-size:12px;color:var(--cy-muted)}.cy-donut-total strong{font-size:clamp(15px,4.5vw,23px);max-width:100%;overflow-wrap:anywhere;text-align:center}.cy-donut-total small{font-size:11px;color:var(--cy-muted)}
.cy-category-level{display:flex;gap:8px;align-items:center;justify-content:flex-end;font-size:12px;color:var(--cy-muted);padding:8px 0 0}.cy-category-level input{width:17px;height:17px;accent-color:var(--cy-accent)}.cy-chart-empty{text-align:center}
.cy-rank-row{display:block;width:100%;color:inherit;padding:13px 0}.cy-rank-title{display:flex;align-items:center;gap:9px;font-size:14px}.cy-rank-title>i{width:10px;height:10px;border-radius:50%;flex-shrink:0}.cy-rank-title>span{min-width:0;flex:1;overflow-wrap:anywhere}.cy-rank-title small{display:block;font-size:11px;color:var(--cy-muted);margin-top:5px}.cy-rank-title strong{max-width:46%;overflow-wrap:anywhere;text-align:right;font-weight:550}.cy-rank-track{display:block;height:4px;border-radius:4px;background:var(--cy-soft);margin:9px 0 0 19px;overflow:hidden}.cy-rank-track i{display:block;height:100%;border-radius:4px}
.cy-report-row{display:grid;width:100%;grid-template-columns:1.05fr repeat(3,1fr);gap:8px;align-items:center;padding:13px 0;border-bottom:1px solid var(--cy-line);font-size:12px;color:inherit}.cy-report-row>span{min-width:0;text-align:right;overflow-wrap:anywhere}.cy-report-row>span:first-child{text-align:left}.cy-report-head{color:var(--cy-muted);font-size:11px}.cy-report-expand{width:100%;padding:12px;border:0;background:none;color:var(--cy-accent);font-size:12px}
.cy-secondary-chart{margin-bottom:14px}.cy-secondary-chart>summary{padding:16px;background:var(--cy-card);border:1px solid var(--cy-line);border-radius:12px;cursor:pointer;font-size:14px}.cy-secondary-chart[open]>summary{margin-bottom:12px}

</style>
