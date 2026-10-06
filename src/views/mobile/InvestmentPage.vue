<template>
    <f7-page class="cy-main-page cy-mobile-surface cy-assets-page" ptr @ptr:refresh="refresh" @page:afterin="activate" @page:beforeout="deactivate">
        <main class="cy-page-body">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <section class="cy-panel cy-hero cy-asset-hero" aria-label="资产总览">
                <button id="cy-asset-menu" class="cy-asset-menu" aria-label="资产更多操作" @click="showMenu = true"><f7-icon f7="ellipsis_vertical" size="20" /></button>
                <div><p class="cy-muted">净资产（元）<button class="cy-eye" :aria-label="visible ? '隐藏资产金额' : '显示资产金额'" @click="visible = !visible"><f7-icon :f7="visible ? 'eye' : 'eye_slash'" size="18" /></button></p><p class="cy-major">{{ money(presentationTotals.net) }}</p></div>
                <div class="cy-asset-secondary"><p><span>总资产</span><strong>{{ money(grossAssets) }}</strong></p><p><span>负债</span><strong>{{ money(totalLiabilities) }}</strong></p></div>
            </section>
            <p v-if="presentationTotals.missing" class="cy-message">有 {{ presentationTotals.missing }} 项缺少报价或汇率，净资产暂不可确定；已估值部分净额 {{ money(presentedValuedAssets) }} 元。</p>
            <p v-else-if="summary?.stalePrices" class="cy-message">{{ summary.stalePrices }} 项价格已过期，当前使用最近可用估值。</p>
            <nav class="cy-asset-shortcuts" aria-label="资产摘要">
                <f7-link href="/assets/reimbursements"><strong>报销</strong><span>可报 <b class="cy-income">{{money(reimbursementPending)}}</b></span><span>已报 <b>{{money(reimbursementPaid)}}</b></span></f7-link>
                <f7-link href="/assets/debts"><strong>债务</strong><span>应付 <b class="cy-expense">{{ money(payable) }}</b></span><span>应收 <b class="cy-income">{{ money(receivable) }}</b></span></f7-link>
                <f7-link href="/investments/ledger"><strong>理财</strong><span>总额 <b>{{ money(investmentTotal) }}</b></span><span>持有盈亏 <b :class="amountClass(visibleInvestmentPnl)">{{ money(visibleInvestmentPnl) }}</b></span></f7-link>
            </nav>
            <p v-if="loading && !summary" class="cy-empty" role="status">正在加载资产…</p>
            <template v-if="summary">
                <details v-for="group in groups" :key="group.name" class="cy-panel cy-account-group" open>
                    <summary><h2>{{ group.name }}</h2><strong :class="amountClass(group.total)">{{ money(group.total) }}</strong><f7-icon f7="chevron_down" size="14" /></summary>
                    <f7-link v-for="account in group.accounts" :key="`${account.portfolio ? 'portfolio' : 'cash'}:${account.id}`" class="cy-account-row" :href="account.href"
                             aria-haspopup="dialog" @pointerdown="startAccountHold($event, account)" @pointermove="moveAccountHold" @pointerup="cancelAccountHold" @pointercancel="cancelAccountHold" @pointerleave="cancelAccountHold"
                             @contextmenu.prevent="openAccountActions(account)"
                             @keydown.shift.f10.prevent="openAccountActions(account)" @click.capture="guardAccountClick">
                        <span class="cy-row-icon" :style="{ color: account.color }"><img v-if="account.platformIcon" :src="account.platformIcon" alt="" width="30" height="30" /><item-icon v-else :icon-type="account.customIcon ? 'user-custom' : 'account'" :icon-id="account.icon" /></span>
                        <div class="cy-account-content"><div class="cy-account-line"><span class="cy-row-name">{{ account.name }}</span><span class="cy-row-amount" :class="amountClass(account.balance)">{{ currencyMoney(account.balance, account.currency) }}</span></div>
                            <template v-if="account.credit"><div class="cy-credit-track" :aria-label="visible ? `已用额度 ${account.credit.percent}%` : '信用额度已隐藏'"><span :style="{ width: visible ? `${account.credit.percent}%` : '0%' }"></span></div><small>可用：{{ currencyMoney(account.credit.available, account.currency) }}<span class="cy-credit-limit">额度 {{ money(account.credit.limit) }}</span></small></template>
                            <small v-else-if="account.portfolio">{{ account.subtitle }}<template v-if="account.currency!=='CNY'"> · 折合 ¥{{ money(account.value) }}</template></small>
                            <small v-else-if="account.currency !== 'CNY'">{{ account.currency }} · 折合 ¥{{ money(account.value) }}</small>
                            <small v-else-if="account.category === 3">未设置信用额度</small><small v-if="account.excluded">不计入总资产</small>
                        </div>
                    </f7-link>
                </details>
            </template>
        </main>
        <template #fixed><f7-link class="cy-fab" aria-label="添加账户" @click="showAdd = true"><f7-icon f7="plus" /></f7-link><LedgerNavigation active="assets" /></template>
        <f7-popover target-el="#cy-asset-menu" v-model:opened="showMenu"><f7-list><f7-list-item v-for="item in assetMenu" :key="item.tool" :link="`/assets/tool?tool=${item.tool}`" popover-close :title="item.name"><template #media><f7-icon :f7="item.icon" /></template></f7-list-item></f7-list></f7-popover>
        <f7-sheet v-model:opened="showAdd" class="cy-account-picker cy-mobile-surface" backdrop swipe-to-close>
            <div class="cy-picker-grip" aria-hidden="true"></div>
            <div class="cy-account-presets"><section v-for="group in accountPresetGroups" :key="group.name"><h2>{{ group.name }}</h2><div class="cy-preset-grid"><f7-link v-for="preset in group.items" :key="preset.id" :href="isCryptoAccount(preset.kind) ? `/crypto/add?kind=${preset.kind}` : preset.kind ? `/investments/record?action=account&kind=${preset.kind}` : `/account/add?preset=${preset.id}`" sheet-close><span :style="{ color: preset.color }"><item-icon icon-type="account" :icon-id="preset.icon" /></span><span>{{ preset.name }}</span></f7-link></div></section></div>
        </f7-sheet>
        <AccountDeletionSheet v-if="selectedAccount" v-model:opened="showAccountActions" :target="{id:selectedAccount.id,name:selectedAccount.name,kind:selectedAccount.portfolio?'portfolio':'cash',href:selectedAccount.portfolio?selectedAccount.href:`/transaction/list?accountIds=${selectedAccount.id}`}" @busy="deleting=$event" @deleted="refresh" />
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, onUnmounted } from 'vue';
import { walletCurrency, walletValue } from '@/lib/wallet-entry.ts';
import { investments, investmentError } from '@/lib/investments.ts';
import { LedgerDecimal, ledgerMoney, keepUpToDate } from '@/lib/mobile-ledger.ts';
import { isCryptoAccount, platformIcon } from '@/lib/crypto-platforms.ts';
import { accountPresetGroups } from '@/lib/account-presets.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { IconType } from '@/core/icon.ts';
import type { WealthSummary, InvestmentAccount, WealthCashAccount } from '@/models/investment.ts';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import AccountDeletionSheet from '@/components/mobile/AccountDeletionSheet.vue';
import {assetItems,assetTotals,assetAccountGroup,groupAssetAccountRows} from '@/lib/asset-tools.ts';
import {reimbursements,type ReimbursementClaim} from '@/lib/reimbursements.ts';
import {useAssetToolsStore} from '@/stores/assetTools.ts';
import {useBooksStore} from '@/stores/books.ts';
const accounts = useAccountsStore();
const preferences=useAssetToolsStore(),books=useBooksStore();
const assetMenu=[{tool:'search',name:'搜索账户',icon:'search'},{tool:'reminders',name:'还款提醒',icon:'calendar_badge_plus'},{tool:'books',name:'生效账本',icon:'book'},{tool:'deposits',name:'定期存款',icon:'lock'},{tool:'hidden',name:'资产隐藏',icon:'eye_slash'},{tool:'distribution',name:'资产分布',icon:'chart_pie'},{tool:'more',name:'更多数据',icon:'ellipsis'}];
const presentedItems=computed(()=>assetItems(summary.value,investmentAccounts.value,accounts.allAccountsMap,preferences.preferences).filter(a=>!a.hidden&&(!books.selectedBookIds.length||books.selectedBookIds.some(id=>preferences.available(a.portfolio?'portfolio':'cash',a.id,id)))));
const shownKeys=computed(()=>new Set(presentedItems.value.map(a=>a.key)));
const presentationTotals=computed(()=>assetTotals(presentedItems.value));
const presentedValuedAssets=computed(()=>sum(presentedItems.value.filter(a=>a.value!=null&&!a.excluded).map(a=>a.value)));
const visibleInvestmentPnl=computed(()=>sum(presentedItems.value.filter(a=>a.portfolio).map(a=>a.unrealizedPnl)));

const accountKinds: Record<string,string> = { EXCHANGE: '加密货币交易所', WALLET: '加密钱包', BROKER: '证券账户', OTHER: '其他' };
const summary = ref<WealthSummary>();
const claims=ref<ReimbursementClaim[]>([]);
const investmentAccounts = ref<InvestmentAccount[]>([]);


const loading = ref(true), error = ref(''), visible = ref(true);
const showMenu = ref(false), showAdd = ref(false);
const showAccountActions = ref(false), deleting = ref(false);
const selectedAccount = ref<AccountRow>();
let holdTimer: ReturnType<typeof setTimeout> | undefined;
let holdStart: {x:number; y:number; pointerId:number} | undefined;
function cancelAccountHold(): void { clearTimeout(holdTimer); holdTimer = undefined; holdStart = undefined; }
function startAccountHold(event: PointerEvent, account: AccountRow): void {
    cancelAccountHold();
    if (!event.isPrimary || event.button !== 0 || showAccountActions.value || deleting.value) return;
    holdStart = {x:event.clientX,y:event.clientY,pointerId:event.pointerId};
    holdTimer = setTimeout(() => { cancelAccountHold(); openAccountActions(account); },650);
}
function moveAccountHold(event: PointerEvent): void {
    if (holdStart && (event.pointerId !== holdStart.pointerId || Math.hypot(event.clientX-holdStart.x,event.clientY-holdStart.y) > 10)) cancelAccountHold();
}
function openAccountActions(account: AccountRow): void {
    if (deleting.value || showAccountActions.value) return;
    selectedAccount.value = account; showAccountActions.value = true;
}
function guardAccountClick(event: MouseEvent): void {
    if (showAccountActions.value || deleting.value) { event.preventDefault(); event.stopPropagation(); }
}
let timer: ReturnType<typeof setInterval> | undefined;
let requestNumber = 0;
function sum(values: (string | null | undefined)[]): string | null { return values.some(value => value == null) ? null : values.reduce<InstanceType<typeof LedgerDecimal>>((total,value) => total.plus(value!), new LedgerDecimal(0)).toString(); }
const grossAssets = computed(() => presentationTotals.value.assets);
const totalLiabilities = computed(() => presentationTotals.value.liabilities);
function debtTotal(categories: number[], negative: boolean): string | null {
    if (!summary.value) return null;
    const items = summary.value.cashAccounts.filter(a => accounts.allAccountsMap[a.id]?.assetProfile.kind!=='reimbursement' && shownKeys.value.has('cash:'+a.id) && categories.includes(accounts.allAccountsMap[a.id]?.category || 0) && (negative ? new LedgerDecimal(a.balance).lt(0) : new LedgerDecimal(a.balance).gt(0)));
    return sum(items.map(a => a.value == null ? null : new LedgerDecimal(a.value).abs().toString()));
}
const visibleClaims=computed(()=>claims.value.filter(c=>shownKeys.value.has('cash:'+c.accountId)&&(!books.selectedBookIds.length||books.selectedBookIds.includes(c.bookId))));
function reimbursementValue(claim:ReimbursementClaim,value:string):string|null {
    if(claim.currency==='CNY')return value;
    const a=summary.value?.cashAccounts.find(a=>a.id===claim.accountId);
    return a?.value!=null && !new LedgerDecimal(a.balance).isZero()?new LedgerDecimal(value).mul(a.value).div(a.balance).toString():new LedgerDecimal(value).isZero()?'0':null;
}
const reimbursementPending=computed(()=>sum(visibleClaims.value.filter(c=>!c.closed).map(c=>reimbursementValue(c,c.pending))));
const reimbursementPaid=computed(()=>sum(visibleClaims.value.map(c=>reimbursementValue(c,c.paid))));
const payable = computed(() => debtTotal([5], true)), receivable = computed(() => debtTotal([6], false));
const investmentTotal = computed(() => summary.value ? sum(presentedItems.value.filter(a=>a.portfolio||[7,9].includes(a.category)&&a.kind!=='secondhand').map(a=>a.value)) : null);
interface AccountRow {
    id: string; name: string; currency: string; balance: string | null; value: string | null; icon: string; customIcon: boolean; color: string; category: number; href: string;
    platformIcon?: string; portfolio?: boolean; subtitle?: string; group?:string; excluded?:boolean; credit?: { available: string; limit: string; percent: number };
}
function cashRow(item: WealthCashAccount): AccountRow {
    const account = accounts.allAccountsMap[item.id];
    const profile=account?.assetProfile, night=document.documentElement.classList.contains('dark');
    const row: AccountRow = { ...item, category: account?.category || 0, icon: (night&&profile?.nightIcon)||account?.icon||'1', customIcon: (night&&profile?.nightIcon?profile.nightIconType:account?.iconType)===IconType.UserCustom, color:'#'+((night&&profile?.nightColor)||account?.color||'68bfae'), href:profile?.kind==='reimbursement'?`/assets/reimbursements?id=${item.id}`:`/account/detail?id=${item.id}`,group:assetAccountGroup(account),excluded:!!profile?.excludeFromTotal };
    const main=account?.assetProfile.sharedLimitAccount?accounts.allAccountsMap[account.assetProfile.sharedLimitAccount]:account;
    if (account?.category === 3 && main && new LedgerDecimal(main.creditCardLimit).gt(0)) {
        const limit = new LedgerDecimal(main.creditCardLimit).div(100);
        const shared=Object.values(accounts.allAccountsMap).filter(a=>a.id===main.id||a.assetProfile.sharedLimitAccount===main.id);
        const balance = shared.reduce((s,a)=>s.plus(a.balance),new LedgerDecimal(0)).div(100);
        row.credit = { limit: limit.toString(), available: limit.plus(balance).toString(), percent: LedgerDecimal.min(100,LedgerDecimal.max(0,balance.negated().div(limit).mul(100))).toNumber() };
    }
    return row;
}
const portfolios = computed<AccountRow[]>(() => investmentAccounts.value.map(account => {
    const positions = summary.value?.positions.filter(position => position.accountId === account.id && new LedgerDecimal(position.quantity).gt(0)) || [];
    const value = sum(positions.map(position => position.marketValue));
    return { id: account.id, name: account.name, balance: walletValue(positions,walletCurrency(account),summary.value), value, currency: walletCurrency(account), category: 7, icon: account.kind === 'EXCHANGE' ? '1500' : account.kind === 'WALLET' ? '1' : '801', customIcon: false, color: account.kind === 'EXCHANGE' ? '#bf82ca' : '', portfolio: true, platformIcon: platformIcon(account.platform, account.kind), subtitle: `${accountKinds[account.kind] || '其他'} · ${positions.length} 项持仓`, href: isCryptoAccount(account.kind) ? `/crypto/account?id=${encodeURIComponent(account.id)}` : `/investments/ledger?accountId=${encodeURIComponent(account.id)}` };
}));
const groups = computed(() => {
    const rows=(summary.value?.cashAccounts||[]).filter(a=>shownKeys.value.has('cash:'+a.id)&&![5,6].includes(accounts.allAccountsMap[a.id]?.category||0)).map(cashRow);
    rows.push(...portfolios.value.filter(a=>shownKeys.value.has('portfolio:'+a.id)).map(a=>({...a,group:'投资理财'})));
    return groupAssetAccountRows(rows).map(group=>({...group,total:sum(group.accounts.map(a=>a.value))}));
});
function money(value: string | null | undefined): string { return visible.value ? ledgerMoney(value,false) : '••••'; }
function currencyMoney(value: string | null, currency: string): string { return `${({ CNY:'¥',USD:'$',EUR:'€',HKD:'HK$',JPY:'JP¥' } as Record<string,string>)[currency] || currency + ' '}${money(value)}`; }
function amountClass(value: string | null | undefined): string { return value != null && new LedgerDecimal(value).lt(0) ? 'cy-expense' : 'cy-income'; }
async function refresh(done?: () => void): Promise<void> {
    const version = ++requestNumber; loading.value = true; error.value = '';
    try { const [wealth, portfolios, reimbursementRows] = await Promise.all([investments.summary(), investments.accounts(), reimbursements.list(), accounts.loadAllAccounts({ force: true }).catch(keepUpToDate), preferences.load(true), books.loadBooks()]); if (version === requestNumber) { summary.value = wealth; investmentAccounts.value = portfolios;claims.value=reimbursementRows; } }
    catch (cause) { if (version === requestNumber) error.value = investmentError(cause); }
    finally { if (version === requestNumber) loading.value = false; if (typeof done === 'function') done(); }
}
function deactivate(): void { clearInterval(timer); cancelAccountHold(); if (!deleting.value) showAccountActions.value = false; }
function activate(): void { deactivate(); void refresh(); timer = setInterval(() => { if (!document.hidden && !loading.value && !showAdd.value && !showAccountActions.value && !deleting.value) void refresh(); },15000); }
onUnmounted(deactivate);
</script>
<style scoped>


.cy-assets-page :deep(.page-content){padding-top:0!important}
.cy-account-row{user-select:none;-webkit-touch-callout:none}
.cy-asset-hero{display:grid;grid-template-columns:1.1fr 1fr;gap:14px;align-items:center;min-height:120px;box-sizing:border-box;padding:22px 26px 22px 18px;margin-top:4px}.cy-asset-hero .cy-major{font-size:clamp(26px,7vw,36px)}.cy-eye{border:0;background:transparent;color:white;vertical-align:middle;padding:4px}.cy-asset-menu{position:absolute;right:4px;top:6px;border:0;background:none;color:white;padding:7px}.cy-asset-secondary{display:grid;gap:15px;font-size:12px}.cy-asset-secondary p{display:flex;align-items:baseline;gap:8px;flex-wrap:wrap}.cy-asset-secondary span{color:#ffffffac}.cy-asset-secondary strong{font-size:16px;overflow-wrap:anywhere}.cy-asset-shortcuts{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;margin-bottom:18px}.cy-asset-shortcuts button,.cy-asset-shortcuts a{display:flex;flex-direction:column;align-items:center;gap:6px;border:1px solid var(--cy-line);background:var(--cy-card);border-radius:16px;color:var(--cy-ink);padding:14px 8px;font-size:12px;line-height:1.4}.cy-asset-shortcuts button[aria-pressed=true]{border-color:var(--cy-accent)}.cy-asset-shortcuts strong{font-size:18px;margin-bottom:2px}.cy-asset-shortcuts span{color:var(--cy-muted);max-width:100%;overflow-wrap:anywhere}.cy-asset-shortcuts b{font-weight:500;color:var(--cy-ink)}.cy-asset-shortcuts b.cy-expense{color:var(--cy-expense)}.cy-asset-shortcuts b.cy-income{color:var(--cy-accent)}
.cy-account-group summary{display:flex;gap:10px;align-items:center;cursor:pointer;list-style:none;min-height:26px}.cy-account-group summary::-webkit-details-marker{display:none}.cy-account-group summary h2{flex:1}.cy-account-group summary strong{color:var(--cy-accent);font-size:16px;max-width:45%;overflow-wrap:anywhere}.cy-account-group summary strong.cy-expense{color:var(--cy-expense)}.cy-account-group[open] summary{margin-bottom:5px}.cy-account-group:not([open]) summary .icon{transform:rotate(-90deg)}.cy-account-group .cy-row-icon{background:transparent;border-radius:0;flex-basis:26px}.cy-row-icon :deep(i){font-size:28px!important;color:inherit!important}.cy-account-content{flex:1;min-width:0}.cy-account-line{display:flex;align-items:baseline;gap:8px}.cy-account-row{width:100%;box-sizing:border-box}.cy-account-row .cy-row-name{font-size:16px}.cy-credit-track{height:4px;border-radius:8px;overflow:hidden;background:var(--cy-line);margin:10px 0 6px}.cy-credit-track span{display:block;height:100%;background:var(--cy-accent)}.cy-credit-limit{float:right}.cy-filter-note{display:flex;justify-content:space-between;align-items:center;margin:0 0 15px;font-size:12px;color:var(--cy-muted)}.cy-filter-note button{background:none;border:0;color:var(--cy-accent)}
.cy-account-picker{--f7-sheet-bg-color:var(--cy-card);height:auto;max-height:85dvh;background:var(--cy-card);color:var(--cy-ink);border-radius:22px 22px 0 0;box-shadow:none!important;backdrop-filter:none!important}.cy-account-picker :deep(.sheet-modal-inner){padding-top:0;max-height:85dvh;box-sizing:border-box;overflow:auto}.cy-picker-heading{display:flex;justify-content:space-between;align-items:center;padding:17px 18px 12px;position:sticky;top:0;background:var(--cy-card);z-index:1;border-bottom:1px solid var(--cy-line)}.cy-picker-heading a{color:var(--cy-accent);font-size:14px;padding:4px}.cy-account-presets{padding:16px 16px calc(24px + env(safe-area-inset-bottom))}.cy-account-presets section+section{margin-top:23px}.cy-account-presets h2{font-size:17px;margin-bottom:17px}.cy-preset-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:17px 5px}.cy-preset-grid a{display:flex;flex-direction:column;gap:7px;text-align:center;color:inherit;white-space:normal;font-size:12px;line-height:1.35}.cy-preset-grid a>span:first-child{height:30px;display:grid;place-items:center}.cy-preset-grid :deep(i){font-size:30px!important;color:inherit!important}
.cy-assets-page .cy-asset-hero{background:var(--cy-bg);color:var(--cy-ink);padding:22px 16px 20px;min-height:112px;margin-bottom:10px}.cy-assets-page .cy-asset-hero:before{display:none}.cy-assets-page .cy-asset-hero .cy-muted,.cy-asset-secondary span{color:var(--cy-muted)}.cy-eye,.cy-asset-menu{color:var(--cy-muted)}.cy-assets-page .cy-asset-shortcuts{grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin-bottom:14px}.cy-asset-shortcuts button,.cy-asset-shortcuts a{border:0;border-radius:11px;padding:12px 5px;gap:3px;font-size:11px;align-items:flex-start;padding-left:12px}.cy-asset-shortcuts strong{font-size:15px;font-weight:500;margin-bottom:4px}.cy-assets-page .cy-account-group{border:0;border-radius:12px;padding:12px 14px;margin-bottom:13px}.cy-account-group summary h2{font-size:14px;font-weight:400;color:var(--cy-muted)}.cy-account-group summary strong{font-size:14px;font-weight:400}.cy-assets-page .cy-account-row{border:0;padding:13px 0;min-height:55px}.cy-assets-page .cy-account-row:last-child{padding-bottom:6px}.cy-account-row .cy-row-name{font-size:15px}.cy-account-row .cy-row-amount{font-weight:400}.cy-row-icon :deep(i){font-size:25px!important}.cy-picker-grip{height:4px;width:34px;border-radius:3px;background:var(--cy-line);margin:9px auto}.cy-account-presets{padding-top:7px}.cy-account-presets h2{font-size:14px;font-weight:400}.cy-account-presets section+section{margin-top:20px}
</style>
