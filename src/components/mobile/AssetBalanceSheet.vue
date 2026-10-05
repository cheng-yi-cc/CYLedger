<template>
 <f7-popup :opened="opened" class="cy-mobile-surface" @popup:closed="emit('update:opened',false)"><f7-page class="cy-mobile-surface cy-asset-surface"><f7-navbar title="校准余额"><f7-nav-left><f7-link icon-f7="xmark" :disabled="busy" @click="emit('update:opened',false)"/></f7-nav-left><f7-nav-right><f7-link icon-f7="checkmark" aria-label="保存余额校准" :disabled="busy" @click="save"/></f7-nav-right></f7-navbar><main class="cy-page-body"><p v-if="error" class="cy-message" role="alert">{{error}}</p><section class="cy-panel balance-form"><p>{{account.name}}</p><label>{{account.isLiability?'实际欠款':'实际余额'}}<input v-model="target" inputmode="decimal" aria-label="实际余额" maxlength="17"/></label><div>当前{{account.isLiability?'欠款':'余额'}}<span>{{ledgerMoney(displayBalance,false)}}</span></div></section><section class="cy-panel balance-options"><label><span>生成差额收支账单</span><f7-toggle :checked="countInStatistics" @toggle:change="countInStatistics=$event"/></label><p>{{countInStatistics?'差额计入收入或支出。':'保存校准记录，差额不计入收入或支出。'}}</p></section></main></f7-page></f7-popup>
</template>
<script setup lang="ts">
import {computed,ref,watch} from 'vue';
import type {Account} from '@/models/account.ts';
import {assetTools} from '@/lib/asset-tools.ts';
import {LedgerDecimal,ledgerMoney} from '@/lib/ledger-display.ts';
import {investmentError} from '@/lib/investments.ts';
import {generateRandomUUID} from '@/lib/misc.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
import {useTransactionsStore} from '@/stores/transaction.ts';
const props=defineProps<{account:Account;opened:boolean}>();
const emit=defineEmits<{'update:opened':[value:boolean];saved:[]}>();
const scope=useLedgerScopeStore(),target=ref(''),expected=ref(''),error=ref(''),busy=ref(false),countInStatistics=ref(false);let requestId=generateRandomUUID();
const displayBalance=computed(()=>new LedgerDecimal(props.account.balance).div(100).mul(props.account.isLiability?-1:1).toString());
watch(()=>props.opened,open=>{if(open){target.value=displayBalance.value;expected.value=new LedgerDecimal(props.account.balance).div(100).toString();error.value='';countInStatistics.value=false;requestId=generateRandomUUID();}},{immediate:true});
async function save():Promise<void>{if(busy.value)return;busy.value=true;error.value='';try{if(!/^-?(0|[1-9]\d{0,12})(\.\d{1,2})?$/.test(target.value))throw Error('请输入最多两位小数的金额');await assetTools.adjustBalance({accountId:props.account.id,balance:new LedgerDecimal(target.value).mul(props.account.isLiability?-1:1).toString(),expectedBalance:expected.value,bookId:'',countInStatistics:countInStatistics.value,timeZone:scope.timeZone,requestId});useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});emit('update:opened',false);emit('saved');}catch(e){error.value=investmentError(e);}finally{busy.value=false;}}
</script>
<style scoped>
.balance-form,.balance-options{border:0!important;border-radius:12px!important}.balance-form p{font-size:14px;color:var(--cy-muted);margin-bottom:15px}.balance-form label,.balance-form>div,.balance-options label{display:flex;justify-content:space-between;align-items:center;gap:15px;min-height:44px;font-size:15px}.balance-form input{min-width:0;width:55%;background:none;border:0;color:var(--cy-accent);text-align:right;font-size:23px}.balance-form>div,.balance-options p{font-size:12px;color:var(--cy-muted)}.balance-options p{margin-top:13px;line-height:1.7}
</style>
