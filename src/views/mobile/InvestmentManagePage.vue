<template>
 <f7-page class="cy-mobile-surface cy-investment-page" @page:afterin="load">
  <f7-navbar title="理财管理" back-link="理财" />
  <main class="inv-body"><p v-if="error" role="alert" class="cy-message">{{ error }}</p>
   <section class="inv-card"><h2>理财显示</h2><p v-if="!rows.length" class="cy-empty">还没有理财</p><article v-for="r in rows" :key="r.key" class="inv-event"><div class="inv-event-line"><strong>{{ r.name }}</strong><button class="inv-link" style="padding:0" @click="toggle(r)">{{ r.profile.hidden?'恢复显示':'隐藏' }}</button></div><p class="inv-muted">{{ r.group }} · {{ r.account?.name }}</p><div class="inv-toolbar"><f7-link :href="`/investments/add?accountId=${r.position.accountId}&instrumentId=${encodeURIComponent(r.position.instrumentId)}`">编辑</f7-link><f7-link :href="`/investments/record?action=quote&accountId=${r.position.accountId}&instrumentId=${encodeURIComponent(r.position.instrumentId)}`">更新价格</f7-link><button v-if="r.asset?.id.startsWith('custom:')" class="inv-link" @click="binding=binding===r.asset.id?'':r.asset.id">行情设置</button></div><InstrumentSearch v-if="binding===r.asset?.id" :instrument-id="binding" :initial-market="searchMarket(r)" @saved="binding='';load()" /></article></section>
   <section class="inv-card"><div class="inv-event-line"><h2>投资账户</h2><f7-link href="/investments/record?action=account&kind=BROKER">新增</f7-link></div><div v-for="a in accounts" :key="a.id" class="inv-row"><span style="flex:1">{{ a.name }}</span><AccountOptionsMenu :edit-href="['EXCHANGE','WALLET'].includes(a.kind)?`/crypto/add?id=${a.id}&kind=${a.kind}`:`/investments/record?action=account&accountId=${a.id}`" :target="{id:a.id,name:a.name,kind:'portfolio',href:`/investments/ledger?accountId=${a.id}`}" @deleted="load" /></div></section>
  </main>
 </f7-page>
</template>
<script setup lang="ts">
import {ref} from 'vue';
import {useInvestmentData,type HoldingRow} from '@/lib/investment-mobile.ts';
import {investments,investmentError} from '@/lib/investments.ts';
import AccountOptionsMenu from '@/components/mobile/AccountOptionsMenu.vue';
import InstrumentSearch from '@/components/InstrumentSearch.vue';
const {rows,accounts,error,load}=useInvestmentData(),binding=ref('');
async function toggle(row:HoldingRow):Promise<void>{try{await investments.saveProfile({...row.profile,hidden:!row.profile.hidden});await load();}catch(e){error.value=investmentError(e);}}
function searchMarket(r:HoldingRow):string{const m=r.asset?.market||'';return m.startsWith('CN_')&&m!=='CN_FUND'?'CN':m||'CN_FUND';}
</script>
