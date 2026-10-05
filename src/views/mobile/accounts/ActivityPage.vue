<template>
 <f7-page class="cy-mobile-surface cy-asset-surface cy-account-activity" @page:afterin="load">
  <f7-navbar :title="title" back-link="账户"><f7-nav-right><f7-link v-if="mode==='export'" icon-f7="square_arrow_down" aria-label="导出流水" :disabled="loading" @click="exportCSV"/></f7-nav-right></f7-navbar>
  <main class="cy-page-body">
   <p v-if="error" class="cy-message" role="alert">{{error}}</p>
   <section class="activity-controls"><label>起始日期<input v-model="from" type="date" aria-label="起始日期"/></label><label>结束日期<input v-model="to" type="date" aria-label="结束日期"/></label></section>
   <p v-if="loading" class="cy-empty">正在读取账户流水…</p>
   <template v-else>
    <section class="cy-panel activity-totals"><div>流入<strong>{{ledgerMoney(inflows,false)}}</strong></div><div>流出<strong>{{ledgerMoney(outflows,false)}}</strong></div><div>{{account?.currency}} 余额<strong>{{ledgerMoney(new LedgerDecimal(account?.balance||0).div(100).toString(),false)}}</strong></div></section>
    <template v-if="mode==='balance'"><svg v-if="points.length>1" class="balance-chart" viewBox="0 0 320 128" aria-label="账户余额变化"><polyline :points="points" fill="none" stroke="var(--cy-accent)" stroke-width="2"/></svg><section class="cy-panel activity-list"><f7-link v-for="row in filtered" :key="row.id" :href="`/transaction/detail?id=${row.id}`"><span>{{row.title}}<small>{{moment.unix(row.time).tz(scope.timeZone).format('YYYY-MM-DD HH:mm')}}</small></span><span>{{signed(row.delta)}}<small>余额 {{ledgerMoney(row.balance,false)}}</small></span></f7-link></section></template>
    <template v-else-if="mode==='export'"><section class="cy-panel export-description"><h2>{{account?.name}}</h2><p>已选择 {{filtered.length}} 条流水。导出包含日期、收支、转账、余额、分类和备注。</p><button @click="exportCSV">导出 CSV</button></section></template>
    <LedgerDayList v-else :entries="filtered" account-view/>
    <p v-if="!filtered.length" class="cy-empty">这个日期范围内没有记录</p>
   </template>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue';
import type {Router} from 'framework7/types';
import moment from 'moment-timezone';
import services from '@/lib/services.ts';
import {investmentError} from '@/lib/investments.ts';
import {LedgerDecimal,ledgerMoney,useMobileLedger,keepUpToDate,type LedgerEntry} from '@/lib/mobile-ledger.ts';
import {startDownloadFile} from '@/lib/ui/common.ts';
import {useAccountsStore} from '@/stores/account.ts';
import {useTransactionCategoriesStore} from '@/stores/transactionCategory.ts';
import {useTransactionTagsStore} from '@/stores/transactionTag.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
import {useBooksStore} from '@/stores/books.ts';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
const props=defineProps<{f7route:Router.Route}>();
const accounts=useAccountsStore(),categories=useTransactionCategoriesStore(),tags=useTransactionTagsStore(),scope=useLedgerScopeStore(),books=useBooksStore();
const id=computed(()=>props.f7route.query['id']||''),mode=computed(()=>props.f7route.query['mode']||'flow');
const title=computed(()=>({balance:'余额变动',flow:'资产明细',transfers:'转账记录',repayments:'还款记录',export:'流水导出'}[mode.value]||'资产明细'));
const account=computed(()=>accounts.allAccountsMap[id.value]);
const {normalize}=useMobileLedger({accountId:()=>id.value,applyFilters:()=>false});
type Row=LedgerEntry&{balance:string;delta:string};
const rows=ref<Row[]>([]),loading=ref(false),error=ref(''),from=ref(props.f7route.query['from']||''),to=ref(props.f7route.query['to']||moment().tz(scope.timeZone).format('YYYY-MM-DD'));
const filtered=computed(()=>rows.value.filter(r=>(!from.value||r.day>=from.value)&&(!to.value||r.day<=to.value)&&(mode.value!=='transfers'||r.type===4)&&(mode.value!=='repayments'||r.type===4&&r.transferDirection==='in')).slice().reverse());
const inflows=computed(()=>filtered.value.filter(r=>new LedgerDecimal(r.delta).gt(0)).reduce((s,r)=>s.plus(r.delta),new LedgerDecimal(0)).toString());
const outflows=computed(()=>filtered.value.filter(r=>new LedgerDecimal(r.delta).lt(0)).reduce((s,r)=>s.minus(r.delta),new LedgerDecimal(0)).toString());
const points=computed(()=>{const r=[...filtered.value].reverse();if(!r.length)return '';const amounts=r.map(x=>new LedgerDecimal(x.balance));const low=LedgerDecimal.min(...amounts),span=LedgerDecimal.max(...amounts).minus(low);return r.map((x,i)=>`${8+i*304/Math.max(1,r.length-1)},${span.isZero()?64:120-new LedgerDecimal(x.balance).minus(low).div(span).mul(112).toNumber()}`).join(' ');});
const signed=(v:string)=>`${new LedgerDecimal(v).gt(0)?'+':''}${ledgerMoney(v,false)}`;
async function load():Promise<void>{loading.value=true;error.value='';try{const [response]=await Promise.all([services.getAllTransactions({accountIds:id.value,startTime:0,endTime:0,bookIds:[]}),accounts.loadAllAccounts({force:true}).catch(keepUpToDate),categories.loadAllCategories({force:false}).catch(keepUpToDate),tags.loadAllTags({force:false}).catch(keepUpToDate),books.loadBooks()]);if(!response.data.success)throw Error('流水读取失败');let balance=new LedgerDecimal(0);rows.value=response.data.result.sort((a,b)=>new LedgerDecimal(a.timeSequenceId).cmp(b.timeSequenceId)).map(tx=>{const normalized=normalize(tx),before=balance,amount=new LedgerDecimal(tx.sourceAccountId===id.value?tx.sourceAmount:tx.destinationAmount).div(100);if(tx.type===1)balance=amount;else if(tx.type===2||tx.type===4&&tx.destinationAccountId===id.value)balance=balance.plus(amount);else balance=balance.minus(amount);return {...normalized,balance:balance.toString(),delta:balance.minus(before).toString()};});if(!from.value)from.value=rows.value[0]?.day||moment().tz(scope.timeZone).startOf('month').format('YYYY-MM-DD');}catch(e){error.value=investmentError(e);}finally{loading.value=false;}}
function exportCSV():void{if(loading.value)return;const csv=(values:string[])=>values.map(v=>'"'+(/^[=+\-@\t\r]/.test(v)?"'"+v:v).replace(/"/g,'""')+'"').join(',');const lines=[csv(['日期','账户','币种','类别','流入','流出','余额','报销账户','计入收支','备注']),...filtered.value.map(r=>csv([moment.unix(r.time).tz(scope.timeZone).format('YYYY-MM-DD HH:mm:ss'),account.value?.name||'',r.currency,r.title,new LedgerDecimal(r.delta).gt(0)?r.delta:'',new LedgerDecimal(r.delta).lt(0)?new LedgerDecimal(r.delta).abs().toString():'',r.balance,accounts.allAccountsMap[r.reimbursementAccountId||'']?.name||'',r.excludeFromStatistics||(r.reimbursementAccountId&&r.reimbursementAccountId!=='0')||![2,3].includes(r.type)?'否':'是',r.comment]))];startDownloadFile(`${(account.value?.name||'账户').replace(/[\\/:*?"<>|]/g,'_')}-流水.csv`,new Blob(['\ufeff'+lines.join('\r\n')],{type:'text/csv;charset=utf-8'}));}
</script>
<style scoped>
.activity-controls{display:flex;gap:12px;margin-bottom:16px}.activity-controls label{flex:1;min-width:0;font-size:12px;color:var(--cy-muted)}.activity-controls input{width:100%;background:var(--cy-card);border:0;border-radius:8px;padding:12px 8px;color:var(--cy-ink);margin-top:7px}.activity-totals{display:grid;grid-template-columns:repeat(3,1fr);border:0!important;border-radius:12px!important;padding:18px 12px!important;font-size:12px;color:var(--cy-muted);gap:8px}.activity-totals strong{font-size:17px;display:block;color:var(--cy-ink);font-weight:500;margin-top:8px;overflow-wrap:anywhere}.balance-chart{width:100%;height:150px;margin:8px 0 20px}.activity-list{padding:0 14px!important;border:0!important;border-radius:12px!important}.activity-list a{display:flex;justify-content:space-between;align-items:center;padding:14px 0;color:var(--cy-ink);font-size:14px;gap:12px}.activity-list a>span:last-child{text-align:right}.activity-list small{display:block;font-size:11px;color:var(--cy-muted);margin-top:7px}.export-description p{font-size:14px;color:var(--cy-muted);line-height:1.8;margin:16px 0}.export-description button{width:100%;padding:14px;border:0;border-radius:9px;background:var(--cy-accent);color:var(--cy-card)}
</style>
