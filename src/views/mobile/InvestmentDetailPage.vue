<template>
  <f7-page class="cy-mobile-surface cy-investment-page" ptr @ptr:refresh="refresh" @page:afterin="activate" @page:beforeout="live.stop">
    <f7-navbar :title="selectedAccount?.name || '理财'" back-link="资产">
      <f7-nav-right>
        <f7-link class="inv-nav-icon" :href="`/investments/statistics?accountId=${accountId}`" aria-label="理财统计"><f7-icon f7="chart_pie" /></f7-link>
        <f7-link class="inv-nav-icon" aria-label="搜索理财" @click="showSearch=!showSearch"><f7-icon f7="search" /></f7-link>
        <f7-link @click="showAdd=true">新增</f7-link>
      </f7-nav-right>
    </f7-navbar>
    <main class="inv-body">
      <p v-if="error" role="alert" class="cy-message">{{ error }} <button @click="refresh()">重试</button></p>
      <div v-if="showSearch" class="inv-search"><input v-model="search" aria-label="搜索理财" placeholder="搜索名称或代码" /><button class="inv-link" @click="search='';showSearch=false">取消</button></div>
      <section class="inv-card inv-summary">
        <div><span>总资产（元）</span><strong>{{ money(total) }}</strong></div>
        <f7-link :href="`/investments/statistics?accountId=${accountId}`"><div><span>累计收益（元）</span><strong :class="profitClass(profit)">{{ money(profit) }}</strong></div></f7-link>
      </section>
      <f7-link v-if="pending.length" class="inv-card inv-list-button" :href="`/investments/plans?accountId=${accountId}&tab=orders`"><span>{{ pending.length }} 笔待确认</span><f7-icon f7="chevron_right" /></f7-link>
      <p v-if="loading&&!wealth" class="cy-empty">正在加载理财…</p>
      <details v-for="group in grouped" :key="group.name" class="inv-card" open>
        <summary class="inv-group-head"><span>{{ group.name }}</span><span>{{ money(group.value) }}</span><f7-icon f7="chevron_down" /></summary>
        <template v-for="row in group.rows" :key="row.key">
          <details v-if="row.members.length>1" class="inv-merged"><summary class="inv-holding"><InvestmentHoldingSummary :row="row" :members="row.members.length" /></summary><f7-link v-for="member in row.members" :key="member.key" class="inv-holding inv-member" :href="holdingLink(member)"><InvestmentHoldingSummary :row="member" :members="1" /></f7-link></details>
          <f7-link v-else class="inv-holding" :href="holdingLink(row)"><InvestmentHoldingSummary :row="row" :members="1" /></f7-link>
        </template>
      </details>
      <details v-if="monetaryRows.length" class="inv-card" open><summary class="inv-group-head"><span>货币基金</span><span>{{ money(investmentSum(monetaryRows.map(r=>r.value))) }}</span><f7-icon f7="chevron_down" /></summary><f7-link v-for="r in monetaryRows" :key="r.id" class="inv-holding" :href="`/account/detail?id=${r.id}`"><div class="inv-holding-top"><span>{{ r.binding.name }}</span><small class="inv-code">{{ r.binding.code }}</small><strong>{{ money(r.value) }}</strong></div><div class="inv-holding-info"><div><span class="inv-muted">已记录收益</span><span :class="profitClass(r.binding.totalIncome)">{{ money(r.binding.totalIncome) }}</span><span class="inv-muted">万份收益</span><span>{{ r.binding.lastPerTenThousand || '—' }}</span></div><div class="inv-muted">{{ r.name }} · {{ r.binding.enabled?'自动收益已启用':'自动收益已暂停' }}</div></div></f7-link></details>
      <section v-if="!loading&&!grouped.length&&!monetaryRows.length" class="inv-card cy-empty"><p>{{ search?'没有匹配的理财':'还没有理财记录' }}</p><button class="inv-link" @click="showAdd=true">新增一项理财</button></section>
      <nav class="inv-toolbar"><button class="inv-link" @click="showClosed=!showClosed">{{ showClosed?'收起已清仓':'查看已清仓' }}</button><f7-link href="/investments/plans">理财定投</f7-link><f7-link href="/investments/manage">管理</f7-link></nav>
    </main>
    <f7-sheet class="inv-sheet cy-mobile-surface cy-investment-page" v-model:opened="showAdd" swipe-to-close backdrop>
      <div class="inv-group-head"><span>新增理财</span><button class="inv-link" @click="showAdd=false">取消</button></div>
      <f7-link class="inv-list-button" :href="addLink('FUND')" sheet-close>基金<f7-icon f7="chevron_right" /></f7-link>
      <f7-link class="inv-list-button" href="/investments/add?type=MONETARY" sheet-close>基金（货币型）<f7-icon f7="chevron_right" /></f7-link>
      <f7-link v-for="type in investmentTypes.filter(t=>t.key!=='FUND')" :key="type.key" class="inv-list-button" :href="addLink(type.key)" sheet-close>{{ type.name }}<f7-icon f7="chevron_right" /></f7-link>
    </f7-sheet>
  </f7-page>
</template>
<script setup lang="ts">
import {computed,onUnmounted,ref} from 'vue';
import type {Router} from 'framework7/types';
import {LedgerDecimal,ledgerMoney} from '@/lib/ledger-display.ts';
import InvestmentHoldingSummary from '@/components/mobile/InvestmentHoldingSummary.vue';
import {groupInvestmentHoldings,type HoldingGroupRow} from '@/lib/investment-groups.ts';
import {createValuationRefresh} from '@/lib/valuation-refresh.ts';
import {useInvestmentData,investmentSum,investmentTypes,profitClass,type HoldingRow} from '@/lib/investment-mobile.ts';
const props=defineProps<{f7route:Router.Route}>();
const {monetary,wealth,accounts,orders,events,rows,books,loading,error,load}=useInvestmentData();
const accountId=computed(()=>props.f7route.query['accountId']||''),selectedAccount=computed(()=>accounts.value.find(a=>a.id===accountId.value));
const showAdd=ref(false),showSearch=ref(false),showClosed=ref(false),search=ref('');
const inScope=computed(()=>rows.value.filter(r=>(!accountId.value||r.position.accountId===accountId.value)&&!r.profile.hidden&&(!r.profile.bookIds?.length||!books.selectedBookIds.length||r.profile.bookIds.some(id=>books.selectedBookIds.includes(id)))));
const monetaryRows=computed(()=>accountId.value?[]:monetary.value.filter(b=>!books.selectedBookIds.length||books.selectedBookIds.includes(b.bookId)).flatMap(binding=>{const a=wealth.value?.cashAccounts.find(a=>a.id===binding.accountId);return a?[{...a,binding}]:[]}).filter(r=>!search.value||`${r.name} ${r.binding.name} ${r.binding.code}`.includes(search.value)));
const total=computed(()=>wealth.value?investmentSum(inScope.value.filter(r=>!r.profile.excludeFromTotal).map(r=>r.profile.excludeProfit?r.position.cost:r.position.marketValue).concat(monetaryRows.value.map(r=>r.value))):null);
const profit=computed(()=>wealth.value?investmentSum(inScope.value.map(r=>r.profit).concat(monetaryRows.value.map(r=>r.binding.totalIncome??null))):null);
const grouped=computed(()=>{const groups=new Map<string,HoldingGroupRow[]>();for(const r of groupInvestmentHoldings(inScope.value)){if(!showClosed.value&&new LedgerDecimal(r.position.quantity).isZero()&&events.value.some(e=>!e.voided&&e.accountId===r.position.accountId&&e.instrumentId===r.position.instrumentId))continue;if(search.value&&!`${r.name} ${r.asset?.symbol}`.toLowerCase().includes(search.value.toLowerCase()))continue;groups.set(r.group,[...(groups.get(r.group)||[]),r]);}return [...groups].map(([name,rs])=>({name,rows:rs,value:investmentSum(rs.map(r=>r.position.marketValue))}));});
const pending=computed(()=>orders.value.filter(o=>o.status==='pending'&&(!accountId.value||o.accountId===accountId.value)));
function money(v:string|null|undefined):string{return ledgerMoney(v,false);}
function holdingLink(row:HoldingRow):string{return '/investments/position?'+new URLSearchParams({accountId:row.position.accountId,instrumentId:row.position.instrumentId});}
function addLink(type:string):string{return '/investments/add?'+new URLSearchParams({type,accountId:accountId.value});}

async function refresh(done?:()=>void):Promise<void>{try{await load();}finally{if(typeof done==='function')done();}}
const live=createValuationRefresh(summary=>{wealth.value=summary;});
function activate():void{void load();live.start();}
onUnmounted(live.stop);
</script>

<style scoped>
.inv-merged>summary{list-style:none;cursor:pointer}.inv-merged>summary::-webkit-details-marker{display:none}.inv-member{padding-left:26px!important;background:var(--cy-soft);border-left:3px solid var(--cy-accent)}
</style>
