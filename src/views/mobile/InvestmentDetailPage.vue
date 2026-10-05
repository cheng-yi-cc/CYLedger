<template>
    <f7-page class="cy-main-page cy-mobile-surface" ptr @ptr:refresh="refresh" @page:afterin="activate" @page:beforeout="deactivate">
        <f7-navbar :title="selectedAccount?.name || '投资理财'" back-link="资产"><f7-nav-right><AccountOptionsMenu v-if="selectedAccount" :edit-href="`/investments/record?action=account&accountId=${selectedAccount.id}`" :target="{id:selectedAccount.id,name:selectedAccount.name,kind:'portfolio',href:`/investments/ledger?accountId=${selectedAccount.id}`}" @deleted="f7router.navigate('/investments',{reloadAll:true})" /></f7-nav-right></f7-navbar>
        <main class="cy-page-body">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <section class="cy-panel cy-investment-total"><p class="cy-muted">投资参考市值（人民币）</p><strong>{{ ledgerMoney(totalValue) }}</strong><div><span>持有收益 {{ ledgerMoney(unrealized) }}</span><span>全部账户已实现收益 {{ ledgerMoney(summary?.realizedPnl) }}</span></div></section>
            <nav class="cy-segments cy-investment-types" aria-label="理财类型"><button v-for="item in types" :key="item.value" :aria-pressed="type===item.value" @click="type=item.value">{{ item.name }}</button></nav>
            <input v-model="search" class="cy-position-search" aria-label="查找持仓" placeholder="查找资产名称或代码" />
            <p class="cy-muted cy-holding-note">持仓和成本使用全部账本的完整历史。涨跌幅描述价格变化，持有收益还取决于买入成本。</p>
            <p v-if="loading && !summary" class="cy-empty">正在加载持仓…</p>
            <section v-for="position in visiblePositions" :key="`${position.accountId}:${position.instrumentId}`" class="cy-panel cy-holding-card">
                <f7-link :href="positionLink(position)"><div class="cy-holding-title"><span><strong>{{ instrument(position.instrumentId)?.name || position.instrumentId }}</strong><small>{{ instrument(position.instrumentId)?.symbol }} · {{ accountName(position.accountId) }}</small></span><span class="cy-holding-value"><strong>{{ ledgerMoney(position.marketValue) }}</strong><small>{{ quoteStatus(position.quote) }}</small></span></div><dl><div><dt>持有数量</dt><dd>{{ position.quantity }}</dd></div><div><dt>持有收益</dt><dd>{{ ledgerMoney(position.unrealizedPnl) }}</dd></div><div><dt>{{ quoteChangeLabel(position.quote) }}</dt><dd>{{ quoteChange(position.quote) }}</dd></div></dl></f7-link>
            </section>
            <section v-if="!loading && !visiblePositions.length" class="cy-panel cy-empty"><p>{{ search || type ? '没有匹配的持仓' : '还没有投资持仓' }}</p><p class="cy-muted">先添加投资账户和资产，再录入已有持仓。</p><f7-link href="/investments/manage">管理账户与资产</f7-link></section>
            <div class="cy-investment-actions"><f7-link :href="`/investments/record?action=OPENING&accountId=${encodeURIComponent(f7route.query['accountId'] || '')}`">录入已有持仓</f7-link><f7-link :href="`/investments/record?action=BUY&accountId=${encodeURIComponent(f7route.query['accountId'] || '')}`">记录新买入</f7-link></div>
        </main><template #fixed><LedgerNavigation active="assets" /></template>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, onUnmounted, ref } from 'vue';
import type { Router } from 'framework7/types';
const props = defineProps<{ f7route: Router.Route; f7router:Router.Router }>();
import { investments, investmentError } from '@/lib/investments.ts';
import { LedgerDecimal, ledgerMoney } from '@/lib/ledger-display.ts';
import { quoteStatus, quoteChange, quoteChangeLabel } from '@/lib/investment-display.ts';
import type { Instrument, InvestmentAccount, InvestmentPosition, WealthSummary } from '@/models/investment.ts';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import AccountOptionsMenu from '@/components/mobile/AccountOptionsMenu.vue';
const summary=ref<WealthSummary>(),instruments=ref<Instrument[]>([]),accounts=ref<InvestmentAccount[]>([]),type=ref(''),search=ref(''),loading=ref(false),error=ref('');
const selectedAccount=computed(()=>accounts.value.find(a=>a.id===props.f7route.query['accountId']));
const types=[{value:'',name:'全部'},{value:'STOCK',name:'股票'},{value:'FUND',name:'基金'},{value:'CRYPTO',name:'加密货币'},{value:'OTHER',name:'其他'}];
function instrument(id:string) {return instruments.value.find(item=>item.id===id);}
function accountName(id:string):string{return accounts.value.find(item=>item.id===id)?.name || id;}
const visiblePositions=computed(()=>(summary.value?.positions||[]).filter(position=>{const asset=instrument(position.instrumentId);return (!props.f7route.query['accountId'] || position.accountId===props.f7route.query['accountId'])&& new LedgerDecimal(position.quantity).gt(0)&&(!type.value||asset?.type===type.value)&&(!search.value||`${asset?.name} ${asset?.symbol}`.toLowerCase().includes(search.value.toLowerCase()));}));
function total(field:'marketValue'|'unrealizedPnl'):string|null{if(!summary.value)return null; const values=visiblePositions.value.map(item=>item[field]);return values.some(value=>value==null)?null:values.reduce<InstanceType<typeof LedgerDecimal>>((sum,value)=>sum.plus(value!),new LedgerDecimal(0)).toString();}
const totalValue=computed(()=>total('marketValue')),unrealized=computed(()=>total('unrealizedPnl'));
function positionLink(position:InvestmentPosition):string{return '/investments/position?'+new URLSearchParams({accountId:position.accountId,instrumentId:position.instrumentId}).toString();}
async function refresh(done?:()=>void):Promise<void>{if(loading.value){if(typeof done==='function')done();return;}loading.value=true;error.value='';try{const [s,i,a]=await Promise.all([investments.summary(),investments.instruments(),investments.accounts()]);summary.value=s;instruments.value=i;accounts.value=a;}catch(cause){error.value=investmentError(cause);}finally{loading.value=false;if(typeof done==='function')done();}}
let timer:ReturnType<typeof setInterval>|undefined;
function activate():void{deactivate();void refresh();timer=setInterval(()=>{if(!document.hidden)void refresh();},15000);}
function deactivate():void{clearInterval(timer);}
onUnmounted(deactivate);
</script>
<style scoped>
.cy-investment-total>strong{font-size:32px;display:block;margin:10px 0 18px;overflow-wrap:anywhere}.cy-investment-total>div{display:flex;justify-content:space-between;gap:12px;flex-wrap:wrap;font-size:12px}.cy-investment-types{display:flex;margin-bottom:16px;overflow:auto}.cy-investment-types button{flex:1;white-space:nowrap;font-size:13px}.cy-position-search{width:100%;padding:13px;border:1px solid var(--cy-line);border-radius:10px;background:var(--cy-card);margin-bottom:10px}.cy-holding-note{margin-bottom:18px!important}.cy-holding-card>a{display:block;color:inherit;width:100%}.cy-holding-title{display:flex;justify-content:space-between;gap:10px}.cy-holding-title strong{font-size:17px}.cy-holding-title small{display:block;font-size:11px;color:var(--cy-muted);margin-top:6px}.cy-holding-value{text-align:right;max-width:52%;overflow-wrap:anywhere}.cy-holding-card dl{display:grid;grid-template-columns:1fr 1fr 1fr;gap:10px;margin:20px 0 0}.cy-holding-card dt{font-size:10px;color:var(--cy-muted)}.cy-holding-card dd{margin:7px 0 0;font-size:13px;overflow-wrap:anywhere}.cy-investment-actions{display:grid;grid-template-columns:1fr 1fr;gap:12px;margin:20px 0}.cy-investment-actions a{text-align:center;padding:14px 8px;background:var(--cy-card);border:1px solid var(--cy-accent);border-radius:12px;color:var(--cy-accent)}.cy-investment-actions a:last-child{background:var(--cy-accent);color:#fff}
</style>
