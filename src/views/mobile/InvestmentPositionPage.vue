<template>
 <f7-page class="cy-mobile-surface cy-investment-page" @page:afterin="activate" @page:beforeout="live.stop">
  <f7-navbar :title="row?.name||'理财详情'" back-link="理财"><f7-nav-right><f7-link aria-label="更多理财操作" @click="showMenu=true"><f7-icon f7="ellipsis" /></f7-link></f7-nav-right></f7-navbar>
  <main class="inv-body">
   <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh">重试</button></p>
   <p v-if="loading&&!row" class="cy-empty">正在加载…</p>
   <template v-if="row">
    <section class="inv-card inv-title">
     <p class="inv-muted">持有金额（{{ currency==='USD'?'美元':'元' }}）</p><strong>{{ money(positionValue(row.position,currency,wealth)) }}</strong>
     <dl class="inv-metrics">
      <div :title="row?.position.dailyReason"><dt>今日收益（元）</dt><dd :class="profitClass(dayProfit)">{{ money(dayProfit) }}</dd></div>
      <div><dt>持有收益（元）</dt><dd :class="profitClass(row.position.unrealizedPnl)">{{ money(row.position.unrealizedPnl) }}</dd></div>
      <div><dt>累计收益（元）{{ row.profile.profitOffset!=='0'?' · 含显示修正':'' }}</dt><dd :class="profitClass(row.profit)">{{ money(row.profit) }}</dd></div>
      <div><dt>每份成本价（元）</dt><dd>{{ decimal(row.position.averageCost) }}</dd></div>
      <div><dt>持有份额</dt><dd>{{ decimal(row.position.quantity,8) }}</dd></div>
      <div><dt>{{ row.asset?.type==='FUND'?'基金净值':'最新价' }}{{ row.position.quote?.currency&&row.position.quote.currency!=='CNY'?' · '+row.position.quote.currency:'' }}</dt><dd>{{ decimal(row.position.quote?.price,6) }}</dd></div>
     </dl>
     <details class="inv-details" style="margin-top:16px;text-align:left"><summary>{{ row.asset?.symbol }} · {{ quoteStatus(row.position.quote) }} <span :class="profitClass(row.position.quote?.changePercent)">{{ quoteChange(row.position.quote) }}</span></summary><p>账户：{{ row.account?.name }} · 总成本（元）：{{ money(row.position.cost) }}</p><p v-if="row.asset&&!row.asset.provider&&!isPresetInstrument(row.asset)">未绑定自动行情，可在理财管理中设置。</p><p>{{ row.position.quote?.source || '尚无报价' }} · {{ date(row.position.quote?.sourceTime) }}</p><p v-if="row.position.quote?.currency!=='CNY'">人民币汇率 {{ row.position.quote?.fxRate || '未知' }} · {{ row.position.quote?.fxDate || '日期未知' }}</p><p>今日收益按 {{ zone }} 零点边界前的已收盘价及当时可用的人民币参考汇率计算，并计入今日交易和手续费。{{ row.position.dailyReason || '' }}{{ row.profile.profitOffset!=='0'?`累计收益包含显示修正 ${assetText(row.profile.profitOffset)} 元。`:'' }}</p><p v-if="row.position.dailyReference">零点基准：{{ row.position.dailyReference.price }} {{ row.position.dailyReference.currency }} · {{ row.position.dailyReference.source }}；汇率 {{ row.position.dailyReference.fxRate }}（{{ row.position.dailyReference.fxDate }}）。</p><p v-if="row.profile.note">{{ row.profile.note }}</p></details>
    </section>
    <nav class="inv-trade-buttons"><f7-link :href="actionLink('BUY')"><f7-icon f7="plus_circle" />买入</f7-link><f7-link :href="actionLink('SELL')"><f7-icon f7="minus_circle" />卖出</f7-link></nav>
    <f7-link v-if="pending.length" class="inv-card inv-list-button" :href="scoped('/investments/plans',{tab:'orders'})"><span>{{ pending.length }} 笔待确认</span><f7-icon f7="chevron_right" /></f7-link>
    <section class="inv-card">
      <div class="inv-event-line"><span class="inv-muted">交易记录</span><button class="inv-link" style="padding:0;font-size:12px" @click="showVoided=!showVoided">{{ showVoided?'收起撤销记录':'查看撤销记录' }}</button></div>
      <p v-if="!visibleEvents.length" class="cy-empty">还没有交易记录</p>
      <button v-for="event in visibleEvents" :key="event.id" type="button" class="inv-event" style="border-inline:0;border-top:0;background:none;text-align:left" @click="selectedEvent=event">
        <div class="inv-event-line"><strong>{{ investmentNames[event.type] }}{{ event.voided?' · 已撤销':'' }}{{ event.settlementInstrumentId===instrumentId?' · 结算':'' }}</strong><strong>{{ eventAmount(event) }}</strong></div>
        <div class="inv-event-line"><small>{{ date(event.occurredAt) }}{{ event.note?' · '+event.note:'' }}</small><small style="text-align:right">{{ cashName(event) }}</small></div>
      </button>
    </section>
   </template>
   <p v-else-if="!loading&&!error" class="cy-empty">找不到这项理财，请返回列表</p>
  </main>
  <f7-sheet class="inv-sheet cy-mobile-surface cy-investment-page" v-model:opened="showMenu" swipe-to-close backdrop>
    <f7-link class="inv-list-button" :href="scoped('/investments/statistics')" sheet-close>收益统计<f7-icon f7="chart_bar" /></f7-link>
    <button class="inv-list-button" @click="exportRows">流水导出<f7-icon f7="square_arrow_up" /></button>
    <f7-link v-if="row?.asset?.type==='FUND'" class="inv-list-button" :href="scoped('/investments/plans')" sheet-close>理财定投<f7-icon f7="repeat" /></f7-link>
    <f7-link class="inv-list-button" :href="editLink" sheet-close>编辑持仓<f7-icon f7="pencil" /></f7-link>
    <f7-link class="inv-list-button" :href="accountEditLink" sheet-close>编辑账户<f7-icon f7="building_2_fill" /></f7-link>
    <button class="inv-list-button" @click="showMenu=false;openProfit()">对账修正 · 累计收益显示修正<f7-icon f7="slider_horizontal_3" /></button>
    <f7-link class="inv-list-button" :href="actionLink('quote')" sheet-close>更新参考价格<f7-icon f7="arrow_clockwise" /></f7-link>
    <f7-link class="inv-list-button" :href="actionLink('TRANSFER')" sheet-close>账户间转移<f7-icon f7="arrow_right_arrow_left" /></f7-link>
    <button class="inv-list-button" @click="toggleHidden">{{ row?.profile.hidden?'恢复显示':'隐藏理财' }}<f7-icon f7="eye_slash" /></button>
    <f7-link class="inv-list-button" href="/investments/manage" sheet-close>账户管理与删除<f7-icon f7="gear" /></f7-link>
  </f7-sheet>
  <f7-sheet class="inv-sheet cy-mobile-surface cy-investment-page" :opened="!!selectedEvent" @sheet:closed="selectedEvent=undefined" swipe-to-close backdrop>
    <template v-if="selectedEvent"><h2>{{ investmentNames[selectedEvent.type] }}</h2><p class="inv-caption">{{ date(selectedEvent.occurredAt) }}</p><div class="inv-row"><span>份额</span><strong style="margin-left:auto">{{ assetText(selectedEvent.quantity) }}</strong></div><div class="inv-row"><span>手续费</span><span style="margin-left:auto">{{ assetText(selectedEvent.fee) }}</span></div><p v-if="selectedEvent.note" class="inv-caption">{{ selectedEvent.note }}</p><p v-if="selectedEvent.fund" class="inv-caption">申请 {{ selectedEvent.fund.tradeDate }} · 确认 {{ selectedEvent.fund.confirmDate }}<br />净值 {{ selectedEvent.fund.price }} · {{ selectedEvent.fund.source }}</p><f7-link v-if="!selectedEvent.voided" class="inv-list-button" :href="actionLink('revise',selectedEvent.id)" sheet-close>编辑记录<f7-icon f7="pencil" /></f7-link><f7-link v-if="!selectedEvent.voided" class="inv-list-button" :href="actionLink('void',selectedEvent.id)" sheet-close>撤销记录<f7-icon f7="trash" /></f7-link></template>
  </f7-sheet>
  <f7-sheet class="inv-sheet cy-mobile-surface cy-investment-page" v-model:opened="showProfit" swipe-to-close backdrop><h2>累计收益显示修正</h2><label class="inv-row"><span>累计收益（元）</span><input v-model="profitInput" inputmode="decimal" aria-label="累计收益" /></label><p class="inv-caption">请先核对成本、手续费和分红。差额只修正显示，不改变真实收益、持仓、现金或历史收益曲线。</p><button class="inv-save" :disabled="busy" @click="saveProfit">保存</button></f7-sheet>
 </f7-page>
</template>
<script setup lang="ts">
import {downloadLedgerFile} from '@/lib/ledger-export.ts';
import {computed,onUnmounted,ref} from 'vue';
import type {Router} from 'framework7/types';
import moment from 'moment-timezone';
import {useInvestmentData,investmentNames,investmentSum,profitClass,validInvestmentNumber} from '@/lib/investment-mobile.ts';
import {investments,investmentError} from '@/lib/investments.ts';
import {quoteChange,quoteStatus,investmentDecimalText,isPresetInstrument} from '@/lib/investment-display.ts';
import {LedgerDecimal} from '@/lib/ledger-display.ts';
import {assetMoney as money,assetText,assetAmountsVisible} from '@/lib/asset-visibility.ts';
import {createValuationRefresh} from '@/lib/valuation-refresh.ts';
import {walletCurrency,positionValue} from '@/lib/wallet-entry.ts';
import {isCryptoAccount} from '@/lib/crypto-platforms.ts';
import type {InvestmentEvent} from '@/models/investment.ts';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>(),accountId=props.f7route.query['accountId']||'',instrumentId=props.f7route.query['instrumentId']||'';
const {wealth,rows,events,orders,loading,error,zone,load}=useInvestmentData();
const row=computed(()=>rows.value.find(r=>r.position.accountId===accountId&&r.position.instrumentId===instrumentId)),currency=computed(()=>walletCurrency(row.value?.account));
const showMenu=ref(false),showVoided=ref(false),selectedEvent=ref<InvestmentEvent>(),showProfit=ref(false),profitInput=ref(''),busy=ref(false);
const scopedEvents=computed(()=>events.value.filter(e=>(e.instrumentId===instrumentId&&(e.accountId===accountId||e.toAccountId===accountId))||(e.settlementInstrumentId===instrumentId&&(e.settlementAccountId||e.accountId)===accountId)||(e.accountId===accountId&&e.additionalMovements?.some(m=>m.instrumentId===instrumentId))));
const visibleEvents=computed(()=>scopedEvents.value.filter(e=>showVoided.value||!e.voided).sort((a,b)=>b.occurredAt-a.occurredAt));
const pending=computed(()=>orders.value.filter(o=>o.status==='pending'&&o.accountId===accountId&&o.instrumentId===instrumentId));
const dayProfit=computed(()=>row.value?.position.dailyPnl??null);
function scoped(path:string,extra:Record<string,string>={}):string{return path+'?'+new URLSearchParams({accountId,instrumentId,...extra});}
const editLink=computed(()=>scoped('/investments/add'));
const accountEditLink=computed(()=>isCryptoAccount(row.value?.account?.kind)?'/crypto/add?'+new URLSearchParams({id:accountId,kind:row.value!.account!.kind}):scoped('/investments/record',{action:'account'}));
function actionLink(action:string,eventId=''):string{const event=events.value.find(e=>e.id===eventId);if(event?.wallet)return '/crypto/entry?'+new URLSearchParams({eventId,action});if(isCryptoAccount(row.value?.account?.kind)&&['BUY','SELL','TRANSFER'].includes(action))return scoped('/crypto/convert',{mode:({BUY:'cash',SELL:'redeem',TRANSFER:'transfer'} as Record<string,string>)[action]!});return scoped('/investments/record',{action,eventId});}
function decimal(v:string|null|undefined,places=4):string{return assetAmountsVisible.value?investmentDecimalText(v,places):'••••';}
function date(at?:number):string{return at?moment.unix(at).tz(zone.value).format('YYYY-MM-DD HH:mm'):'时间未知';}
function cashName(e:InvestmentEvent):string{const cash=wealth.value?.cashAccounts.find(a=>a.id===e.cashAccountId);return cash?`${e.type==='SELL'?'收款':'付款'}：${cash.name}`:'';}
function eventAmount(e:InvestmentEvent):string{if(!assetAmountsVisible.value)return '••••';if(!['BUY','SELL'].includes(e.type))return e.quantity+' 份';const d=new LedgerDecimal(e.amount);return money((e.type==='BUY'?d.plus(e.fee):d.minus(e.fee)).toString())+' '+(wealth.value?.cashAccounts.find(a=>a.id===e.cashAccountId)?.currency||'结算单位');}
async function refresh():Promise<void>{await load();}
const live=createValuationRefresh(summary=>{wealth.value=summary;});function activate():void{void refresh();live.start();}onUnmounted(live.stop);
async function toggleHidden():Promise<void>{if(!row.value)return;try{await investments.saveProfile({...row.value.profile,hidden:!row.value.profile.hidden});showMenu.value=false;await load();}catch(e){error.value=investmentError(e);}}
function openProfit():void{profitInput.value=row.value?.profit||'';showProfit.value=true;}
async function saveProfit():Promise<void>{if(!row.value||busy.value)return;busy.value=true;try{validInvestmentNumber(profitInput.value,'累计收益',false,true);const base=investmentSum([row.value.position.realizedPnl,row.value.position.unrealizedPnl]);if(base===null)throw Error('成本或报价未知，请先补齐，再调整累计收益');await investments.saveProfile({...row.value.profile,profitOffset:new LedgerDecimal(profitInput.value).minus(base).toString()});showProfit.value=false;await load();}catch(e){error.value=investmentError(e);showProfit.value=false;}finally{busy.value=false;}}
function exportRows():void{const esc=(s:unknown)=>'"'+String(s??'').replace(/"/g,'""')+'"';const data=[['日期','类型','数量','金额','手续费','历史汇率','备注','状态'],...scopedEvents.value.map(e=>[date(e.occurredAt),investmentNames[e.type],e.quantity,e.amount,e.fee,e.exchangeRate,e.note,e.voided?'已撤销':'有效'])].map(r=>r.map(esc).join(',')).join('\r\n');downloadLedgerFile('\uFEFF'+data,'理财流水.csv','text/csv;charset=utf-8');showMenu.value=false;}
</script>
