<template>
    <f7-page class="cy-mobile-surface cy-account-detail" ptr @ptr:refresh="refresh" @page:afterin="refresh">
        <f7-navbar :title="account?.name || '账户'" back-link="资产"><f7-nav-right><f7-link @click="statistics">统计</f7-link><AccountOptionsMenu v-if="account" :edit-href="`/account/edit?id=${id}`" :items="menuItems" :target="{id,name:account.name,kind:'cash',href:`/transaction/list?accountIds=${id}`}" @action="handleAction" @deleted="deleted" /></f7-nav-right></f7-navbar>
        <main class="cy-page-body">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <template v-if="account">
                <section class="cy-panel cy-account-balance"><button @click="showBalance=true"><span>{{ account.isLiability ? '总欠款' : '账户余额' }}</span><strong>{{ currencySymbol }}{{ ledgerMoney(balance,false) }}</strong></button><div v-if="account.category===3" class="credit-balance-info"><span>可用额度 {{ledgerMoney(availableCredit,false)}}</span><span>总额度 {{ledgerMoney(creditLimit,false)}}</span></div></section>
                <nav class="cy-panel cy-account-entry"><f7-link :href="entryLink(3)"><f7-icon f7="doc_badge_plus" /><span>添加一条新记账</span></f7-link><f7-link :href="entryLink(4)"><f7-icon f7="tray_arrow_up" /><span>转账</span></f7-link></nav>
                <p v-if="loading && !entries.length" class="cy-empty" role="status">正在读取流水…</p>
                <p v-else-if="!entries.length && !error" class="cy-empty">还没有流水</p>
                <LedgerDayList :entries="entries" account-view :partial-last-day="hasMore" />
                <button v-if="hasMore" class="cy-load-more" :disabled="loading" @click="loadAccount(true)">{{ loading ? '加载中…' : '查看更多流水' }}</button>
            </template>
        </main>
        <AssetBalanceSheet v-if="account" v-model:opened="showBalance" :account="account" @saved="refresh" />
    </f7-page>
</template>
<script setup lang="ts">
import {assetMoney as ledgerMoney} from '@/lib/asset-visibility.ts';
import {computed,ref} from 'vue';
import {f7} from 'framework7-vue';
import {investmentError} from '@/lib/investments.ts';
import type {Router} from 'framework7/types';
import {useAccountsStore} from '@/stores/account.ts';
import {useLedgerScopeStore} from '@/stores/ledgerScope.ts';
import {LedgerDecimal,useMobileLedger} from '@/lib/mobile-ledger.ts';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import AccountOptionsMenu from '@/components/mobile/AccountOptionsMenu.vue';
import AssetBalanceSheet from '@/components/mobile/AssetBalanceSheet.vue';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const id=computed(()=>props.f7route.query['id']||''),accounts=useAccountsStore();
const account=computed(()=>accounts.allAccountsMap[id.value]);
const showBalance=ref(false);
const creditMain=computed(()=>accounts.allAccountsMap[account.value?.assetProfile.sharedLimitAccount||'']||account.value);
const creditLimit=computed(()=>new LedgerDecimal(creditMain.value?.creditCardLimit||0).div(100).toString());
const availableCredit=computed(()=>Object.values(accounts.allAccountsMap).filter(a=>a.id===creditMain.value?.id||a.assetProfile.sharedLimitAccount===creditMain.value?.id).reduce((s,a)=>s.plus(a.balance),new LedgerDecimal(0)).div(100).plus(creditLimit.value).toString());
const menuItems=computed(()=>[
 {title:'流水导出',href:`/account/activity?id=${id.value}&mode=export`},
 {title:'流水对账',href:`/account/reconciliation_statements?accountId=${id.value}`},
 {title:'资产明细',href:`/account/activity?id=${id.value}&mode=flow`},
 {title:'余额变动',href:`/account/activity?id=${id.value}&mode=balance`},
 {title:'转账记录',href:`/account/activity?id=${id.value}&mode=transfers`},
 ...(account.value?.category===3?[
  {title:'每期还款账单',href:`/account/credit?id=${id.value}`},
  {title:'分期管理',href:`/account/installments?id=${id.value}`},
  {title:'还款记录',href:`/account/activity?id=${id.value}&mode=repayments`}
 ]:[{title:'定期存款',href:`/assets/tool?tool=deposits&accountId=${id.value}`},{title:'转为信贷账户',action:'credit'}])
]);
function handleAction(action:string):void{if(action!=='credit'||!account.value)return;f7.dialog.confirm('转换后以欠款和可用额度展示，现有余额与流水保留。','转为信贷账户',async()=>{try{const a=account.value!.cloneSelf();a.category=3;a.assetProfile.group='信贷账户';await accounts.saveAccount({account:a,subAccounts:[],isEdit:true,clientSessionId:''});await refresh();}catch(e){error.value=investmentError(e);}});}
const balance=computed(()=>new LedgerDecimal(account.value?.balance||'0').div(100).mul(account.value?.isLiability?-1:1).toString());
const currencySymbol=computed(()=>({CNY:'¥',USD:'$',EUR:'€',HKD:'HK$',JPY:'JP¥'}[account.value?.currency||'CNY'] || `${account.value?.currency} `));
const {entries,loading,error,loadAccount,hasMore}=useMobileLedger({applyFilters:()=>false,accountId:()=>id.value});
async function refresh(done?:()=>void):Promise<void>{await loadAccount();if(typeof done==='function')done();}
function entryLink(type:number):string{return `/transaction/add?type=${type}&accountId=${encodeURIComponent(id.value)}`;}
function statistics():void{useLedgerScopeStore().accountId=id.value;props.f7router.navigate('/statistics');}
function deleted():void{props.f7router.navigate('/investments',{reloadAll:true});}
</script>
<style scoped>
.cy-account-detail :deep(.page-content){padding-bottom:30px}.cy-account-balance{padding:24px;margin-bottom:16px}.cy-account-balance p{color:var(--cy-muted);font-size:15px;margin:0 0 10px}.cy-account-balance strong{font-size:32px;font-weight:600;overflow-wrap:anywhere}.cy-account-entry{display:flex;padding:0;margin-bottom:22px;gap:0}.cy-account-entry a{display:flex;align-items:center;justify-content:center;gap:12px;min-height:62px;color:var(--cy-ink);font-size:15px;flex:1}.cy-account-entry a:first-child{flex:1.8}.cy-account-entry .icon{font-size:24px;color:var(--cy-accent);width:36px;height:28px;line-height:28px;flex:0 0 36px;text-align:center}.cy-account-entry span{display:block;min-width:0;white-space:normal;line-height:1.5}.cy-load-more{display:block;width:100%;border:1px solid var(--cy-line);background:var(--cy-card);border-radius:12px;min-height:46px;font:inherit;color:var(--cy-accent)}
.cy-account-detail .cy-account-balance{padding:18px 16px;border:0;border-radius:12px;margin-bottom:14px}.cy-account-balance button{display:flex;width:100%;align-items:center;gap:10px;background:none;border:0;padding:0;color:var(--cy-ink)}.cy-account-balance button>span{font-size:15px;margin-right:auto;color:var(--cy-muted)}.cy-account-balance strong{font-size:25px;font-weight:500}.cy-account-balance .icon{font-size:16px;color:var(--cy-muted)}.cy-account-entry{border:0!important;border-radius:12px!important;margin-bottom:18px}.cy-account-entry a{min-height:54px;font-size:14px}.credit-balance-info{display:flex;justify-content:space-between;margin-top:18px;color:var(--cy-muted);font-size:12px}
</style>

<style scoped>
.cy-account-balance button{display:block;text-align:left}.cy-account-balance button>span{display:block;font-size:14px}.cy-account-balance button strong{display:block;font-size:25px;margin-top:8px}.cy-account-detail .cy-account-balance{padding:17px 16px}.cy-account-detail :deep(.cy-day-card){border:0;border-radius:12px}
</style>
