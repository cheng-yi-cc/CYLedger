<template>
 <div class="cy-account-form">
  <section class="asset-form-card">
   <label class="asset-form-row"><span>账户名称</span><input v-model="account.name" placeholder="输入账户名称" maxlength="64" aria-label="账户名称"/></label>
   <button v-if="profile.kind!=='reimbursement'" class="asset-form-row" type="button" @click="showBalance=true"><span>{{account.isLiability?'欠款金额':'账户余额'}}</span><strong>{{balanceText}}</strong></button>
   <label v-if="profile.kind!=='reimbursement'" class="asset-form-row"><span>账户简称</span><input v-model="profile.shortName" placeholder="选填，显示在账单中" maxlength="24" aria-label="账户简称"/></label>
   <label class="asset-form-row"><span>备注</span><input v-model="account.comment" placeholder="选填" maxlength="255" aria-label="账户备注"/></label>
   <label v-if="profile.kind!=='reimbursement'" class="asset-form-row"><span>卡号</span><input v-model="profile.cardNumber" placeholder="选填" maxlength="64" aria-label="卡号"/></label>
  </section>
  <p v-if="account.isLiability" class="asset-form-hint">欠款填正数，溢缴款填负数。</p>
  <section v-if="edit && balanceChanged" class="asset-form-card"><label class="asset-form-row"><span>生成差额收支账单</span><f7-toggle :checked="countAdjustment" @toggle:change="emit('update:countAdjustment',$event)"/></label><p class="asset-form-hint">{{countAdjustment?'差额计入收入或支出。':'保存校准记录，差额不计入收入或支出。'}}</p></section>
  <section v-if="[5,6].includes(account.category)&&profile.kind!=='reimbursement'" class="asset-form-card"><label class="asset-form-row"><span>{{account.category===5?'下次还款日期':'下次收款日期'}}</span><input v-model="profile.debtDueDate" type="date" aria-label="下次借还款日期"/></label></section>
  <section v-if="account.category===3" class="asset-form-card">
   <button class="asset-form-row" type="button" @click="showLimit=true"><span>信用额度</span><strong>{{money(account.creditCardLimit)}}</strong></button>
   <label class="asset-form-row"><span>共享额度</span><select v-model="profile.sharedLimitAccount" aria-label="共享额度账户"><option value="">不共享</option><option v-for="a in creditAccounts" :key="a.id" :value="a.id">{{a.name}}</option></select></label>
   <label class="asset-form-row"><span>账单日</span><select v-model.number="account.creditCardStatementDate" aria-label="账单日"><option :value="0">未设置</option><option v-for="n in 31" :key="n" :value="n">每月{{n}}日</option></select></label>
   <label class="asset-form-row"><span>还款日</span><select v-model.number="profile.repaymentDay" aria-label="还款日" @change="profile.repaymentAfterDays=0"><option :value="0">未设置 / 按账单日</option><option v-for="n in 31" :key="n" :value="n">每月{{n}}日</option></select></label>
   <label v-if="!profile.repaymentDay" class="asset-form-row"><span>账单后还款天数</span><input v-model.number="profile.repaymentAfterDays" type="number" min="0" max="60" aria-label="账单后还款天数" placeholder="未设置"/></label>
   <label class="asset-form-row"><span>账单日消费计入下期</span><f7-toggle :checked="!!profile.statementNextCycle" @toggle:change="profile.statementNextCycle=$event"/></label>
   <button class="asset-form-row" type="button" @click="showAnnual=!showAnnual"><span>年费设置</span><span class="asset-form-value">{{profile.annualFee?`${profile.annualFee}元`:'未设置'}}<f7-icon f7="chevron_right"/></span></button>
   <template v-if="showAnnual">
    <label class="asset-form-row"><span>年费金额</span><input v-model="profile.annualFee" inputmode="decimal" maxlength="16" placeholder="0.00" aria-label="年费金额"/></label>
    <label class="asset-form-row"><span>年费日期</span><input v-model="profile.annualFeeDate" type="date" aria-label="年费日期"/></label>
    <label class="asset-form-row"><span>免年费消费金额</span><input v-model="profile.annualWaiverAmount" inputmode="decimal" maxlength="16" placeholder="0.00" aria-label="免年费消费金额"/></label>
    <label class="asset-form-row"><span>免年费消费次数</span><input v-model.number="profile.annualWaiverCount" type="number" min="0" max="10000" placeholder="0" aria-label="免年费消费次数"/></label>
   </template>
  </section>
  <section class="asset-form-card">
   <p v-if="[2,3].includes(account.category)" class="asset-form-hint bank-logo-hint">根据银行名称自动匹配标志；无法识别时使用所选图标。</p>
   <div class="asset-form-row"><span>图标</span><div class="asset-form-icons"><button type="button" aria-label="选择账户图标" @click="showIcon=true"><item-icon :icon-type="getAccountIconType(account.iconType)" :icon-id="account.icon" :color="account.color"/></button><button type="button" aria-label="选择图标颜色" @click="showColor=true"><i :style="{background:'#'+account.color}"/></button></div></div>
   <button class="asset-form-row" type="button" @click="showNightIcon=true"><span>夜间图标</span><span class="asset-form-value"><item-icon :icon-type="getAccountIconType(profile.nightIconType||0)" :icon-id="profile.nightIcon||account.icon" :color="profile.nightColor||account.color"/><f7-icon f7="chevron_right"/></span></button>
   <button class="asset-form-row" type="button" @click="showGroups=true"><span>分组</span><span class="asset-form-value">{{profile.group||defaultGroup}}<f7-icon f7="chevron_right"/></span></button>
   <f7-link v-if="account.currency==='CNY'&&[1,2,4,8].includes(account.category)&&!profile.kind" class="asset-form-row" :href="edit?`/account/income?id=${account.id}`:undefined" @click="!edit&&emit('income')"><span>收益同步</span><span class="asset-form-value">{{incomeName||'未绑定'}}<f7-icon f7="chevron_right"/></span></f7-link>
   <button v-if="profile.kind!=='reimbursement'" class="asset-form-row" type="button" :disabled="edit && !account.currencyEditable" @click="showCurrency=true"><span>货币单位</span><span class="asset-form-value">{{account.currency}}<f7-icon v-if="!edit || account.currencyEditable" f7="chevron_right"/></span></button>
   <p v-if="edit && !account.currencyEditable" class="asset-form-hint">原币种已锁定，保留已有余额、历史记录及金额规则的含义。</p>
  </section>
  <section class="asset-form-card"><label class="asset-form-row"><span>计入总资产</span><f7-toggle :checked="!profile.excludeFromTotal" @toggle:change="profile.excludeFromTotal=!$event"/></label></section>
  <section class="asset-form-card"><button class="asset-form-row" type="button" @click="showBooks=!showBooks"><span>生效账本</span><span class="asset-form-value">{{disabledBooks.length?'部分账本':'全部账本'}}<f7-icon f7="chevron_right"/></span></button><template v-if="showBooks"><label v-for="book in books.allBooks" :key="book.id" class="asset-form-row"><span>{{book.name}}</span><input type="checkbox" :checked="!disabledBooks.includes(book.id)" @change="toggleBook(book.id,($event.target as HTMLInputElement).checked)"/></label></template></section>
  <number-pad-sheet :min-value="TRANSACTION_MIN_AMOUNT" :max-value="TRANSACTION_MAX_AMOUNT" :currency="account.currency" :flip-negative="account.isLiability" v-model:show="showBalance" v-model="account.numericBalance"/>
  <number-pad-sheet :min-value="0" :max-value="TRANSACTION_MAX_AMOUNT" :currency="account.currency" v-model:show="showLimit" v-model="account.numericCreditCardLimit"/>
  <icon-selection-sheet :all-system-icon-infos="ALL_ACCOUNT_ICONS" :color="account.color" v-model:show="showIcon" v-model:icon-type="account.iconType" v-model="account.icon"/>
  <icon-selection-sheet :all-system-icon-infos="ALL_ACCOUNT_ICONS" :color="profile.nightColor||account.color" v-model:show="showNightIcon" v-model:icon-type="nightIconType" v-model="nightIcon"/>
  <color-selection-sheet :all-system-color-infos="ALL_ACCOUNT_COLORS" v-model:show="showColor" v-model="account.color"/>
  <list-item-selection-popup value-type="item" key-field="currencyCode" value-field="currencyCode" title-field="displayName" title="币种" :enable-filter="true" :items="currencies" v-model:show="showCurrency" v-model="account.currency"/>
  <f7-sheet v-model:opened="showGroups" class="cy-mobile-surface asset-group-sheet" backdrop><div class="asset-sheet-head"><strong>分组</strong><f7-link sheet-close>完成</f7-link></div><div class="asset-group-options"><button v-for="group in groups" :key="group" @click="profile.group=group;showGroups=false">{{group}}</button><label class="asset-form-row"><input v-model="profile.group" placeholder="输入自定义分组" maxlength="40" aria-label="自定义分组"/></label></div></f7-sheet>
 </div>
</template>
<script setup lang="ts">
import { computed, ref, onMounted } from 'vue';
import type { Account } from '@/models/account.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useI18n } from '@/locales/helpers.ts';
import { getAccountIconType } from '@/lib/icon.ts';
import { LedgerDecimal, ledgerMoney } from '@/lib/ledger-display.ts';
import { ALL_ACCOUNT_ICONS } from '@/consts/icon.ts';
import { ALL_ACCOUNT_COLORS } from '@/consts/color.ts';
import { TRANSACTION_MIN_AMOUNT, TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
const props=defineProps<{account:Account;edit:boolean;initialBalance:string;incomeName?:string;disabledBooks:string[];countAdjustment:boolean}>();
const emit=defineEmits<{income:[];'update:disabledBooks':[value:string[]];'update:countAdjustment':[value:boolean]}>();
const accounts=useAccountsStore(),books=useBooksStore(),{getAllCurrencies}=useI18n();
const profile=computed(()=>props.account.assetProfile);
const showBalance=ref(false),showLimit=ref(false),showIcon=ref(false),showColor=ref(false),showNightIcon=ref(false),showCurrency=ref(false),showGroups=ref(false),showBooks=ref(false),showAnnual=ref(false);
const nightIcon=computed({get:()=>profile.value.nightIcon||props.account.icon,set:(v:string)=>{profile.value.nightIcon=v;}});
const nightIconType=computed({get:()=>profile.value.nightIconType||0,set:(v:number)=>{profile.value.nightIconType=v;}});
const defaultGroup=computed(()=>profile.value.kind==='reimbursement'?'报销':profile.value.kind==='prepaid'?'预付账户':profile.value.kind==='secondhand'?'二手资产':[3].includes(props.account.category)?'信贷账户':[5,6].includes(props.account.category)?'债务':[7,9].includes(props.account.category)?'投资理财':'资金账户');
const groups=computed(()=>[...new Set(['资金账户','信贷账户','预付账户','投资理财','二手资产','债务','报销',...Object.values(accounts.allAccountsMap).map(a=>a.assetProfile.group).filter((g):g is string=>!!g)])]);
const currencies=computed(()=>getAllCurrencies());
const creditAccounts=computed(()=>Object.values(accounts.allAccountsMap).filter(a=>a.id!==props.account.id&&a.category===3&&a.currency===props.account.currency&&!a.assetProfile.sharedLimitAccount));
const money=(value:string)=>ledgerMoney(new LedgerDecimal(value).div(100).toString(),false);
const balanceText=computed(()=>money(new LedgerDecimal(props.account.balance).mul(props.account.isLiability?-1:1).toString()));
const balanceChanged=computed(()=>props.account.balance!==props.initialBalance);
function toggleBook(id:string,checked:boolean):void{emit('update:disabledBooks',checked?props.disabledBooks.filter(v=>v!==id):[...props.disabledBooks,id]);}
onMounted(()=>{void books.loadBooks();});
</script>
<style scoped>
.cy-account-form{max-width:640px;margin:auto;padding:14px 14px 30px}.asset-form-card{background:var(--cy-card);border-radius:12px;margin-bottom:14px;overflow:hidden}.asset-form-row{box-sizing:border-box;min-height:51px;padding:10px 16px;display:flex;align-items:center;justify-content:space-between;gap:14px;width:100%;border:0;background:transparent;color:var(--cy-ink);font:inherit;font-size:15px;text-align:left}.asset-form-row>span:first-child{flex-shrink:0}.asset-form-row input:not([type=checkbox]),.asset-form-row select{width:100%;min-width:0;border:0;color:var(--cy-ink);background:transparent;outline:none;text-align:right;font:inherit}.asset-form-row input::placeholder{color:var(--cy-muted)}.asset-form-row strong{font-weight:400;font-variant-numeric:tabular-nums}.asset-form-value{display:flex;align-items:center;justify-content:flex-end;gap:8px;min-width:0;text-align:right;color:var(--cy-muted)}.asset-form-value>.icon{font-size:14px}.asset-form-icons{display:flex;gap:18px;align-items:center}.asset-form-icons button{border:0;background:none;color:inherit;padding:0;width:26px;height:28px}.asset-form-icons i{display:block;width:17px;height:17px;border-radius:50%}.asset-form-hint{font-size:12px;color:var(--cy-muted);line-height:1.6;margin:-5px 16px 15px}.asset-form-card .bank-logo-hint{padding-top:12px}.asset-form-card .asset-form-hint{margin:0 16px 12px}.asset-form-row input[type=checkbox]{width:20px;height:20px;accent-color:var(--cy-accent)}.asset-sheet-head{display:flex;justify-content:space-between;padding:18px}.asset-group-options{padding:0 14px 30px;max-height:55vh;overflow:auto}.asset-group-options>button{background:var(--cy-card);border:0;border-radius:8px;color:var(--cy-ink);padding:12px 20px;margin:5px;font:inherit}.asset-group-sheet{height:auto}
</style>
