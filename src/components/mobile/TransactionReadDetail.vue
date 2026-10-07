<template>
 <main class="bill-detail">
  <section class="bill-card bill-hero">
   <span class="bill-category-icon"><ItemIcon v-if="category?.icon" :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" /><f7-icon v-else :f7="transaction.type===4?'arrow_right_arrow_left':transaction.type===2?'money_dollar_circle':'cart'" /></span>
   <div class="bill-category"><strong>{{ typeName }}{{ category?.name && category.name!==typeName ? '-'+category.name : '' }}</strong></div>
   <div class="bill-amount" :class="transaction.type===2?'cy-income':transaction.type===3?'cy-expense':''"><strong>{{ amount }}</strong><small v-if="(transaction.wallet?.currency || currency)!=='CNY'">{{ transaction.wallet?.currency || currency }}</small></div>
  </section>
  <section class="bill-card">
   <div class="bill-row"><span>账单日期</span><b>{{ date(transaction.time) }}</b></div>
   <div class="bill-row"><span>{{ transaction.type===4?'转出账户':'收支账户' }}</span><b><ItemIcon v-if="source?.icon" :icon-type="getAccountIconType(source.iconType)" :icon-id="source.icon" />{{ transaction.wallet?.accountName || source?.name || '账户已删除' }}</b></div>
   <template v-if="transaction.type===4"><div class="bill-row"><span>转入账户</span><b><ItemIcon v-if="destination?.icon" :icon-type="getAccountIconType(destination.iconType)" :icon-id="destination.icon" />{{ destination?.name || '账户已删除' }}</b></div><div class="bill-row"><span>转入金额</span><b>{{ transaction.hideAmount?'••••':money(transaction.destinationAmount,destination?.currency) }}</b></div><div v-if="nonzero(transaction.transferFeeAmount)" class="bill-row"><span>手续费</span><b>{{ transaction.transferFeeAmount }} {{ currency }}</b></div></template>
   <div v-if="nonzero(transaction.discountAmount)" class="bill-row"><span>优惠金额</span><b>{{ transaction.discountAmount }} {{ currency }}</b></div>
  </section>
  <section class="bill-card">
   <div class="bill-row bill-note"><span>备注</span><b>{{ transaction.comment || '无备注' }}</b></div>
   <div class="bill-row"><span>标签</span><b class="bill-tags"><em v-for="tag in tags" :key="tag">{{ tag }}</em><template v-if="!tags.length">无标签</template></b></div>
   <div class="bill-row"><span>附件</span><b>{{ transaction.pictures?.length ? transaction.pictures.length+' 张' : '无附件' }}</b></div>
   <div v-if="transaction.pictures?.length" class="bill-pictures"><button v-for="picture in transaction.pictures" :key="picture.pictureId" aria-label="查看附件" @click="emit('picture',picture)"><img :src="pictureUrl(picture)" alt="账单附件" /></button></div>
   <div v-if="[2,3].includes(transaction.type)" class="bill-row"><span>不计入收支与预算</span><span class="bill-switch" :class="{on:excluded}" role="img" :aria-label="excluded?'已开启':'未开启'"><i /></span></div>
   <div v-if="transaction.reimbursementAccountId && transaction.reimbursementAccountId!=='0'" class="bill-row"><span>报销账户</span><b>{{ reimbursementName || '账户已删除' }}</b></div>
  </section>
  <details v-if="transaction.monetaryIncome" class="bill-card bill-settlement"><summary class="bill-row"><span>自动收益结算依据</span><f7-icon f7="chevron_down" /></summary>
   <div class="bill-row"><span>收益所属日期</span><b>{{ transaction.monetaryIncome.date }}</b></div>
   <div class="bill-row"><span>货币基金</span><b>{{ transaction.monetaryIncome.code }}</b></div>
   <div class="bill-row"><span>万份收益（元）</span><b>{{ transaction.monetaryIncome.perTenThousand }}</b></div>
   <div class="bill-row"><span>原始计息本金（元）</span><b>{{ transaction.hideAmount?'••••':transaction.monetaryIncome.principal }}</b></div>
  </details>
  <section class="bill-card">
   <div class="bill-row"><span>所属账本</span><b>{{ bookName || '默认账本' }}</b></div>
   <div class="bill-row bill-note"><span>地点信息</span><b>{{ transaction.locationName || (transaction.geoLocation ? `${transaction.geoLocation.latitude.toFixed(5)}, ${transaction.geoLocation.longitude.toFixed(5)}` : '无地点') }}</b></div>
   <div class="bill-row"><span>记录时间</span><b>{{ transaction.createdAt ? date(transaction.createdAt) : '历史记录未保存时间' }}</b></div>
   <div class="bill-row"><span>记录方式</span><b>{{ transaction.monetaryIncome?'货币基金自动收益':transaction.wallet?'钱包收支':transaction.investmentEventId?'投资关联账单':transaction.scheduledCreated?'定时记账':'普通账单' }}</b></div>
   <f7-link v-if="transaction.transferFeeParentId && transaction.transferFeeParentId!=='0'" class="bill-row" :href="`/transaction/detail?id=${transaction.transferFeeParentId}&type=4`"><span>关联账单</span><b>原转账 <f7-icon f7="chevron_right" /></b></f7-link>
   <div v-if="transaction.debtDueDate" class="bill-row"><span>约定还款日期</span><b>{{ transaction.debtDueDate }}</b></div>
  </section>
 </main>
</template>
<script setup lang="ts">
import {computed} from 'vue';
import moment from 'moment-timezone';
import type {Transaction} from '@/models/transaction.ts';
import type {Account} from '@/models/account.ts';
import type {TransactionCategory} from '@/models/transaction_category.ts';
import type {TransactionPictureInfoBasicResponse} from '@/models/transaction_picture_info.ts';
import {LedgerDecimal,ledgerMoney} from '@/lib/ledger-display.ts';
import {getAccountIconType,getCategoryIconType} from '@/lib/icon.ts';
import ItemIcon from './ItemIcon.vue';
const props=defineProps<{transaction:Transaction;source?:Account;destination?:Account;category?:TransactionCategory;tags:string[];bookName?:string;reimbursementName?:string;timeZone:string;pictureUrl:(p:TransactionPictureInfoBasicResponse)=>string|undefined}>();
const emit=defineEmits<{picture:[p:TransactionPictureInfoBasicResponse]}>();
const typeName=computed(()=>({1:'余额调整',2:'收入',3:'支出',4:'转账'} as Record<number,string>)[props.transaction.type]);
const currency=computed(()=>props.source?.currency||'CNY');
const amount=computed(()=>{const t=props.transaction;if(t.hideAmount)return '••••';const value=t.wallet?.amount||new LedgerDecimal(t.sourceAmount).div(100).toString();return (t.type===2?'+':t.type===3?'-':'')+ledgerMoney(value,false);});
const excluded=computed(()=>props.transaction.excludeFromStatistics||!!props.transaction.reimbursementReceiptId||!!props.transaction.reimbursementAccountId&&props.transaction.reimbursementAccountId!=='0');
function date(at:number):string{return moment.unix(at).tz(props.timeZone).format('YYYY年MM月DD日 HH:mm');}
function money(amount:number,currency='CNY'):string{return ledgerMoney(new LedgerDecimal(amount).div(100).toString(),false)+' '+currency;}
function nonzero(value?:string):boolean{return !!value&&!new LedgerDecimal(value).isZero();}
</script>
<style scoped>
.bill-detail{padding:14px 14px 100px;max-width:650px;margin:auto;font-size:14px}.bill-card{background:var(--cy-card);border-radius:11px;margin-bottom:12px;padding:4px 16px;overflow:hidden}.bill-hero{display:flex;align-items:center;gap:12px;padding:18px 16px}.bill-category-icon{width:36px;height:36px;display:grid;place-items:center;border-radius:8px;background:var(--cy-soft);color:var(--cy-accent);flex-shrink:0}.bill-category-icon :deep(.icon),.bill-category-icon :deep(svg){width:25px;height:25px;font-size:25px}.bill-category{min-width:0;display:grid;gap:5px}.bill-category>span{font-size:12px;color:var(--cy-muted)}.bill-category>strong{font-size:15px;font-weight:500;overflow-wrap:anywhere}.bill-amount{margin-left:auto;text-align:right;flex-shrink:0;max-width:62%;overflow-wrap:anywhere}.bill-amount strong{font-size:23px;line-height:1.2;font-weight:600;font-family:Arial,"Microsoft YaHei",sans-serif;letter-spacing:-.7px}.bill-amount small{display:block;font-size:10px;color:var(--cy-muted);margin-top:6px}.bill-row{min-height:48px;display:flex;align-items:center;justify-content:space-between;gap:18px;line-height:1.5;color:inherit;width:100%;box-sizing:border-box}.bill-row>span:first-child{flex-shrink:0}.bill-row>b{font-weight:400;text-align:right;min-width:0;overflow-wrap:anywhere;color:var(--cy-muted);display:flex;justify-content:flex-end;align-items:center;gap:6px}.bill-row>b :deep(.icon),.bill-row>b :deep(svg){width:18px;height:18px;font-size:18px;flex-shrink:0}.bill-note{align-items:flex-start;padding:15px 0}.bill-note>b{white-space:pre-wrap}.bill-tags{flex-wrap:wrap}.bill-tags em{font-style:normal;font-size:12px;background:var(--cy-soft);padding:3px 7px;border-radius:4px}.bill-switch{height:24px;width:41px;flex-shrink:0;background:var(--cy-line);border-radius:20px;padding:3px;box-sizing:border-box}.bill-switch i{display:block;width:18px;height:18px;border-radius:50%;background:var(--cy-muted)}.bill-switch.on{background:var(--cy-accent)}.bill-switch.on i{background:var(--cy-card);transform:translateX(17px)}.bill-pictures{display:flex;gap:8px;padding:0 0 14px;overflow-x:auto}.bill-pictures button{border:0;background:none;padding:0;flex-shrink:0}.bill-pictures img{width:70px;height:70px;object-fit:cover;border-radius:6px}@media(max-width:360px){.bill-detail{padding:10px 10px 100px}.bill-card{padding-inline:12px}.bill-amount strong{font-size:24px}.bill-row{gap:10px;font-size:13px}}
.bill-settlement summary{list-style:none;cursor:pointer}.bill-settlement summary::-webkit-details-marker{display:none}.bill-settlement summary>.icon{font-size:14px;color:var(--cy-muted)}.bill-settlement[open] summary>.icon{transform:rotate(180deg)}
</style>
