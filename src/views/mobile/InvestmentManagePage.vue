<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="load">
        <f7-navbar title="账户与投资资产" back-link="理财" />
        <main class="cy-page-body"><p v-if="error" class="cy-message" role="alert">{{ error }}</p>
            <section class="cy-panel"><div class="cy-section-head"><h2>投资账户</h2><f7-link href="/crypto/add?kind=EXCHANGE">添加交易所</f7-link><f7-link href="/crypto/add?kind=WALLET">添加钱包</f7-link><f7-link href="/investments/record?action=account&kind=BROKER">添加证券账户</f7-link></div><p v-for="account in accounts" :key="account.id" class="cy-management-row">{{ account.name }} <small>{{ kinds[account.kind] || account.kind }}</small></p><p v-if="!accounts.length" class="cy-muted">按交易所、钱包或证券账户建立账户。</p></section>
            <section class="cy-panel"><InstrumentSearch @saved="added" /></section>
            <section class="cy-panel"><div class="cy-section-head"><h2>资产目录</h2><f7-link href="/investments/record?action=instrument">手动创建</f7-link></div><p v-if="notice" class="cy-message" role="status">{{ notice }}</p><article v-for="asset in instruments" :key="asset.id" class="cy-asset-management"><strong>{{ asset.name }} · {{ asset.symbol }}</strong><p class="cy-muted">{{ instrumentMarketLabel(asset) }} · {{ asset.provider ? `${asset.provider} / ${asset.providerId}` : isPresetInstrument(asset) ? '自动行情由系统维护' : '手动估值' }}</p><p class="cy-muted">{{ currentQuoteLabel(asset) }}</p><div><button v-if="asset.id.startsWith('custom:')" @click="binding= binding===asset.id ? '' : asset.id">{{ asset.provider ? '更换行情绑定' : '绑定行情' }}</button><f7-link :href="`/investments/record?action=quote&instrumentId=${encodeURIComponent(asset.id)}`">手动报价</f7-link></div><InstrumentSearch v-if="asset.id.startsWith('custom:') && binding===asset.id" :instrument-id="asset.id" @saved="added" /></article></section>
        </main>
    </f7-page>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import InstrumentSearch from '@/components/InstrumentSearch.vue';
import { investments, investmentError } from '@/lib/investments.ts';
import { instrumentMarketLabel, isPresetInstrument, quoteStatus } from '@/lib/investment-display.ts';
import type { Instrument,InvestmentAccount,InvestmentQuote } from '@/models/investment.ts';
const instruments=ref<Instrument[]>([]),accounts=ref<InvestmentAccount[]>([]),quotes=ref<InvestmentQuote[]>([]),error=ref(''),notice=ref(''),binding=ref('');
const kinds:Record<string,string>={EXCHANGE:'加密货币交易所',WALLET:'加密钱包',BROKER:'证券账户',OTHER:'其他'};
function currentQuoteLabel(asset:Instrument):string{const quote=quotes.value.find(item=>item.instrumentId===asset.id);if(quote?.state==='manual')return `当前使用手动报价（人民币）${isPresetInstrument(asset)||asset.provider?'，自动行情已被覆盖':''}`;if(quote?.price&&quote.source)return `当前来源：${quote.source} · ${quoteStatus(quote)}`;return isPresetInstrument(asset)||asset.provider?'自动行情尚无可用报价，可手动估值':'尚未填写手动报价';}
async function load():Promise<void>{error.value='';try{[instruments.value,accounts.value,quotes.value]=await Promise.all([investments.instruments(),investments.accounts(),investments.quotes()]);}catch(cause){error.value=investmentError(cause);}}
async function added(asset:Instrument):Promise<void>{notice.value=`${asset.name} 已保存。`;binding.value='';await load();}
</script>
<style scoped>
.cy-management-row{display:flex;justify-content:space-between;gap:10px;padding:12px 0;border-bottom:1px solid var(--cy-line)}.cy-management-row small{color:var(--cy-muted)}.cy-asset-management{padding:18px 0;border-bottom:1px solid var(--cy-line)}.cy-asset-management:last-child{border:0}.cy-asset-management>div{display:flex;gap:20px;margin:10px 0;font-size:12px}.cy-asset-management button{border:0;background:transparent;padding:0;color:var(--cy-accent)}
</style>
