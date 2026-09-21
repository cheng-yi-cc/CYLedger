<template>
    <f7-page class="cy-main-page cy-mobile-surface" ptr @ptr:refresh="refresh" @page:afterin="activate" @page:beforeout="deactivate">
        <f7-navbar title="资产"><f7-nav-right><f7-link href="/account/list">管理账户</f7-link></f7-nav-right></f7-navbar>
        <main class="cy-page-body">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <section class="cy-panel cy-hero cy-asset-hero">
                <div><p class="cy-muted">净资产（元）<button class="cy-eye" :aria-label="visible ? '隐藏资产金额' : '显示资产金额'" @click="visible = !visible"><f7-icon :f7="visible ? 'eye' : 'eye_slash'" size="18" /></button></p><p class="cy-major">{{ money(summary?.netAssets) }}</p></div>
                <div class="cy-asset-secondary"><p><span>总资产</span><strong>{{ money(grossAssets) }}</strong></p><p><span>负债</span><strong>{{ money(summary?.liabilities) }}</strong></p></div>
            </section>
            <p v-if="summary?.missingPrices" class="cy-message">有 {{ summary.missingPrices }} 项缺少报价或汇率，净资产暂不可确定；已估值部分净额 {{ money(summary.valuedAssets) }} 元。</p>
            <p v-else-if="summary?.stalePrices" class="cy-message">{{ summary.stalePrices }} 项价格已过期，当前使用最近可用估值。</p>
            <nav class="cy-asset-shortcuts" aria-label="资产分类"><button :aria-pressed="filter === 'all'" @click="filter = 'all'"><f7-icon f7="creditcard" />全部账户<small>余额与明细</small></button><button :aria-pressed="filter === 'debt'" @click="filter = 'debt'"><f7-icon f7="arrow_left_arrow_right" />债务与应收<small>信贷 · 借入 · 借出</small></button><f7-link href="/investments/ledger"><f7-icon f7="chart_bar_alt_fill" />投资理财<small>持仓与投资流水</small></f7-link></nav>
            <p v-if="loading && !summary" class="cy-empty" role="status">正在加载资产…</p>
            <template v-if="summary">
                <details v-for="group in groups" :key="group.name" class="cy-panel cy-account-group" open><summary><h2>{{ group.name }}</h2><strong>{{ money(group.total) }}</strong><f7-icon f7="chevron_down" size="14" /></summary><f7-link v-for="account in group.accounts" :key="account.id" class="cy-account-row" :href="`/transaction/list?accountIds=${account.id}`"><span class="cy-row-icon"><f7-icon :f7="group.icon" size="20" /></span><span class="cy-row-name">{{ account.name }}<small>{{ account.currency }}</small></span><span class="cy-row-amount" :class="{ 'cy-expense': new LedgerDecimal(account.balance).isNegative(), 'cy-income': !new LedgerDecimal(account.balance).isNegative() }">{{ money(account.balance) }}<small v-if="account.currency !== 'CNY'">折合 ¥{{ money(account.value) }}</small></span></f7-link></details>
                <details v-if="filter === 'all' && portfolios.length" class="cy-panel cy-account-group" open><summary><h2>投资理财</h2><strong>{{ money(summary.missingPrices ? null : summary.investmentValue) }}</strong><f7-icon f7="chevron_down" size="14" /></summary><f7-link v-for="account in portfolios" :key="account.id" href="/investments/ledger" class="cy-account-row"><span class="cy-row-icon"><f7-icon f7="chart_pie" size="20" /></span><span class="cy-row-name">{{ account.name }}<small>{{ accountKinds[account.kind] || '其他' }} · {{ account.count }} 项持仓</small></span><span class="cy-row-amount cy-income">{{ money(account.value) }}<small>人民币市值</small></span></f7-link></details>
                <section v-if="!groups.length && (filter === 'debt' || !portfolios.length)" class="cy-panel cy-empty"><p>{{ filter === 'debt' ? '还没有债务或应收账户' : '还没有资产账户' }}</p><f7-link href="/account/add">添加账户</f7-link></section>
                <p class="cy-muted">账户分组金额均折合人民币；账户行保留原币余额。投资行情只更新估值。</p>
            </template>
        </main>
        <template #fixed><f7-link class="cy-fab" popover-open=".cy-add-account" aria-label="添加账户"><f7-icon f7="plus" /></f7-link><LedgerNavigation active="assets" /></template>
        <f7-popover class="cy-add-account"><f7-list><f7-list-item link="/account/add" popover-close title="添加日常账户" /><f7-list-item link="/investments/ledger" popover-close title="添加投资账户" /></f7-list></f7-popover>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, onUnmounted } from 'vue';
import { investments, investmentError } from '@/lib/investments.ts';
import { LedgerDecimal, ledgerMoney, keepUpToDate } from '@/lib/mobile-ledger.ts';
import { useAccountsStore } from '@/stores/account.ts';
import type { WealthSummary, InvestmentAccount } from '@/models/investment.ts';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
const accounts = useAccountsStore();
const accountKinds: Record<string,string> = { EXCHANGE: '交易所', WALLET: '个人钱包', BROKER: '证券账户', OTHER: '其他' };
const summary = ref<WealthSummary>();
const investmentAccounts = ref<InvestmentAccount[]>([]);
const loading = ref(true), error = ref(''), visible = ref(true), filter = ref<'all' | 'debt'>('all');
let timer: ReturnType<typeof setInterval> | undefined;
let requestNumber = 0;
function sum(values: (string | null | undefined)[]): string | null { return values.some(value => value == null) ? null : values.reduce<InstanceType<typeof LedgerDecimal>>((total,value) => total.plus(value!), new LedgerDecimal(0)).toString(); }
const grossAssets = computed(() => summary.value && !summary.value.missingPrices ? sum([summary.value.cashAssets,summary.value.investmentValue]) : null);
const groups = computed(() => [
    { name: '信贷账户', categories: [3], icon: 'creditcard' }, { name: '债务与应收', categories: [5,6], icon: 'arrow_left_arrow_right' },
    { name: '日常理财账户', categories: [7,9], icon: 'chart_bar' }, { name: '资金账户', categories: [1,2,4,8], icon: 'money_yen_circle' }
].filter(group => filter.value === 'all' || group.categories.some(category => [3,5,6].includes(category))).map(group => {
    const items = (summary.value?.cashAccounts || []).filter(account => group.categories.includes(accounts.allAccountsMap[account.id]?.category || 1));
    return { ...group, accounts: items, total: sum(items.map(account => account.value)) };
}).filter(group => group.accounts.length));
const portfolios = computed(() => investmentAccounts.value.map(account => {
    const positions = summary.value?.positions.filter(position => position.accountId === account.id) || [];
    return { ...account, count: positions.length, value: sum(positions.map(position => position.marketValue)) };
}));
function money(value: string | null | undefined): string { return visible.value ? ledgerMoney(value,false) : '••••'; }
async function refresh(done?: () => void): Promise<void> {
    const version = ++requestNumber; loading.value = true; error.value = '';
    try { const [wealth, portfolios] = await Promise.all([investments.summary(), investments.accounts(), accounts.loadAllAccounts({ force: true }).catch(keepUpToDate)]); if (version === requestNumber) { summary.value = wealth; investmentAccounts.value = portfolios; } }
    catch (cause) { if (version === requestNumber) error.value = investmentError(cause); }
    finally { if (version === requestNumber) loading.value = false; if (typeof done === 'function') done(); }
}
function deactivate(): void { clearInterval(timer); }
function activate(): void { deactivate(); void refresh(); timer = setInterval(() => { if (!document.hidden && !loading.value) void refresh(); },15000); }
onUnmounted(deactivate);
</script>
<style scoped>
.cy-asset-hero{display:grid;grid-template-columns:1.1fr 1fr;gap:16px;align-items:center;min-height:128px}.cy-asset-hero .cy-major{font-size:clamp(26px,7vw,36px)}.cy-eye{border:0;background:transparent;color:white;vertical-align:middle;padding:4px 8px}.cy-asset-secondary{display:grid;gap:16px;font-size:12px}.cy-asset-secondary span{color:#ffffffac;display:block;margin-bottom:4px}.cy-asset-secondary strong{font-size:17px;overflow-wrap:anywhere}.cy-asset-shortcuts{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px;margin-bottom:22px}.cy-asset-shortcuts button,.cy-asset-shortcuts a{display:flex;flex-direction:column;align-items:center;gap:9px;border:1px solid var(--cy-line);background:var(--cy-card);border-radius:14px;color:var(--cy-ink);padding:16px 3px;font-size:14px}.cy-asset-shortcuts button[aria-pressed=true]{border-color:var(--cy-accent)}.cy-asset-shortcuts .icon{color:var(--cy-accent);font-size:23px}.cy-asset-shortcuts small{color:var(--cy-muted);font-size:10px}.cy-account-group summary{display:flex;gap:10px;align-items:center;cursor:pointer;list-style:none}.cy-account-group summary::-webkit-details-marker{display:none}.cy-account-group summary h2{flex:1}.cy-account-group summary strong{color:var(--cy-accent);font-size:15px;max-width:45%;overflow-wrap:anywhere}.cy-account-group[open] summary{margin-bottom:4px}.cy-account-group:not([open]) summary .icon{transform:rotate(-90deg)}
</style>
