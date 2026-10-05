<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="load">
        <f7-navbar title="到期事项" back-link="返回"><f7-nav-right><f7-link @click="create">添加</f7-link></f7-nav-right></f7-navbar>
        <main class="cy-page-body">
            <p class="cy-muted">手动维护到期日期和金额，实际还款或赎回时另行记账。</p>
            <p v-if="error" class="cy-message" role="alert">{{ error }}</p>
            <form v-if="editing" class="cy-panel cy-due-form" @submit.prevent="save">
                <h2>{{ editing.id ? '编辑到期事项' : '添加到期事项' }}</h2>
                <label>类型<select v-model="editing.kind" @change="editing.accountId = ''"><option value="repayment">还款</option><option value="deposit">定存到期</option></select></label>
                <label>账本<select v-model="editing.bookId" required><option v-for="book in books.activeBooks" :key="book.id" :value="book.id">{{ book.name }}</option></select></label>
                <label>账户<select v-model="editing.accountId" required aria-label="事项账户"><option value="" disabled>选择{{ editing.kind === 'repayment' ? '信用卡或借入' : '定期存款' }}账户</option><option v-for="account in eligibleAccounts" :key="account.id" :value="account.id">{{ account.name }} · {{ account.currency }}</option></select></label>
                <f7-link href="/account/add">＋ 添加账户</f7-link>
                <label>到期日<input v-model="editing.date" type="date" min="1900-01-01" max="9999-12-31" required aria-label="到期日" /></label>
                <label>金额 <small>{{ currency }}</small><input v-model="editing.amount" inputmode="decimal" maxlength="16" placeholder="0.00" required aria-label="到期金额" /></label>
                <label>备注<input v-model="editing.note" maxlength="200" placeholder="选填" aria-label="事项备注" /></label>
                <div class="cy-actions"><button class="cy-button cy-primary" :disabled="busy">保存事项</button><button class="cy-button" type="button" :disabled="busy" @click="editing = null">取消</button></div>
            </form>
            <label class="cy-due-month">查看月份<input v-model="month" type="month" min="1900-01" max="9999-12" @change="loadItems" /></label>
            <p v-if="loading" class="cy-empty">正在加载…</p>
            <p v-else-if="!scopedItems.length" class="cy-empty">本月还没有到期事项</p>
            <section v-for="item in scopedItems" :key="item.id" class="cy-panel cy-due-item">
                <div class="cy-section-head"><strong>{{ item.kind === 'repayment' ? '还款' : '定存到期' }} · {{ item.accountName }}</strong><span>{{ ledgerMoney(item.amount,false) }} {{ item.currency }}</span></div>
                <p>{{ item.date }} <span :class="item.completed ? 'cy-muted' : 'cy-accent'">{{ item.completed ? '已完成' : item.date < scope.currentDay ? '已逾期' : '待处理' }}</span></p>
                <p v-if="item.note" class="cy-muted">{{ item.note }}</p>
                <div class="cy-actions"><button class="cy-button" :disabled="busy" @click="editing = {...item}">编辑</button><button class="cy-button" :disabled="busy" @click="complete(item)">{{ item.completed ? '恢复待办' : '标记完成' }}</button><button class="cy-button cy-expense" :disabled="busy" @click="remove(item)">删除</button></div>
            </section>
        </main>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { f7 } from 'framework7-vue';
import type { Router } from 'framework7/types';
import { calendarEvents, type CalendarEvent, type CalendarEventInput } from '@/lib/calendar-events.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { keepUpToDate } from '@/lib/mobile-ledger.ts';
import { ledgerMoney } from '@/lib/ledger-display.ts';
import { investmentError } from '@/lib/investments.ts';
const props = defineProps<{ f7route: Router.Route }>();
const accounts = useAccountsStore(), books = useBooksStore(), scope = useLedgerScopeStore();
const initialDate = /^\d{4}-\d{2}-\d{2}$/.test(props.f7route.query['date'] || '') ? props.f7route.query['date']! : scope.selectedDay;
const month = ref(initialDate.slice(0,7)), items = ref<CalendarEvent[]>([]), editing = ref<CalendarEventInput | null>(null);
const busy = ref(false), loading = ref(false), error = ref('');
let initialized = false, generation = 0;
const scopedItems = computed(() => items.value.filter(item => !books.selectedBookIds.length || books.selectedBookIds.includes(item.bookId)));
const eligibleAccounts = computed(() => Object.values(accounts.allAccountsMap).filter(account => account.type === 1 && (account.visible || account.id === editing.value?.accountId) && (editing.value?.kind === 'deposit' ? account.category === 9 : [3,5].includes(account.category))));
const currency = computed(() => accounts.allAccountsMap[editing.value?.accountId || '']?.currency || '');
function create(): void { editing.value = { id: '', kind: 'repayment', bookId: books.defaultBookId, accountId: '', date: month.value === initialDate.slice(0,7) ? initialDate : `${month.value}-01`, amount: '', note: '' }; }
async function loadItems(): Promise<void> {
    const version = ++generation; loading.value = true; error.value = '';
    try { const result = await calendarEvents.list(month.value); if (version === generation) items.value = result; }
    catch (cause) { if (version === generation) { error.value = investmentError(cause); items.value = []; } }
    finally { if (version === generation) loading.value = false; }
}
async function load(): Promise<void> {
    try { await Promise.all([books.loadBooks(), accounts.loadAllAccounts({force:true}).catch(keepUpToDate)]); await loadItems();
        if (!initialized) { initialized = true; const existing = items.value.find(item => item.id === props.f7route.query['id']); if (existing) editing.value = {...existing}; else if (props.f7route.query['add'] === '1') create(); }
    } catch(cause) { error.value = investmentError(cause); }
}
async function save(): Promise<void> {
    if (busy.value || !editing.value) return;
    if (!/^(0|[1-9]\d{0,12})(\.\d{1,2})?$/.test(editing.value.amount) || /^0(?:\.0{1,2})?$/.test(editing.value.amount)) { error.value = '请输入大于零的金额，最多两位小数。'; return; }
    busy.value = true; error.value = '';
    try { const result = await calendarEvents.save({...editing.value}); month.value = result.date.slice(0,7); editing.value = null; await loadItems(); }
    catch(cause) { error.value = investmentError(cause); } finally { busy.value = false; }
}
async function complete(item: CalendarEvent): Promise<void> {
    if (busy.value) return; busy.value = true; error.value = '';
    try { await calendarEvents.complete(item.id,!item.completed); await loadItems(); } catch(cause) { error.value = investmentError(cause); } finally { busy.value = false; }
}
function remove(item: CalendarEvent): void { f7.dialog.confirm('删除这条到期事项？', '删除事项', async () => {
    if (busy.value) return; busy.value = true; error.value = '';
    try { await calendarEvents.remove(item.id); if (editing.value?.id === item.id) editing.value = null; await loadItems(); } catch(cause) { error.value = investmentError(cause); } finally { busy.value = false; }
}); }
</script>
<style scoped>
.cy-due-form{display:flex;flex-direction:column;gap:17px}.cy-due-form h2{margin:0;font-size:18px}.cy-due-form label{display:flex;gap:10px;align-items:center;font-size:14px}.cy-due-form label input,.cy-due-form select{flex:1;min-width:0;padding:10px 7px;border:1px solid var(--cy-line);border-radius:8px;background:var(--cy-card);color:var(--cy-ink);font:inherit}.cy-due-form>a{align-self:flex-start;font-size:13px}.cy-due-month{display:flex;align-items:center;justify-content:space-between;margin:24px 0 15px}.cy-due-month input{color:var(--cy-ink);background:transparent;border:0;font:inherit}.cy-due-item strong{font-size:14px;font-weight:500}.cy-due-item p{font-size:12px;overflow-wrap:anywhere}.cy-due-item p span{margin-left:8px}.cy-due-item .cy-section-head{gap:10px;flex-wrap:wrap}.cy-due-item .cy-section-head>span{font-size:15px}
</style>
