<template>
 <section class="inv-card inv-form">
  <template v-if="buy">
   <label class="inv-row"><span>付款账户</span><select v-model="cashAccountId" aria-label="付款账户" @change="exchangeRate=currency==='CNY'?'1':''"><option value="" disabled>选择实际付款账户</option><option v-for="a in cashAccounts" :key="a.id" :value="a.id">{{ a.name }} · {{ a.currency }}</option></select></label>
   <label class="inv-row"><span>实际支付（{{ currency }}）</span><input v-model="payment" inputmode="decimal" maxlength="80" placeholder="含手续费的实付金额" aria-label="实际支付" /></label>
   <label class="inv-row"><span>手续费（{{ currency }}）</span><input v-model="fee" inputmode="decimal" maxlength="80" placeholder="默认0" aria-label="买入手续费" /></label>
   <label v-if="currency!=='CNY'" class="inv-row"><span>历史人民币汇率</span><input v-model="exchangeRate" inputmode="decimal" maxlength="80" placeholder="1付款币种折合人民币" aria-label="历史人民币汇率" /></label>
   <p class="inv-caption">填写已确认的成交份额与实际支付，一次保存付款和持仓。净值尚未确认时请待确认后填写。</p>
  </template>
  <label class="inv-row"><span>{{ editing?'校准日期':buy?'成交日期':'持仓日期' }}</span><input v-model="date" type="datetime-local" required aria-label="持仓日期" /></label>
  <div class="inv-row"><span>记入账本</span><BookPicker v-model="bookId" compact /></div>
 </section>
</template>
<script setup lang="ts">
import {computed} from 'vue';
import type {WealthCashAccount} from '@/models/investment.ts';
import BookPicker from '@/components/mobile/BookPicker.vue';
const props=defineProps<{buy:boolean;editing:boolean;cashAccounts:WealthCashAccount[]}>();
const cashAccountId=defineModel<string>('cashAccountId',{required:true}),payment=defineModel<string>('payment',{required:true}),fee=defineModel<string>('fee',{required:true}),exchangeRate=defineModel<string>('exchangeRate',{required:true}),date=defineModel<string>('date',{required:true}),bookId=defineModel<string>('bookId',{required:true});
const currency=computed(()=>props.cashAccounts.find(a=>a.id===cashAccountId.value)?.currency||'CNY');
</script>
