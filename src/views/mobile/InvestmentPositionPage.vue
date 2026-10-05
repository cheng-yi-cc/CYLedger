<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="refresh">
        <f7-navbar :title="asset?.name || '持仓详情'" back-link="理财" />
        <main class="cy-page-body cy-position-body">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh">重试</button></p>
            <p v-if="loading" class="cy-empty">正在加载持仓…</p>
            <template v-else-if="asset">
                <section class="cy-panel"><p class="cy-muted">{{ asset.symbol }} · {{ account?.name || '投资账户' }} · {{ instrumentMarketLabel(asset) }}</p><p class="cy-muted cy-value-label">当前市值（人民币）</p><strong class="cy-position-value">{{ ledgerMoney(position?.marketValue) }}</strong><dl class="cy-position-metrics"><div><dt>持有数量</dt><dd>{{ position?.quantity || '0' }}</dd></div><div><dt>剩余成本</dt><dd>{{ position?.costKnown ? ledgerMoney(position.cost) : '未知' }}</dd></div><div><dt>持有收益</dt><dd>{{ ledgerMoney(position?.unrealizedPnl) }}</dd></div><div><dt>已实现收益</dt><dd>{{ ledgerMoney(position?.realizedPnl) }}</dd></div></dl></section>
                <section class="cy-panel"><div class="cy-section-head"><h2>当前行情</h2><f7-link :href="actionLink('quote')">手动估值</f7-link></div><p class="cy-price"><strong>{{ position?.quote?.price || '—' }}</strong> {{ position?.quote?.currency || asset.currency || (isPresetInstrument(asset) ? 'USD' : 'CNY') }}</p><p>{{ quoteChangeLabel(position?.quote) }} <strong>{{ quoteChange(position?.quote) }}</strong></p><p class="cy-muted">{{ quoteStatus(position?.quote) }} · {{ position?.quote?.source || '尚无报价' }}</p><details class="cy-quote-details"><summary>报价来源与时间</summary><p v-if="position?.quote" class="cy-muted">报价时间 {{ date(position.quote.sourceTime) }}<br />收到时间 {{ date(position.quote.receivedAt) }}</p><p v-if="position?.quote?.currency !== 'CNY'" class="cy-muted">折算汇率 {{ position?.quote?.fxRate || '未知' }} · {{ position?.quote?.fxDate || '日期未知' }}<br />{{ position?.quote?.fxSource || '缺少汇率来源' }} · {{ position?.quote?.fxState === 'stale' ? '已过期' : position?.quote?.fxRate ? '最近可用参考汇率' : '不可折算' }}</p><p class="cy-muted">涨跌幅为币价变化，不是个人收益。</p></details></section>
                <section class="cy-panel"><div class="cy-section-head"><h2>投资流水</h2><span>{{ visibleEvents.length }} 笔</span></div><BookScope /><p v-if="!visibleEvents.length" class="cy-empty">所选账本没有这项资产的流水。</p><article v-for="event in visibleEvents" :key="event.id" class="cy-position-event"><div><strong>{{ event.settlementInstrumentId === asset.id ? "结算 · " : "" }}{{ eventNames[event.type] }} <small v-if="event.voided">已撤销</small></strong><span>{{ eventQuantity(event) }} {{ asset.symbol }}</span></div><p class="cy-muted">{{ date(event.occurredAt) }} · {{ books.allBooks.find(book=>book.id===event.bookId)?.name || '默认账本' }}</p><p v-if="event.fee !== '0'" class="cy-muted">手续费 {{ event.fee }} {{ event.type==='TRANSFER' ? asset.symbol : '结算单位' }}</p><p v-if="event.note">{{ event.note }}</p><f7-link v-if="!event.voided" :href="actionLink('revise',event.id)">查看与修订</f7-link><f7-link v-if="!event.voided" :href="actionLink('void',event.id)">撤销</f7-link></article></section>
            </template>
            <p v-else-if="!error" class="cy-empty">找不到这项资产，请返回理财列表。</p>
        </main>
        <template #fixed><div class="cy-position-actions"><f7-link :href="actionLink('BUY')">买入</f7-link><f7-link :href="actionLink('SELL')">卖出</f7-link><f7-link :href="actionLink('TRANSFER')">转移</f7-link></div></template>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import { isCryptoAccount } from '@/lib/crypto-platforms.ts';
import { investments, investmentError } from '@/lib/investments.ts';
import { LedgerDecimal, ledgerMoney } from '@/lib/ledger-display.ts';
import { instrumentMarketLabel, isPresetInstrument, quoteChange, quoteChangeLabel, quoteStatus } from '@/lib/investment-display.ts';
import { useBooksStore } from '@/stores/books.ts';
import BookScope from '@/components/mobile/BookScope.vue';
import type { Instrument, InvestmentAccount, InvestmentPosition, InvestmentEvent } from '@/models/investment.ts';
const props=defineProps<{f7route:Router.Route}>();
const books=useBooksStore(),loading=ref(false),error=ref(''),zone=ref('Asia/Shanghai');
const asset=ref<Instrument>(),account=ref<InvestmentAccount>(),position=ref<InvestmentPosition>(),events=ref<InvestmentEvent[]>([]);
const eventNames={OPENING:'录入已有持仓',BUY:'买入',SELL:'卖出',TRANSFER:'账户间转移'};
const visibleEvents=computed(()=>events.value.filter(item=>!books.selectedBookIds.length || books.selectedBookIds.includes(item.bookId || '')).sort((a,b)=>b.occurredAt-a.occurredAt));
function eventQuantity(event:InvestmentEvent):string { if(event.settlementInstrumentId!==asset.value?.id)return event.quantity; const amount=new LedgerDecimal(event.amount); return (event.type==='BUY'?amount.plus(event.fee).negated():amount.minus(event.fee)).toString(); }
function date(value:number):string{return value?moment.unix(value).tz(zone.value).format('YYYY-MM-DD HH:mm'):'时间未知';}
function actionLink(action:string,eventId=''):string{if(isCryptoAccount(account.value?.kind)&&asset.value?.type==='CRYPTO'&&['BUY','SELL','TRANSFER'].includes(action))return '/crypto/convert?'+new URLSearchParams({mode:({BUY:'cash',SELL:'redeem',TRANSFER:'transfer'} as Record<string,string>)[action]!,accountId:props.f7route.query['accountId']||'',instrumentId:props.f7route.query['instrumentId']||''}).toString();return '/investments/record?'+new URLSearchParams({action,accountId:props.f7route.query['accountId']||'',instrumentId:props.f7route.query['instrumentId']||'',eventId}).toString();}
async function refresh():Promise<void>{if(loading.value)return;loading.value=true;error.value='';try{const [summary,instruments,accounts,allEvents,settings]=await Promise.all([investments.summary(),investments.instruments(),investments.accounts(),investments.events(),investments.settings(),books.loadBooks()]);const id=props.f7route.query['instrumentId'],accountId=props.f7route.query['accountId'];asset.value=instruments.find(item=>item.id===id);account.value=accounts.find(item=>item.id===accountId);position.value=summary.positions.find(item=>item.instrumentId===id&&item.accountId===accountId);events.value=allEvents.filter(item=>(item.instrumentId===id&&(item.accountId===accountId||item.toAccountId===accountId))||(item.settlementInstrumentId===id&&item.settlementAccountId===accountId));zone.value=settings.timeZone||zone.value;}catch(cause){error.value=investmentError(cause);}finally{loading.value=false;}}
</script>
<style scoped>
.cy-position-body{padding-bottom:90px!important}.cy-value-label{margin-top:22px!important}.cy-position-value{display:block;font-size:34px;margin:7px 0 22px;overflow-wrap:anywhere}.cy-position-metrics{display:grid;grid-template-columns:1fr 1fr;gap:20px;margin:0 0 22px}.cy-position-metrics dt{color:var(--cy-muted);font-size:12px}.cy-position-metrics dd{font-size:19px;margin:7px 0 0;overflow-wrap:anywhere}.cy-price{margin:12px 0!important}.cy-price strong{font-size:25px}.cy-panel>p{margin-top:10px}.cy-position-event{padding:18px 0;border-bottom:1px solid var(--cy-line)}.cy-position-event:last-child{border:0}.cy-position-event>div{display:flex;justify-content:space-between;gap:8px}.cy-position-event p{font-size:12px;margin-top:7px}.cy-position-event a{font-size:12px;margin:12px 20px 0 0}.cy-position-actions{position:absolute;bottom:0;left:0;right:0;display:flex;padding:12px 16px calc(12px + env(safe-area-inset-bottom));gap:12px;background:var(--cy-card);border-top:1px solid var(--cy-line);z-index:100}.cy-position-actions a{flex:1;text-align:center;padding:13px 0;border-radius:10px;background:var(--cy-soft);color:var(--cy-accent)}.cy-position-actions a:first-child{background:var(--cy-accent);color:#fff}
.cy-quote-details{font-size:12px;color:var(--cy-muted);margin-top:12px}.cy-quote-details summary{cursor:pointer;padding:8px 0}
</style>
