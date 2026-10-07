<template>
 <f7-page class="cy-mobile-surface cy-asset-surface cy-installments" @page:afterin="load">
  <f7-navbar :title="editing?(preview?'分期预览':editing.id?'编辑分期':'新增分期'):'分期管理'" :back-link="editing?undefined:'账户'">
   <f7-nav-left v-if="editing"><f7-link icon-f7="xmark" :disabled="busy" @click="preview?preview=false:editing=null"/></f7-nav-left>
   <f7-nav-right><f7-link v-if="editing" icon-f7="checkmark" :aria-label="preview?'确认分期':'预览分期'" :disabled="busy" @click="preview?save():calculate()"/><f7-link v-else icon-f7="plus" aria-label="新增分期" @click="create()"/></f7-nav-right>
  </f7-navbar>
  <main class="cy-page-body">
   <p v-if="error" class="cy-message" role="alert">{{error}}</p>
   <template v-if="editing">
    <template v-if="!preview">
     <section class="installment-fields">
      <label>原始账单<select :disabled="!!editing.id" v-model="sourceKey" aria-label="选择分期账单" @change="selectSource"><option value="">请选择</option><optgroup label="每月账单"><option v-for="s in statements" :key="s.month" :value="`month:${s.month}`">{{s.month}} · 待还 {{ledgerMoney(s.remaining,false)}}</option></optgroup><optgroup label="单笔消费 / 借入"><option v-for="t in expenses" :key="t.id" :value="`expense:${t.id}`">{{moment.unix(t.time).tz(scope.timeZone).format('MM-DD')}} · {{t.comment||'账单'}} {{ledgerMoney(new LedgerDecimal(t.sourceAmount).div(100).toString(),false)}}</option></optgroup></select></label>
      <label>分期本金<input v-model="editing.principal" inputmode="decimal" maxlength="16" aria-label="分期本金" placeholder="输入分期金额"/></label>
      <label>总服务费<input v-model="editing.totalFee" inputmode="decimal" maxlength="16" aria-label="分期服务费" placeholder="0.00"/></label>
      <label>分期期数<input v-model.number="editing.periods" type="number" min="1" max="500" aria-label="分期期数"/></label>
      <label>第一期日期<input v-model="editing.firstDate" type="date" aria-label="第一期日期"/></label>
     </section>
     <section class="installment-fields">
      <label>分期方式<select v-model="editing.method" aria-label="分期方式"><option value="monthly">本金与服务费按月分摊</option><option value="first_fee">服务费首期收取</option><option value="immediate_fee">服务费立即收取</option><option value="balloon">本金最后一期偿还</option></select></label>
      <label>尾差处理<select v-model="editing.remainder" aria-label="尾差处理"><option value="first">尾差计入首期</option><option value="last">尾差计入末期</option><option value="except_first">除首期外均分</option><option value="except_last">除末期外均分</option></select></label>
      <label>备注<input v-model="editing.note" maxlength="200" aria-label="分期备注" placeholder="选填"/></label>
     </section>
     <p class="installment-hint">本金沿用原账单；确认分期后，服务费将在约定日期入账。实际还款时选择资金账户完成转账。</p>
    </template>
    <template v-else>
     <section class="installment-summary"><span>本金<strong>{{ledgerMoney(editing.principal,false)}}</strong></span><span>总服务费<strong>{{ledgerMoney(editing.totalFee,false)}}</strong></span><span>期数<strong>{{editing.periods}}</strong></span></section>
     <p class="installment-hint">核对每期日期与金额。可调整未到期项目，合计须与本金和总服务费一致。</p>
     <section class="installment-payments"><header><span>日期</span><span>本金</span><span>服务费</span></header><div v-for="(p,i) in editing.payments" :key="i"><input v-model="p.date" type="date" :disabled="p.accrued" :aria-label="`第${i+1}期日期`"/><input v-model="p.principal" inputmode="decimal" :disabled="p.accrued" maxlength="16" :aria-label="`第${i+1}期本金`"/><input v-model="p.fee" inputmode="decimal" :disabled="p.accrued" maxlength="16" :aria-label="`第${i+1}期服务费`"/></div></section>
     <button class="installment-save" :disabled="busy" @click="save">{{busy?'正在保存…':'确认分期计划'}}</button>
    </template>
   </template>
   <template v-else>
    <div class="installment-tabs"><button :class="{active:!closed}" @click="closed=false">进行中</button><button :class="{active:closed}" @click="closed=true">已结束</button></div>
    <p v-if="loading" class="cy-empty">正在读取分期…</p><p v-else-if="!shown.length" class="cy-empty">暂无分期记录</p>
    <section v-for="plan in shown" :key="plan.id" class="installment-record">
     <header><strong>{{plan.data.note||`${plan.statementMonth||'单笔账单'}分期`}}</strong><span>{{plan.data.periods}}期</span></header>
     <div class="installment-summary"><span>本金<strong>{{ledgerMoney(plan.data.principal,false)}}</strong></span><span>总服务费<strong>{{ledgerMoney(plan.data.totalFee,false)}}</strong></span><span>已到期<strong>{{plan.data.payments.filter(p=>p.accrued).length}}期</strong></span></div>
     <button class="installment-expand" @click="expanded=expanded===plan.id?'':plan.id">{{expanded===plan.id?'收起':'查看每期计划'}}<f7-icon f7="chevron_down"/></button>
     <template v-if="expanded===plan.id"><div v-for="(p,i) in plan.data.payments" :key="i" class="installment-payment"><span>{{p.date}}<small>{{p.accrued?'已到期':'未到期'}}<f7-link v-if="p.feeTransactionId" :href="`/transaction/detail?id=${p.feeTransactionId}&type=3`"> · 服务费流水</f7-link></small></span><strong>{{ledgerMoney(new LedgerDecimal(p.principal).plus(p.fee).toString(),false)}}<small>本金 {{ledgerMoney(p.principal)}} + 服务费 {{ledgerMoney(p.fee)}}</small></strong></div></template>
     <footer v-if="!plan.closed"><button @click="edit(plan)">编辑</button><button @click="close(plan)">结束分期</button><f7-link :href="repay(plan)">还款</f7-link></footer>
    </section>
   </template>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import {assetMoney as ledgerMoney} from '@/lib/asset-visibility.ts';
import {computed,ref} from 'vue';
import {f7} from 'framework7-vue';
import type {Router} from 'framework7/types';
import moment from 'moment-timezone';
import {creditAccounts,type CreditInstallment,type InstallmentInput,type CreditStatement} from '@/lib/credit-accounts.ts';
import {LedgerDecimal,keepUpToDate} from '@/lib/mobile-ledger.ts';
import {investmentError} from '@/lib/investments.ts';
import services from '@/lib/services.ts';
import type {TransactionInfoResponse} from '@/models/transaction.ts';
import {useAccountsStore} from '@/stores/account.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
import {useBooksStore} from '@/stores/books.ts';
const props=defineProps<{f7route:Router.Route}>(),accounts=useAccountsStore(),scope=useLedgerScopeStore(),books=useBooksStore();
const id=computed(()=>props.f7route.query['id']||''),account=computed(()=>accounts.allAccountsMap[id.value]);
const plans=ref<CreditInstallment[]>([]),statements=ref<CreditStatement[]>([]),expenses=ref<TransactionInfoResponse[]>([]),editing=ref<InstallmentInput|null>(null),preview=ref(false),sourceKey=ref(''),closed=ref(false),expanded=ref(''),busy=ref(false),loading=ref(false),error=ref('');
const shown=computed(()=>plans.value.filter(p=>p.accountId===id.value&&p.closed===closed.value));
let initialized=false;
async function load():Promise<void>{loading.value=true;error.value='';try{await creditAccounts.sync();await Promise.all([accounts.loadAllAccounts({force:true}).catch(keepUpToDate),books.loadBooks()]);const[p,r,t]=await Promise.all([creditAccounts.installments(),creditAccounts.reports(scope.timeZone,id.value),services.getAllTransactions({accountIds:id.value,startTime:0,endTime:0,bookIds:[]})]);plans.value=p;statements.value=(r[0]?.statements||[]).filter(s=>new LedgerDecimal(s.remaining).gt(0));if(!t.data.success)throw Error('原始账单读取失败');expenses.value=t.data.result.filter(tx=>tx.sourceAccountId===id.value&&[3,4].includes(tx.type));if(!initialized&&props.f7route.query['month']){create();sourceKey.value=`month:${props.f7route.query['month']}`;selectSource();}initialized=true;}catch(e){error.value=investmentError(e);}finally{loading.value=false;}}
function create():void{error.value='';preview.value=false;sourceKey.value='';editing.value={id:'',version:'',accountId:id.value,expenseId:'',statementMonth:'',principal:'',totalFee:'0',periods:12,firstDate:moment().tz(scope.timeZone).add(1,'month').format('YYYY-MM-DD'),method:'monthly',remainder:'first',bookId:books.defaultBookId,timeZone:scope.timeZone,note:'',requestId:crypto.randomUUID(),payments:[]};}
function selectSource():void{const e=editing.value;if(!e)return;const[k,v='']=sourceKey.value.split(':');e.expenseId=k==='expense'?v:'';e.statementMonth=k==='month'?v:'';e.payments=[];if(k==='month'){const s=statements.value.find(s=>s.month===v);e.principal=s?.remaining||'';if(s?.dueDate)e.firstDate=s.dueDate;}else{const t=expenses.value.find(t=>t.id===v);e.principal=t?new LedgerDecimal(t.sourceAmount).div(100).toString():'';e.bookId=t?.bookId||books.defaultBookId;}}
function edit(p:CreditInstallment):void{error.value='';editing.value={...JSON.parse(JSON.stringify(p.data)),id:p.id,version:p.version,accountId:p.accountId,expenseId:p.expenseId,statementMonth:p.statementMonth,requestId:crypto.randomUUID()};sourceKey.value=p.expenseId&&p.expenseId!=='0'?`expense:${p.expenseId}`:`month:${p.statementMonth}`;preview.value=true;}
async function calculate():Promise<void>{if(!editing.value||busy.value)return;busy.value=true;error.value='';try{const d=await creditAccounts.preview({...editing.value,payments:[]});editing.value={...editing.value,...d};preview.value=true;}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
async function save():Promise<void>{if(!editing.value||busy.value)return;busy.value=true;error.value='';try{await creditAccounts.save(editing.value);editing.value=null;await load();f7.toast.create({text:'分期计划已保存',closeTimeout:2000}).open();}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
function close(p:CreditInstallment):void{f7.dialog.confirm('结束后取消后续计划和未入账服务费，已入账的本金与服务费保留。','结束分期',async()=>{try{await creditAccounts.close(p.id,p.version);await load();}catch(e){error.value=investmentError(e);}});}
function repay(p:CreditInstallment):string{const source=accounts.allPlainAccounts.find(a=>[1,2,4,8].includes(a.category)&&!a.assetProfile.kind&&a.currency===account.value?.currency),next=p.data.payments.find(x=>x.date>=moment().tz(scope.timeZone).format('YYYY-MM-DD'))||p.data.payments[p.data.payments.length-1],amount=new LedgerDecimal(next?.principal||0).plus(next?.fee||0).mul(100).toFixed(0);return `/transaction/add?type=4&accountId=${source?.id||''}&destinationAccountId=${id.value}&amount=${amount}`;}
</script>
<style scoped>
.installment-fields,.installment-record,.installment-payments{background:var(--cy-card);border-radius:12px;padding:0 16px;margin-bottom:14px}.installment-fields label{display:flex;align-items:center;justify-content:space-between;gap:15px;min-height:53px;font-size:14px}.installment-fields input,.installment-fields select{min-width:0;flex:1;width:60%;text-align:right;background:none;border:0;color:var(--cy-ink);font:inherit}.installment-hint{font-size:12px;color:var(--cy-muted);line-height:1.8;margin:18px 6px}.installment-summary{display:flex;justify-content:space-between;gap:10px;padding:20px 0}.installment-summary span{font-size:12px;color:var(--cy-muted)}.installment-summary strong{display:block;color:var(--cy-ink);font-size:20px;font-weight:500;margin-top:8px}.installment-tabs{display:flex;gap:24px;padding:0 3px 18px}.installment-tabs button,.installment-record button{background:none;border:0;color:var(--cy-muted);font:inherit;padding:6px 0}.installment-tabs .active{color:var(--cy-ink);border-bottom:2px solid var(--cy-accent)}.installment-record{padding:17px}.installment-record header{display:flex;justify-content:space-between;gap:10px;font-size:14px}.installment-record header strong{font-weight:500}.installment-record header>span{color:var(--cy-muted)}.installment-record footer{display:flex;gap:24px;justify-content:flex-end;align-items:center;padding-top:12px;font-size:13px}.installment-record footer button{color:var(--cy-accent)}.installment-expand{width:100%;display:flex;justify-content:space-between;font-size:12px!important}.installment-expand .icon{font-size:12px}.installment-payment{display:flex;align-items:center;justify-content:space-between;gap:12px;padding:14px 0;font-size:13px}.installment-payment small{display:block;color:var(--cy-muted);font-size:10px;margin-top:6px}.installment-payment strong{text-align:right;font-weight:400}.installment-payments>header,.installment-payments>div{display:grid;grid-template-columns:1.5fr 1fr 1fr;gap:12px;padding:13px 0;font-size:12px}.installment-payments header{color:var(--cy-muted)}.installment-payments input{width:100%;min-width:0;border:0;background:none;color:var(--cy-ink);font:inherit}.installment-payments input:disabled{opacity:.5}.installment-save{width:100%;border:0;background:var(--cy-accent);color:var(--cy-card);padding:14px;border-radius:10px;font:inherit}
</style>
