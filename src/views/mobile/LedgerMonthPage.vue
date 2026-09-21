<template>
    <f7-page class="cy-main-page cy-mobile-surface" ptr @ptr:refresh="refresh" @page:afterin="refresh()">
        <f7-navbar :title="isCalendar ? '日历' : '日常账本'">
            <f7-nav-right><f7-link href="/transaction/list" icon-f7="search" aria-label="搜索和筛选账单" /><f7-link :href="addLink" icon-f7="plus" aria-label="添加记账" /></f7-nav-right>
        </f7-navbar>
        <main class="cy-page-body">
            <div class="cy-month-control"><button class="cy-icon-button" aria-label="上个月" @click="shift(-1)">‹</button><input v-model="month" type="month" aria-label="账单月份" @change="monthChanged" /><button class="cy-icon-button" aria-label="下个月" @click="shift(1)">›</button><button v-if="isCalendar" class="cy-icon-button" @click="today">今</button></div>
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <template v-if="!error">
                <section v-if="!isCalendar" class="cy-panel cy-hero" aria-label="月度收支概览">
                    <div class="cy-hero-stats"><div><p class="cy-muted">月收入</p><strong>{{ display(totals.income) }}</strong></div><div><p class="cy-muted">月结余</p><strong>{{ display(totals.balance) }}</strong></div><div><p class="cy-muted">本月账单</p><strong>{{ loading ? '—' : entries.length }} <small>笔</small></strong></div></div>
                    <p class="cy-muted">月支出（元）</p><p class="cy-major">{{ display(totals.expense) }}</p>
                </section>
                <section v-else class="cy-panel cy-calendar-panel">
                    <div class="cy-calendar-totals"><span class="cy-income">收 {{ display(totals.income) }}</span><span class="cy-expense">支 {{ display(totals.expense) }}</span><span>余 {{ display(totals.balance) }}</span></div>
                    <div class="cy-calendar-grid" role="group" aria-label="按日期查看账单">
                        <span v-for="weekday in weekDays" :key="weekday" class="cy-weekday">{{ weekday }}</span>
                        <span v-for="blank in firstWeekday" :key="`blank-${blank}`" aria-hidden="true" />
                        <button v-for="day in days" :key="day.key" :disabled="loading" :aria-label="`${day.key}，收入${ledgerMoney(day.totals.income,false)}，支出${ledgerMoney(day.totals.expense,false)}`" :aria-pressed="selectedDay === day.key" :class="{ 'cy-today': day.key === currentDay }" @click="selectedDay = day.key">
                            <strong>{{ day.number }}</strong><small class="cy-expense">{{ loading || day.totals.expense === '0' ? '' : compact(day.totals.expense,-1) }}</small><small class="cy-income">{{ loading || day.totals.income === '0' ? '' : compact(day.totals.income,1) }}</small>
                        </button>
                    </div>
                </section>
                <p v-if="!loading && !totals.complete" class="cy-message">部分外币缺少汇率，收支概览仅显示已折算金额。原币账单完整保留。</p>
                <f7-link v-if="!isCalendar" class="cy-primary-action" href="/transaction/add"><f7-icon f7="doc_badge_plus" />添加一条新记账</f7-link>
                <p v-if="loading" class="cy-empty" role="status">正在加载账单…</p>
                <template v-else><LedgerDayList :entries="visibleEntries" /><section v-if="!visibleEntries.length" class="cy-panel cy-empty"><p>{{ isCalendar ? `${selectedDay} 还没有账单` : '这个月还没有账单' }}</p><p class="cy-muted">从一笔收入或支出开始记录。</p><f7-link :href="addLink">记一笔</f7-link></section></template>
                <p class="cy-muted cy-ledger-footnote">收支按人民币汇总；账户互转和投资结算不计入生活收支。</p>
            </template>
        </main>
        <template #fixed><f7-link v-if="isCalendar" class="cy-fab" :href="addLink" aria-label="添加记账"><f7-icon f7="plus" /></f7-link><LedgerNavigation :active="isCalendar ? 'calendar' : 'home'" /></template>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import { LedgerDecimal, ledgerMoney, ledgerTotals, useMobileLedger } from '@/lib/mobile-ledger.ts';
import { useUserStore } from '@/stores/user.ts';
const props = defineProps<{ f7route: Router.Route }>();
const isCalendar = computed(() => props.f7route.path === '/calendar');
const { entries, loading, error, load } = useMobileLedger();
const users = useUserStore();
const month = ref(moment().format('YYYY-MM'));
const currentDay = moment().format('YYYY-MM-DD');
const selectedDay = ref(currentDay);
const addLink = computed(() => isCalendar.value ? `/transaction/add?time=${moment(selectedDay.value).hour(12).unix()}&noTransactionDraft=true` : '/transaction/add');
const totals = computed(() => ledgerTotals(entries.value));
const visibleEntries = computed(() => isCalendar.value ? entries.value.filter(item => item.day === selectedDay.value) : entries.value);
const weekStart = computed(() => users.currentUserFirstDayOfWeek);
const weekDays = computed(() => Array.from({ length: 7 }, (_, index) => ['日','一','二','三','四','五','六'][(index + weekStart.value) % 7]));
const firstWeekday = computed(() => (moment(month.value,'YYYY-MM').day() - weekStart.value + 7) % 7);
const days = computed(() => Array.from({ length: moment(month.value,'YYYY-MM').daysInMonth() }, (_, index) => {
    const key = `${month.value}-${String(index + 1).padStart(2,'0')}`;
    return { key, number: index + 1, totals: ledgerTotals(entries.value.filter(item => item.day === key)) };
}));
function display(value: string): string { return loading.value ? '—' : ledgerMoney(value, false); }
function compact(value: string, direction: number): string { const amount = new LedgerDecimal(value).mul(direction); return `${amount.gt(0) ? '+' : ''}${amount.abs().gte(10000) ? `${amount.div(10000).toFixed(1)}万` : amount.toFixed(amount.isInteger() ? 0 : 2)}`; }
async function refresh(done?: () => void): Promise<void> {
    let date = moment(month.value,'YYYY-MM',true);
    if (!date.isValid()) { date = moment(); month.value = date.format('YYYY-MM'); selectedDay.value = date.format('YYYY-MM-DD'); }
    await load(date.clone().startOf('month').unix(), date.clone().endOf('month').unix());
    if (typeof done === 'function') done();
}
function monthChanged(): void { selectedDay.value = month.value === currentDay.slice(0,7) ? currentDay : `${month.value}-01`; void refresh(); }
function shift(direction: number): void { month.value = moment(month.value,'YYYY-MM').add(direction,'month').format('YYYY-MM'); monthChanged(); }
function today(): void { month.value = moment().format('YYYY-MM'); selectedDay.value = moment().format('YYYY-MM-DD'); void refresh(); }
</script>
<style scoped>
.cy-hero-stats small{font-size:11px;font-weight:400}.cy-calendar-panel{padding:16px 10px!important}.cy-calendar-totals{display:flex;flex-wrap:wrap;gap:12px;font-size:12px;margin:0 5px 24px}.cy-calendar-grid{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:8px 1px}.cy-weekday{text-align:center;color:var(--cy-muted);font-size:12px;padding-bottom:8px}.cy-calendar-grid button{background:transparent;border:0;border-radius:9px;min-height:66px;padding:5px 0;display:flex;flex-direction:column;align-items:center;min-width:0;gap:3px}.cy-calendar-grid button strong{font-size:16px;font-weight:500}.cy-calendar-grid button small{display:block;font-size:9px;line-height:1.25;min-height:12px;max-width:100%;overflow:hidden;white-space:nowrap}.cy-calendar-grid .cy-today strong{color:var(--cy-accent);font-weight:750}.cy-calendar-grid button[aria-pressed=true]{background:var(--cy-accent);color:white}.cy-calendar-grid button[aria-pressed=true] *{color:white}.cy-ledger-footnote{padding-bottom:6px}
</style>
