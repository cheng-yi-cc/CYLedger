<template>
 <f7-page class="cy-main-page cy-mobile-surface cy-statistics" @page:afterin="refresh">
  <f7-navbar :title="listMode ? '全部账单' : title" back-link="统计"><f7-nav-right><f7-link v-if="!listMode" :href="link({list:'true'})">全部账单</f7-link></f7-nav-right></f7-navbar>
  <main class="cy-page-body">
   <p class="cy-muted detail-range">{{ bookLabel }} · {{ rangeLabel }}</p>
   <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh">重试</button></p><p v-else-if="loading" class="cy-empty">正在加载流水…</p>
   <template v-else>
    <section class="cy-panel"><div class="stat-three"><div><span>{{ metricLabel }}</span><strong :class="metric === 'expense' ? 'cy-expense' : 'cy-income'">{{ complete ? ledgerMoney(amount,false) : '—' }}</strong></div><div><span>日均{{ metricLabel }}</span><strong>{{ complete ? ledgerMoney(new LedgerDecimal(amount).div(days).toString(),false) : '—' }}</strong></div><div><span>账单笔数</span><strong>{{ filtered.length }}</strong></div></div></section>
    <p v-if="!complete" class="cy-message">包含未确认历史汇率的外币账单，合计保持未知。流水显示原币金额。</p>
    <template v-if="!listMode && !query['aux']">
     <section class="cy-panel"><div class="stat-heading"><h2>收支趋势</h2><div class="stat-controls"><div v-if="!query['type']" class="cy-segments stat-segment"><button :aria-pressed="metric === 'expense'" @click="metric = 'expense'">支出</button><button :aria-pressed="metric === 'income'" @click="metric = 'income'">收入</button><button :aria-pressed="metric === 'balance'" @click="metric = 'balance'">结余</button></div><button class="stat-small" @click="kind = kind === 'bar' ? 'line' : 'bar'">{{ kind === 'bar' ? '⌁' : '▥' }}</button></div></div><StatisticsChart :option="chart" @select="selectBucket" /><f7-link v-if="selectedBucket" class="stat-chart-link" :href="link({start:String(selectedBucket.from),end:String(selectedBucket.to),title:selectedBucket.key,list:'true'})">{{ selectedBucket.key }} · 查看账单 ›</f7-link></section>
     <section v-if="groups.items.length > 1" class="cy-panel"><div class="stat-heading"><h2>{{ query['tagId'] ? '标签分类' : query['accountId'] ? '账户分类' : '分类明细' }}</h2></div><f7-link v-for="(item,index) in groups.items" :key="item.id" class="stat-rank" :href="link({categoryId:item.id,primary:'false',title:item.name,metric,type:metric === 'balance' ? '' : metric === 'expense' ? '3' : '2'})"><div><i :style="{background:chartColors[index % chartColors.length]}" /><span>{{ item.name }}<small>{{ item.count }} 笔 · {{ item.percent === null ? '—' : item.percent+'%' }}</small></span><strong>{{ item.complete ? ledgerMoney(item.amount,false) : '—' }}</strong></div><div class="stat-progress"><i :style="{width:item.width+'%',background:chartColors[index % chartColors.length]}" /></div></f7-link></section>
    </template>
    <div class="stat-heading"><h2>{{ title }}账单</h2><select v-if="listMode" v-model="sort" aria-label="账单排序"><option value="newest">时间从新到旧</option><option value="oldest">时间从旧到新</option><option value="large">金额从大到小</option><option value="small">金额从小到大</option></select><f7-link v-else :href="link({list:'true'})">排序 / 全部 ›</f7-link></div>
    <template v-if="listMode"><LedgerDayList v-for="item in sorted" :key="item.id" :entries="[item]" :show-heading="true" /></template><LedgerDayList v-else :entries="filtered" />
    <p v-if="!filtered.length" class="cy-empty">所选条件下没有流水。</p>
   </template>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import StatisticsChart from '@/components/mobile/StatisticsChart.vue';
import { useMobileLedger, ledgerMoney, ledgerTotals, LedgerDecimal } from '@/lib/mobile-ledger.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { useStatisticsWorkspaceStore } from '@/stores/statisticsWorkspace.ts';
import { filterStatisticsEntries } from '@/lib/statistics-links.ts';
import { reportBuckets, groupReport, reportBreakdown, elapsedDays, type StatisticsMetric } from '@/lib/statistics-report.ts';
import { cashChart, chartColors } from '@/lib/statistics-charts.ts';
import { investmentError } from '@/lib/investments.ts';
const props=defineProps<{f7route:Router.Route}>();
const query=computed(()=>props.f7route.query as Record<string,string>),books=useBooksStore(),scope=useLedgerScopeStore(),tags=useTransactionTagsStore(),workspace=useStatisticsWorkspaceStore();
const bookIds=computed(()=>query.value['bookIds'] !== undefined ? query.value['bookIds'].split(',').filter(Boolean) : books.selectedBookIds);
const {entries,loading,error,load}=useMobileLedger({applyFilters:()=>false,bookIds:()=>bookIds.value});
const title=computed(()=>query.value['title']||'统计流水'),listMode=computed(()=>query.value['list']==='true');
const from=computed(()=>moment.unix(Number(query.value['start'])).tz(scope.timeZone)),to=computed(()=>moment.unix(Number(query.value['end'])).tz(scope.timeZone));
const rangeLabel=computed(()=>`${from.value.format('YYYY-MM-DD')} 至 ${to.value.format('YYYY-MM-DD')}`);
const bookLabel=computed(()=>bookIds.value.length?books.allBooks.filter(b=>bookIds.value.includes(b.id)).map(b=>b.name).join('、'):'全部账本');
const filtered=computed(()=>filterStatisticsEntries(entries.value,query.value,tags.allTransactionTagsMap,workspace.auxiliary));
const metric=ref<StatisticsMetric>(query.value['metric']==='balance'?'balance':query.value['type']==='2'?'income':'expense'),kind=ref<'bar'|'line'>('bar'),sort=ref('newest'),selectedKey=ref('');
const totals=computed(()=>ledgerTotals(filtered.value)),complete=computed(()=>filtered.value.every(item=>item.cny!==null));
const amount=computed(()=>query.value['aux']?filtered.value.reduce((sum,item)=>sum.plus(query.value['aux']==='discount'?item.discountAmount||'0':item.cny||'0'),new LedgerDecimal(0)).abs().toString():totals.value[metric.value]);
const metricLabel=computed(()=>query.value['aux']?'合计':metric.value==='balance'?'结余':metric.value==='expense'?'支出':'收入');
const days=computed(()=>elapsedDays(from.value,to.value,moment().tz(scope.timeZone)));
const buckets=computed(()=>reportBuckets(filtered.value,from.value,to.value,to.value.diff(from.value,'days')>90?'month':'day'));
const chart=computed(()=>cashChart(buckets.value,[metric.value],kind.value));
const selectedBucket=computed(()=>buckets.value.find(b=>b.key===selectedKey.value));
function selectBucket(value:{dataIndex:number}){selectedKey.value=buckets.value[value.dataIndex]?.key||'';}
const groups=computed(()=>reportBreakdown(groupReport(filtered.value,'category',{primary:false,type:metric.value==='balance'?undefined:metric.value==='expense'?3:2}),metric.value));
const sorted=computed(()=>[...filtered.value].sort((a,b)=>sort.value==='newest'?b.time-a.time:sort.value==='oldest'?a.time-b.time:(a.currency.localeCompare(b.currency)||new LedgerDecimal(a.amount).comparedTo(b.amount)*(sort.value==='large'?-1:1))));
function link(extra:Record<string,string>){return `/ledger/details?${new URLSearchParams({...query.value,...extra}).toString()}`;}
async function refresh(){const start=Number(query.value['start']),end=Number(query.value['end']);if(!Number.isSafeInteger(start)||!Number.isSafeInteger(end)||start<0||end<start){error.value='日期范围无效，请返回统计重新选择。';loading.value=false;return;}try{if(query.value['aux']==='debt'||query.value['aux']==='fees')await workspace.load(true);await load(start,end);}catch(cause){error.value=investmentError(cause);loading.value=false;}}
</script>
<style scoped src="@/styles/mobile/statistics.css"></style>
<style scoped>.detail-range{margin-bottom:14px}.stat-three strong{font-size:20px}.stat-heading{margin-top:15px}</style>
