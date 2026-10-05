<template>
    <section class="cy-debt-entry" aria-label="债务记账">
        <nav class="cy-debt-operations" aria-label="债务操作"><button v-for="item in DEBT_OPERATIONS" :key="item.value" :aria-pressed="operation === item.value" :disabled="disabled" @click="operation = item.value">{{ item.label }}</button></nav>
        <label class="cy-debt-field">{{ isBorrowing ? '借款对象' : '借出对象' }}<select v-model="counterpartyId" :disabled="disabled" aria-label="债务往来对象"><option value="">选择对象</option><option v-for="account in counterparties" :key="account.id" :value="account.id">{{ account.name }} · {{ account.currency }}</option></select></label>
        <button v-if="!creating" class="cy-debt-add" :disabled="disabled" @click="creating = true">＋ 新建往来对象</button>
        <form v-else class="cy-debt-new" @submit.prevent="createCounterparty"><input v-model="name" maxlength="50" placeholder="对象姓名或事项" aria-label="往来对象名称" required /><button :disabled="busy || !name.trim()">创建</button><button type="button" :disabled="busy" @click="creating = false">取消</button></form>
        <p v-if="error" class="cy-debt-error" role="alert">{{ error }}</p>
        <label class="cy-debt-field">资金账户<select v-model="cashId" :disabled="disabled" aria-label="债务资金账户"><option value="">选择账户</option><option v-for="account in cashAccounts" :key="account.id" :value="account.id">{{ account.name }} · {{ account.currency }}</option></select></label>
        <f7-link class="cy-debt-add" :class="{ disabled }" href="/account/add">＋ 添加资金账户</f7-link>
        <div v-if="counterparty" class="cy-debt-outstanding">
            <span>{{ isBorrowing ? '剩余欠款' : '剩余应收' }}<strong>{{ money(outstanding) }} <small>{{ counterparty.currency }}</small></strong></span>
            <span>本次记账后<strong>{{ money(projected) }} <small>{{ counterparty.currency }}</small></strong></span>
        </div>
        <f7-link class="cy-debt-history" v-if="counterparty" :href="`/transaction/list?accountIds=${counterparty.id}`">查看往来记录</f7-link>
    </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import Decimal from 'decimal.js';
import { Account, type AccountInfoResponse } from '@/models/account.ts';
import { AccountCategory } from '@/core/account.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { getCurrentUnixTime } from '@/lib/datetime.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { investmentError } from '@/lib/investments.ts';
import { DEBT_OPERATIONS, debtAccountCategory, debtIsSource, debtPrincipal, isDebtCashAccount, projectedDebtPrincipal, type DebtOperation } from '@/lib/ledger-debt.ts';

const props = defineProps<{ accounts: Account[]; categories: Pick<TransactionCategory, 'id' | 'name' | 'subCategories'>[]; amount: number; disabled: boolean; previousAccountId?: string; previousImpact?: string }>();
const operation = defineModel<DebtOperation>('operation', { required: true });
const sourceId = defineModel<string>('sourceId', { required: true });
const destinationId = defineModel<string>('destinationId', { required: true });
const categoryId = defineModel<string>('categoryId', { required: true });
const store = useAccountsStore(), creating = ref(false), busy = ref(false), name = ref(''), error = ref('');
const isBorrowing = computed(() => debtAccountCategory(operation.value) === AccountCategory.DebtAccount.type);
const counterparties = computed(() => props.accounts.filter(account => account.category === debtAccountCategory(operation.value)));
const counterpartyId = computed({ get: () => debtIsSource(operation.value) ? sourceId.value : destinationId.value, set: (id: string) => { if (debtIsSource(operation.value)) sourceId.value = id; else destinationId.value = id; } });
const cashId = computed({ get: () => debtIsSource(operation.value) ? destinationId.value : sourceId.value, set: (id: string) => { if (debtIsSource(operation.value)) destinationId.value = id; else sourceId.value = id; } });
const counterparty = computed(() => counterparties.value.find(account => account.id === counterpartyId.value));
const cashAccounts = computed(() => props.accounts.filter(account => isDebtCashAccount(account) && (!counterparty.value || account.currency === counterparty.value.currency)));
const outstanding = computed(() => counterparty.value ? debtPrincipal(counterparty.value) : '0');
const projected = computed(() => counterparty.value ? projectedDebtPrincipal(counterparty.value, operation.value, String(props.amount), props.previousAccountId === counterparty.value.id ? props.previousImpact : '0') : '0');
function money(cents: string): string { return new Decimal(cents).div(100).toFixed(2); }
function normalizeAccounts(previousOperation: DebtOperation): void {
    const oldParty = debtIsSource(previousOperation) ? sourceId.value : destinationId.value;
    const oldCash = debtIsSource(previousOperation) ? destinationId.value : sourceId.value;
    const party = counterparties.value.some(account => account.id === oldParty) ? oldParty : '';
    const cash = props.accounts.find(account => account.id === oldCash && isDebtCashAccount(account)) || props.accounts.find(account => account.id === oldParty && isDebtCashAccount(account));
    counterpartyId.value = party;
    cashId.value = cash && (!counterparty.value || cash.currency === counterparty.value.currency) ? cash.id : cashAccounts.value[0]?.id || '';
}
watch(operation, (value, previous) => {
    normalizeAccounts(previous || value);
    const label = DEBT_OPERATIONS.find(item => item.value === value)?.label;
    if (previous || !props.categories.some(parent => parent.subCategories?.some(child => child.id === categoryId.value))) {
        categoryId.value = props.categories.flatMap(parent => parent.subCategories || []).find(child => child.name === label)?.id || props.categories[0]?.subCategories?.[0]?.id || '';
    }
}, { immediate: true });
watch(cashAccounts, accounts => { if (!accounts.some(account => account.id === cashId.value)) cashId.value = accounts[0]?.id || ''; });
async function createCounterparty(): Promise<void> {
    if (busy.value || !name.value.trim()) return;
    busy.value = true; error.value = '';
    try {
        const currency = props.accounts.find(account => account.id === cashId.value)?.currency || 'CNY';
        const account = Account.createNewAccount(isBorrowing.value ? AccountCategory.DebtAccount : AccountCategory.Receivables, currency, getCurrentUnixTime());
        account.name = `${isBorrowing.value ? '借入' : '借出'} · ${name.value.trim()}`;
        const saved: AccountInfoResponse = await store.saveAccount({ account, subAccounts: [], isEdit: false, clientSessionId: generateRandomUUID() });
        counterpartyId.value = saved.id; name.value = ''; creating.value = false;
    } catch (cause) { error.value = investmentError(cause); }
    finally { busy.value = false; }
}
</script>

<style scoped>
.cy-debt-entry{padding:18px 16px;display:flex;flex-direction:column;gap:17px}.cy-debt-operations{display:grid;grid-template-columns:repeat(4,1fr);gap:7px}.cy-debt-operations button{padding:11px 0;border:0;background:var(--cy-soft);border-radius:9px;color:var(--cy-muted);font-size:14px}.cy-debt-operations button[aria-pressed=true]{background:var(--cy-accent);color:var(--cy-card)}
.cy-debt-field{display:flex;align-items:center;gap:15px;font-size:14px}.cy-debt-field select{flex:1;min-width:0;padding:10px 6px;background:var(--cy-card);border:1px solid var(--cy-line);border-radius:8px;color:var(--cy-ink)}.cy-debt-add{align-self:flex-start;padding:0;border:0;background:transparent;color:var(--cy-accent);font-size:13px}.cy-debt-new{display:flex;gap:7px}.cy-debt-new input{min-width:0;flex:1;border:1px solid var(--cy-line);background:var(--cy-card);border-radius:8px;padding:9px}.cy-debt-new button{border:0;background:var(--cy-soft);border-radius:8px;color:var(--cy-accent);padding:8px}.cy-debt-error{font-size:12px;color:var(--cy-expense)}
.cy-debt-outstanding{display:grid;grid-template-columns:1fr 1fr;gap:16px;padding:17px 12px;background:var(--cy-soft);border-radius:12px;font-size:12px;color:var(--cy-muted)}.cy-debt-outstanding strong{display:block;overflow-wrap:anywhere;font-size:22px;color:var(--cy-ink);margin-top:7px;font-variant-numeric:tabular-nums}.cy-debt-outstanding small{font-size:10px;font-weight:400}.cy-debt-history{font-size:12px;align-self:flex-start}
</style>
