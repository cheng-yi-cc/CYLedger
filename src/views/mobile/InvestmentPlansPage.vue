<template>
 <f7-page class="cy-mobile-surface cy-investment-page" @page:afterin="refresh">
  <f7-navbar title="理财定投" back-link="理财"><f7-nav-right><f7-link :disabled="busy" @click="newPlan">新增</f7-link></f7-nav-right></f7-navbar>
  <main class="inv-body">
   <p v-if="error" class="cy-message" role="alert">{{ error }}</p><p v-if="notice" class="cy-message" role="status">{{ notice }}</p>
   <nav class="inv-mode"><button :aria-pressed="tab==='plans'" @click="tab='plans'">定投计划</button><button :aria-pressed="tab==='orders'" @click="tab='orders'">待确认 {{ scopedOrders.filter(o=>o.status==='pending').length||'' }}</button></nav>
   <template v-if="tab==='plans'">
    <section v-for="p in scopedPlans" :key="p.id" class="inv-card"><div class="inv-event-line"><strong>{{ assetName(p.instrumentId) }}</strong><strong>{{ money(p.amount) }}</strong></div><p class="inv-muted" style="margin-top:9px">{{ cycleNames[p.cycle] }} · {{ p.time }} · {{ p.paused?'已暂停':p.endDate&&p.nextDate>p.endDate?'已结束':'下次 '+p.nextDate }}</p><div class="inv-toolbar"><button class="inv-link" @click="edit(p)">编辑</button><button class="inv-link" :disabled="busy" @click="changePlan(p,{paused:!p.paused})">{{ p.paused?'恢复':'暂停' }}</button><button class="inv-link" :disabled="busy" @click="removePlan(p)">删除</button></div></section>
    <section v-if="!loading&&!scopedPlans.length" class="inv-card cy-empty"><p>还没有定投计划</p><button class="inv-link" @click="newPlan">新增定投</button></section>
    <p class="inv-caption">按期使用基金已公布净值自动入账，份额保留两位小数。15:00后及非净值公布日顺延至下一公布日。应用重新打开时补齐到期记录；净值缺失或余额不足时保留待确认。</p>
   </template>
   <template v-else>
    <div class="inv-toolbar"><button class="inv-link" :disabled="busy" @click="sync">{{ busy?'正在确认…':'刷新并自动确认' }}</button><button class="inv-link" @click="showHistory=!showHistory">{{ showHistory?'只看待确认':'查看全部记录' }}</button></div>
    <section v-for="o in shownOrders" :key="o.id" class="inv-card"><div class="inv-event-line"><strong>{{ assetName(o.instrumentId) }}</strong><strong>{{ o.amount?money(o.amount):assetText(o.quantity)+' 份' }}</strong></div><p class="inv-muted" style="margin-top:8px">{{ o.tradeDate }} · {{ o.type==='BUY'?'买入':'卖出' }} · {{ statuses[o.status] }}</p><p v-if="o.error&&o.status==='pending'" class="inv-caption" style="padding:0;margin-bottom:0">{{ o.error }}</p><p v-if="o.priceDate" class="inv-muted">{{ o.priceDate }} 净值 {{ o.price }}</p><div v-if="o.status==='pending'" class="inv-toolbar"><button class="inv-link" @click="openConfirm(o)">填写实际净值</button><button class="inv-link" :disabled="busy" @click="cancel(o)">取消本期</button></div><f7-link v-else-if="o.eventId" class="inv-link" :href="`/investments/record?action=revise&eventId=${o.eventId}&accountId=${o.accountId}&instrumentId=${encodeURIComponent(o.instrumentId)}`">查看入账记录</f7-link></section>
    <p v-if="!loading&&!shownOrders.length" class="cy-empty">没有{{ showHistory?'':'待确认' }}记录</p>
    <p class="inv-caption">待确认记录尚未扣款或增加持仓。确认后将资金与份额一起入账；已取消的本期不会再次自动生成。</p>
   </template>
  </main>
  <f7-popup v-model:opened="showEditor" class="cy-mobile-surface cy-investment-page"><f7-page class="cy-mobile-surface cy-investment-page"><f7-navbar :title="draft.id?'编辑定投':'新增定投'"><f7-nav-left><f7-link @click="showEditor=false">取消</f7-link></f7-nav-left><f7-nav-right><f7-link :disabled="busy" @click="save">保存</f7-link></f7-nav-right></f7-navbar><main class="inv-body"><p v-if="formError" class="cy-message" role="alert">{{ formError }}</p><section class="inv-card inv-form">
   <label class="inv-row"><span>重复周期</span><select v-model="draft.cycle"><option v-for="(label,value) in cycleNames" :key="value" :value="value">{{ label }}</option></select></label>
   <label class="inv-row"><span>生效日期</span><input v-model="draft.startDate" type="date" required /></label><label class="inv-row"><span>结束日期</span><input v-model="draft.endDate" type="date" aria-label="结束日期，留空永不结束" /></label><label class="inv-row"><span>定投时间</span><input v-model="draft.time" type="time" required /></label>
  </section><section class="inv-card inv-form"><label class="inv-row"><span>买入基金</span><select v-model="selectedHolding" required><option value="" disabled>选择基金</option><option v-for="r in fundRows" :key="r.key" :value="r.key">{{ r.name }} · {{ r.account?.name }}</option></select></label><label class="inv-row"><span>付款账户</span><select v-model="draft.cashAccountId" required><option value="" disabled>选择账户</option><option v-for="a in cash" :key="a.id" :value="a.id">{{ a.name }}</option></select></label><label class="inv-row"><span>金额（元）</span><input v-model="draft.amount" inputmode="decimal" placeholder="每期支付总额" aria-label="每期金额" /></label><label class="inv-row"><span>手续费（%）</span><input v-model="draft.feePercent" inputmode="decimal" placeholder="默认0" aria-label="定投费率" /></label><label class="inv-row"><span>备注</span><input v-model="draft.note" maxlength="300" /></label><div class="inv-row"><span>记入账本</span><BookPicker v-model="draft.bookId" compact /></div></section><p class="inv-caption">金额包含手续费。结束日期留空表示永不结束；每月按生效日期对应的日子执行，短月份使用月末。会计时区：{{ zone }}。</p><button class="inv-save" :disabled="busy" @click="save">{{ busy?'正在保存…':'保存定投' }}</button></main></f7-page></f7-popup>
  <f7-sheet class="inv-sheet cy-mobile-surface cy-investment-page" :opened="!!confirming" @sheet:closed="confirming=undefined" swipe-to-close backdrop><h2>确认基金净值</h2><p v-if="formError" class="cy-message" role="alert">{{ formError }}</p><label class="inv-row"><span>确认日期</span><input v-model="confirmDate" type="date" /></label><label class="inv-row"><span>确认净值</span><input v-model="confirmPrice" inputmode="decimal" aria-label="确认净值" /></label><p class="inv-caption">确认后将按该净值计算份额，并更新资金账户。</p><button class="inv-save" :disabled="busy" @click="confirm">确认入账</button></f7-sheet>
 </f7-page>
</template>
<script setup lang="ts">
import {computed,ref} from 'vue';
import {f7} from 'framework7-vue';
import type {Router} from 'framework7/types';
import moment from 'moment-timezone';
import {investments,investmentError} from '@/lib/investments.ts';
import {useInvestmentData,invalidateInvestmentData,validInvestmentNumber} from '@/lib/investment-mobile.ts';
import {assetMoney as ledgerMoney,assetText} from '@/lib/asset-visibility.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
import type {InvestmentPlan,InvestmentOrder} from '@/models/investment.ts';
const props=defineProps<{f7route:Router.Route}>(),q=props.f7route.query;
const {wealth,rows,assets,orders,books,loading,error,zone,load}=useInvestmentData();
const tab=ref(q['tab']==='orders'?'orders':'plans'),plans=ref<InvestmentPlan[]>([]),busy=ref(false),showHistory=ref(false),showEditor=ref(false),notice=ref(''),formError=ref(''),selectedHolding=ref(''),confirming=ref<InvestmentOrder>(),confirmPrice=ref(''),confirmDate=ref('');
const cycleNames:Record<string,string>={daily:'每天',weekly:'每周',biweekly:'每两周',monthly:'每月'},statuses={pending:'待确认',completed:'已入账',cancelled:'已取消'};
function blank():InvestmentPlan{return {id:'',accountId:q['accountId']||'',instrumentId:q['instrumentId']||'',cashAccountId:'',bookId:books.defaultBookId,amount:'',feePercent:'0',cycle:'monthly',startDate:moment().tz(zone.value).format('YYYY-MM-DD'),endDate:'',nextDate:'',time:'09:00',timeZone:zone.value,note:'',paused:false,deleted:false,version:0};}
const draft=ref<InvestmentPlan>(blank());
const scopedPlans=computed(()=>plans.value.filter(p=>(!q['accountId']||p.accountId===q['accountId'])&&(!q['instrumentId']||p.instrumentId===q['instrumentId']))),scopedOrders=computed(()=>orders.value.filter(o=>(!q['accountId']||o.accountId===q['accountId'])&&(!q['instrumentId']||o.instrumentId===q['instrumentId']))),shownOrders=computed(()=>scopedOrders.value.filter(o=>showHistory.value||o.status==='pending'));
const fundRows=computed(()=>rows.value.filter(r=>r.asset?.type==='FUND')),cash=computed(()=>wealth.value?.cashAccounts.filter(a=>a.currency==='CNY'&&!a.liability)||[]);
function assetName(id:string):string{return assets.value.find(a=>a.id===id)?.name||id;}
function money(v:string):string{return ledgerMoney(v,false);}
async function refresh():Promise<void>{await load();try{plans.value=await investments.plans();}catch(e){error.value=investmentError(e);}}
function newPlan():void{draft.value=blank();selectedHolding.value=fundRows.value.find(r=>r.position.accountId===q['accountId']&&r.position.instrumentId===q['instrumentId'])?.key||fundRows.value[0]?.key||'';draft.value.cashAccountId=cash.value[0]?.id||'';formError.value='';showEditor.value=true;}
function edit(p:InvestmentPlan):void{draft.value={...p};selectedHolding.value=p.accountId+':'+p.instrumentId;formError.value='';showEditor.value=true;}
async function save():Promise<void>{if(busy.value)return;busy.value=true;formError.value='';try{const r=fundRows.value.find(r=>r.key===selectedHolding.value);if(!r)throw Error('请先新增一项基金理财');validInvestmentNumber(draft.value.amount,'金额',true);validInvestmentNumber(draft.value.feePercent,'费率');await investments.savePlan({...draft.value,accountId:r.position.accountId,instrumentId:r.position.instrumentId,timeZone:zone.value});showEditor.value=false;notice.value='定投已保存';await refresh();}catch(e){formError.value=investmentError(e);}finally{busy.value=false;}if(!showEditor.value)void sync();}
async function changePlan(p:InvestmentPlan,changes:Partial<InvestmentPlan>):Promise<void>{if(busy.value)return;busy.value=true;try{await investments.savePlan({...p,...changes});await refresh();}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
function removePlan(p:InvestmentPlan):void{f7.dialog.confirm('删除后停止自动定投，已入账记录保留。','删除定投',()=>{void changePlan(p,{deleted:true,paused:true});});}
async function sync():Promise<void>{if(busy.value)return;busy.value=true;error.value='';try{const result=await investments.syncPlans(true);notice.value=`已入账 ${result.created} 笔，待确认 ${result.pending} 笔`;if(result.created)invalidateInvestmentData();await refresh();}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
function openConfirm(o:InvestmentOrder):void{confirming.value=o;confirmDate.value=o.confirmDate||o.tradeDate;confirmPrice.value='';formError.value='';}
async function confirm():Promise<void>{if(!confirming.value||busy.value)return;busy.value=true;try{validInvestmentNumber(confirmPrice.value,'净值',true);await investments.confirmOrder({id:confirming.value.id,version:confirming.value.version,price:confirmPrice.value,date:confirmDate.value});confirming.value=undefined;invalidateInvestmentData();await refresh();}catch(e){formError.value=investmentError(e);}finally{busy.value=false;}}
async function cancel(o:InvestmentOrder):Promise<void>{if(busy.value)return;busy.value=true;try{await investments.cancelOrder({id:o.id,version:o.version});await refresh();}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
</script>
