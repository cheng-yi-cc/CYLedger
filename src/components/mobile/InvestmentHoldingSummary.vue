<template>
 <div class="holding-summary">
  <div class="inv-holding-top"><span>{{ row.name }}</span><small class="inv-code">{{ row.asset?.symbol }}</small><strong>{{ money(row.position.marketValue) }}</strong></div>
  <div class="holding-metrics"><div><span>持仓成本</span><b>{{ money(row.position.cost) }}</b></div><div><span>今日收益</span><b :class="profitClass(row.position.dailyPnl)">{{ money(row.position.dailyPnl) }}</b></div><div><span>累计收益</span><b :class="profitClass(row.profit)">{{ money(row.profit) }}</b></div></div>
  <div class="holding-caption"><span>{{ row.position.quantity }} {{ row.asset?.symbol }}</span><span>{{ members>1?`${members} 个账户 · 点击展开`:row.account?.name }}</span></div>
  <div class="holding-caption"><span>{{ row.position.quote?.price || '—' }} {{ row.position.quote?.currency }} · {{ quoteStatus(row.position.quote) }}</span><time v-if="row.position.quote?.sourceTime">{{ moment.unix(row.position.quote.sourceTime).format('MM-DD HH:mm:ss') }}</time></div>
  <p v-if="row.position.dailyPnl==null" class="holding-caption">{{ row.position.dailyReason || '缺少零点历史行情' }}</p>
 </div>
</template>
<script setup lang="ts">
import moment from 'moment-timezone';
import type {HoldingRow} from '@/lib/investment-mobile.ts';
import {profitClass} from '@/lib/investment-mobile.ts';
import {ledgerMoney} from '@/lib/ledger-display.ts';
import {quoteStatus} from '@/lib/investment-display.ts';
defineProps<{row:HoldingRow;members:number}>();
function money(v:string|null|undefined):string{return ledgerMoney(v,false);}
</script>
<style scoped>
.holding-summary{width:100%;min-width:0}.holding-metrics{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin:15px 0 12px}.holding-metrics>div{display:grid;gap:5px}.holding-metrics span{font-size:11px;color:var(--cy-muted)}.holding-metrics b{font-size:14px;font-weight:500;overflow-wrap:anywhere}.holding-metrics>div:not(:first-child){text-align:right}.holding-caption{display:flex;justify-content:space-between;gap:8px;font-size:10px;color:var(--cy-muted);line-height:1.7;flex-wrap:wrap}.holding-caption time{white-space:nowrap}
</style>
