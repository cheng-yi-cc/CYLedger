<template>
 <div class="holding-summary">
  <div class="inv-holding-top"><span>{{ row.name }}</span><strong>{{ money(row.position.marketValue) }}</strong></div>
  <div class="holding-metrics"><div><span>今日收益</span><b :class="profitClass(row.position.dailyPnl)">{{ money(row.position.dailyPnl) }}</b></div><div><span>累计收益{{ row.profile.profitOffset!=='0'?'（含显示修正）':'' }}</span><b :class="profitClass(row.profit)">{{ money(row.profit) }}</b></div></div>
  <div class="holding-caption"><span>{{ members>1?`${members} 个账户 · 点击展开`:row.account?.name }}</span><span v-if="row.profile.excludeFromTotal">不计入净资产</span><span v-else-if="row.profile.excludeProfit">净资产按成本计入</span></div>
  <p v-if="row.position.marketValue==null" class="holding-caption">估值暂不可用 · 进入详情补充行情</p>
 </div>
</template>
<script setup lang="ts">
import type {HoldingRow} from '@/lib/investment-mobile.ts';
import {profitClass} from '@/lib/investment-mobile.ts';
import {assetMoney as money} from '@/lib/asset-visibility.ts';
defineProps<{row:HoldingRow;members:number}>();
</script>
<style scoped>
.holding-summary{width:100%;min-width:0}.holding-metrics{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin:14px 0 10px}.holding-metrics>div{display:grid;gap:5px}.holding-metrics span{font-size:12px;color:var(--cy-muted)}.holding-metrics b{font-size:14px;font-weight:500;overflow-wrap:anywhere}.holding-metrics>div:last-child{text-align:right}.holding-caption{display:flex;justify-content:space-between;gap:8px;font-size:12px;color:var(--cy-muted);line-height:1.7;flex-wrap:wrap}
</style>
