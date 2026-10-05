<template>
 <f7-page class="cy-mobile-surface cy-asset-surface cy-credit-page" @page:afterin="load">
  <f7-navbar title="每期还款账单" back-link="账户"><f7-nav-right><f7-link :href="`/account/installments?id=${id}`">分期</f7-link></f7-nav-right></f7-navbar>
  <main class="cy-page-body">
   <p v-if="error" class="cy-message" role="alert">{{error}}</p>
   <template v-if="report">
    <section class="credit-summary"><span>{{report.accountName}} · 总待还</span><strong>{{ledgerMoney(report.outstanding,false)}}<small>{{report.currency}}</small></strong><f7-link v-if="new LedgerDecimal(report.outstanding).gt(0)" :href="repay(report.outstanding)">还款</f7-link></section>
    <section v-if="report.annualDate" class="credit-annual"><div><span>年费 {{ledgerMoney(report.annualFee||'0',false)}} · {{report.annualDate}}</span><strong>{{report.annualWaived?'已达到免年费条件':'免年费进度'}}</strong></div><p>本年度消费 {{ledgerMoney(report.annualSpend,false)}}，共 {{report.annualCount}} 笔</p><f7-link :href="`/account/edit?id=${id}`">查看年费条件 ›</f7-link></section>
    <div class="credit-tabs"><button v-for="f in filters" :key="f.id" :class="{active:filter===f.id}" @click="filter=f.id">{{f.name}}</button></div>
    <section v-for="row in statements" :key="row.month" class="credit-statement">
     <header><strong>{{row.month.replace('-','年')}}月账单</strong><span>{{row.unbilled?'未出账':new LedgerDecimal(row.remaining).isZero()?'已还清':'待还款'}}</span></header>
     <button class="credit-debt" @click="expanded=expanded===row.month?'':row.month"><span>剩余待还</span><strong>{{ledgerMoney(row.remaining,false)}}</strong><f7-icon f7="chevron_down"/></button>
     <div class="credit-meta"><span>{{row.startDate}} — {{row.endDate}}</span><span>{{row.dueDate?`${row.dueDate}还款`:'未设置还款日'}}</span></div>
     <p v-if="new LedgerDecimal(row.futureFees).gt(0)" class="credit-hint">另有尚未入账的分期服务费 {{ledgerMoney(row.futureFees,false)}}</p>
     <template v-if="expanded===row.month"><div class="credit-meta"><span>本期本金 {{ledgerMoney(row.charges,false)}}</span><span>已入账服务费 {{ledgerMoney(row.fees,false)}}</span></div><f7-link :href="`/account/activity?id=${id}&from=${row.startDate}&to=${row.endDate}`">查看本期流水 ›</f7-link></template>
     <footer v-if="new LedgerDecimal(row.remaining).gt(0)"><f7-link :href="`/account/installments?id=${id}&month=${row.month}&amount=${row.remaining}`">账单分期</f7-link><f7-link :href="repay(row.remaining)">还款</f7-link></footer>
    </section>
    <p v-if="!statements.length" class="cy-empty">暂无对应账单</p>
   </template><p v-else-if="loading" class="cy-empty">正在读取账单…</p>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue';
import type {Router} from 'framework7/types';
import {creditAccounts,type CreditReport} from '@/lib/credit-accounts.ts';
import {LedgerDecimal,ledgerMoney,keepUpToDate} from '@/lib/mobile-ledger.ts';
import {investmentError} from '@/lib/investments.ts';
import {useAccountsStore} from '@/stores/account.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
const props=defineProps<{f7route:Router.Route}>(),accounts=useAccountsStore(),scope=useLedgerScopeStore();
const id=computed(()=>props.f7route.query['id']||''),report=ref<CreditReport>(),loading=ref(false),error=ref(''),expanded=ref(''),filter=ref('unpaid');
const filters=[{id:'unpaid',name:'待还'},{id:'unbilled',name:'未出账'},{id:'all',name:'全部'}];
const statements=computed(()=>report.value?.statements.filter(row=>filter.value==='all'||(filter.value==='unbilled'?row.unbilled:!row.unbilled&&new LedgerDecimal(row.remaining).gt(0)))||[]);
function repay(amount:string):string{const source=accounts.allPlainAccounts.find(a=>[1,2,4,8].includes(a.category)&&!a.assetProfile.kind&&a.currency===report.value?.currency);return `/transaction/add?type=4&accountId=${source?.id||''}&destinationAccountId=${id.value}&amount=${new LedgerDecimal(amount).mul(100).toFixed(0)}`;}
async function load():Promise<void>{loading.value=true;error.value='';try{await creditAccounts.sync();await accounts.loadAllAccounts({force:true}).catch(keepUpToDate);report.value=(await creditAccounts.reports(scope.timeZone,id.value))[0];}catch(e){error.value=investmentError(e);}finally{loading.value=false;}}
</script>
<style scoped>
.credit-summary{padding:16px 8px 25px;position:relative}.credit-summary>span{font-size:13px;color:var(--cy-muted)}.credit-summary>strong{display:block;font-size:32px;font-weight:500;margin-top:12px}.credit-summary small{font-size:12px;font-weight:400;color:var(--cy-muted);margin-left:9px}.credit-summary>a{position:absolute;right:6px;bottom:27px;background:var(--cy-accent);color:var(--cy-card);border-radius:8px;padding:9px 19px}.credit-annual,.credit-statement{background:var(--cy-card);padding:16px;border-radius:12px;margin-bottom:14px}.credit-annual{font-size:12px;color:var(--cy-muted)}.credit-annual div{display:flex;justify-content:space-between;gap:12px}.credit-annual strong{color:var(--cy-accent);font-weight:400}.credit-tabs{display:flex;gap:22px;padding:8px 4px 17px}.credit-tabs button{background:none;border:0;color:var(--cy-muted);font:inherit;padding:5px 0}.credit-tabs .active{color:var(--cy-ink);border-bottom:2px solid var(--cy-accent)}.credit-statement header{display:flex;justify-content:space-between;font-size:14px;margin-bottom:23px}.credit-statement header strong{font-weight:500}.credit-statement header span{font-size:12px;color:var(--cy-muted)}.credit-debt{display:flex;align-items:center;gap:10px;width:100%;padding:0;border:0;background:none;color:var(--cy-ink);font:inherit}.credit-debt span{margin-right:auto;font-size:13px;color:var(--cy-muted)}.credit-debt strong{font-size:24px;font-weight:500}.credit-debt .icon{font-size:13px;color:var(--cy-muted)}.credit-meta{display:flex;justify-content:space-between;gap:10px;font-size:11px;color:var(--cy-muted);margin:17px 0 6px}.credit-hint{font-size:12px;color:var(--cy-muted)}.credit-statement footer{display:flex;justify-content:flex-end;gap:26px;padding-top:18px}.credit-statement a{font-size:13px}
</style>
