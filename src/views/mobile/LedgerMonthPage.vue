<template>
    <f7-page class="cy-main-page cy-mobile-surface" :class="{'cy-calendar-page': isCalendar}" ptr @ptr:refresh="refresh" @page:afterin="onPageAfterIn" @page:beforeout="active = false">
        <f7-navbar :class="{'cy-calendar-navbar': isCalendar}">
            <f7-nav-left v-if="isCalendar" class="cy-calendar-nav-left"><div class="cy-calendar-caption"><button aria-label="选择月份" @click="showMonth = true">{{ date(month).format('YYYY年M月') }}<small>⌄</small></button><div class="cy-calendar-totals"><span>收 <b class="cy-income">{{ display(totals.income) }}</b></span><span>支 <b class="cy-expense">{{ display(totals.expense) }}</b></span><span>余 {{ display(totals.balance) }}</span></div></div></f7-nav-left>
            <f7-nav-left v-else class="cy-home-nav-left">
                <button id="cy-home-book-menu" class="cy-book-trigger" aria-label="切换或添加账本" aria-haspopup="dialog" :aria-expanded="showBooks" @click="showBooks = true">
                    <f7-icon f7="book_closed_fill" /><span>{{ books.selectedBookIds.length ? books.scopeName : '日常账本' }}</span><f7-icon class="cy-book-chevron" f7="chevron_down" />
                </button>
            </f7-nav-left>
            <f7-nav-right v-if="isCalendar" class="cy-calendar-tools"><button aria-label="回到今天" @click="today">今</button><f7-link :href="addLink" aria-label="添加记账"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiNotePlusOutline" /></svg></f7-link><button id="cy-calendar-menu" aria-label="日历菜单" @click="showMenu = true"><f7-icon f7="ellipsis_vertical" /></button></f7-nav-right>
            <f7-nav-right v-else><f7-link href="/transaction/list" icon-f7="search" aria-label="搜索账单" /></f7-nav-right>
        </f7-navbar>
        <main class="cy-page-body" :class="{'cy-calendar-body': isCalendar}">
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh()">重试</button></p>
            <template v-if="!error">
                <section v-if="!isCalendar" class="cy-panel cy-hero" aria-label="月度收支概览">
                    <div class="cy-hero-stats"><div><p class="cy-muted">本月收入</p><strong>{{ display(totals.income) }}</strong></div><div><p class="cy-muted">本月结余</p><strong>{{ display(totals.balance) }}</strong></div><div><p class="cy-muted">本月账单</p><strong>{{ loading ? '—' : entries.length }} <small>笔</small></strong></div></div>
                    <p class="cy-muted">本月支出（元）</p><p class="cy-major">{{ display(totals.expense) }}</p>
                </section>
                <section v-else class="cy-calendar-panel">
                    <div class="cy-calendar-grid" role="group" aria-label="按日期查看账单" @touchstart.passive="touchStart" @touchend.passive="touchEnd">
                        <span v-for="weekday in weekDays" :key="weekday" class="cy-weekday">{{ weekday }}</span>
                        <span v-for="blank in firstWeekday" :key="`blank-${blank}`" aria-hidden="true" />
                        <button v-for="day in days" :key="day.key" :disabled="loading" :aria-label="dayLabel(day)" :aria-pressed="selectedDay === day.key" :class="{ 'cy-today': day.key === currentDay }" :style="heatStyle(day)" @click="selectedDay = day.key">
                            <strong>{{ day.number }}</strong>
                            <template v-if="day.hasFlows"><small class="cy-expense">{{ day.totals.complete ? compact(day.totals.expense,-1) : '待换算' }}</small><small class="cy-income">{{ day.totals.complete ? compact(day.totals.income,1) : '' }}</small></template>
                            <small v-else-if="preferences.calendarShowLunar" class="cy-lunar">{{ lunar(day.key) }}</small>
                            <span v-if="day.due.length" class="cy-due-dots"><i v-if="day.due.some(item => item.kind === 'repayment')" class="cy-repay-dot" /><i v-if="day.due.some(item => item.kind === 'deposit')" class="cy-deposit-dot" /></span>
                        </button>
                    </div>
                </section>
                <p v-if="isCalendar && dueError" class="cy-message" role="alert">{{ dueError }} <button @click="loadDue">重试</button></p>
                <header v-if="isCalendar" class="cy-selected-heading"><h2>{{ selectedDay === currentDay ? '今天' : date(selectedDay,'YYYY-MM-DD').format('M月D日') }} <small v-if="preferences.calendarShowLunar">{{ lunar(selectedDay) }}</small></h2><span><span>收 <b class="cy-income">{{ display(dayTotals.income,dayTotals.complete) }}</b></span><span>支 <b class="cy-expense">{{ display(dayTotals.expense,dayTotals.complete) }}</b></span><span>余 {{ display(dayTotals.balance,dayTotals.complete) }}</span></span></header>
                <section v-if="isCalendar && selectedDue.length" class="cy-due-card"><f7-link v-for="item in selectedDue" :key="item.id" :href="`/calendar/due?date=${item.date}&id=${item.id}`"><span><strong>{{ item.kind === 'repayment' ? '待还款' : '定存到期' }} · {{ item.accountName }}</strong><small>{{ item.note || (item.date < currentDay ? '已逾期 · 点击处理' : '点击查看或处理') }}</small></span><b>{{ ledgerMoney(item.amount,false) }} <small>{{ item.currency }}</small></b></f7-link></section>
                <p v-if="!loading && !totals.complete" class="cy-message" role="status">部分金额待换算</p>
                <f7-link v-if="!isCalendar" class="cy-primary-action" :href="addLink"><svg class="cy-add-icon" viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiNotePlusOutline" /></svg><span>添加一条新记账</span></f7-link>
                <p v-if="loading" class="cy-empty" role="status">正在加载账单…</p>
                <template v-else><LedgerDayList :entries="visibleEntries" :show-heading="!isCalendar" /><section v-if="!visibleEntries.length" class="cy-panel cy-empty"><p>{{ isCalendar ? `${selectedDay} 还没有账单` : '这个月还没有账单' }}</p><f7-link v-if="isCalendar" :href="addLink">记一笔</f7-link></section></template>
            </template>
        </main>
        <template #fixed><LedgerNavigation :active="isCalendar ? 'calendar' : 'home'" /></template>
        <f7-popover v-if="isCalendar" target-el="#cy-calendar-menu" v-model:opened="showMenu" class="cy-calendar-popover"><f7-list>
            <f7-list-item title="本月账单" :link="monthBillsLink" popover-close><template #media><f7-icon f7="calendar" /></template></f7-list-item>
            <f7-list-item title="多选账本" link="#" popover-close @click="openBookSelector"><template #media><f7-icon f7="checkmark_square" /></template></f7-list-item>
            <f7-list-item :title="preferences.calendarHeatmap ? '隐藏热力背景' : '显示热力背景'" link="#" popover-close @click="settings.setCalendarPreference('calendarHeatmap',!preferences.calendarHeatmap)"><template #media><f7-icon f7="square_stack_3d_up" /></template></f7-list-item>
            <f7-list-item title="更多设置" link="/calendar/settings" popover-close><template #media><f7-icon f7="ellipsis" /></template></f7-list-item>
        </f7-list></f7-popover>
        <f7-sheet v-if="isCalendar" v-model:opened="showMonth" class="cy-calendar-sheet" backdrop swipe-to-close><f7-toolbar><div class="left">选择月份</div><div class="right"><f7-link sheet-close>完成</f7-link></div></f7-toolbar><div class="cy-month-choice"><button class="cy-button" aria-label="上个月" @click="shift(-1)">‹</button><input v-model="month" type="month" min="1900-01" max="9999-12" aria-label="账单月份" @change="monthChanged" /><button class="cy-button" aria-label="下个月" @click="shift(1)">›</button></div></f7-sheet>
        <f7-sheet v-if="isCalendar" v-model:opened="showBookSelector" class="cy-calendar-sheet" backdrop swipe-to-close><f7-toolbar><div class="left">多选账本</div><div class="right"><f7-link @click="applyBooks">完成</f7-link></div></f7-toolbar><div class="cy-book-options"><label><input type="checkbox" :checked="!draftBooks.length" @change="draftBooks = []" />全部账本</label><label v-for="book in books.allBooks" :key="book.id"><input v-model="draftBooks" type="checkbox" :value="book.id" />{{ book.icon }} {{ book.name }}{{ book.archived ? ' · 已归档' : '' }}</label></div></f7-sheet>
        <f7-popover v-if="!isCalendar" target-el="#cy-home-book-menu" v-model:opened="showBooks" class="cy-home-books">
            <f7-list>
                <f7-list-item title="全部账本" link="#" popover-close :no-chevron="true" @click="selectBook('')"><template #after><f7-icon v-if="!books.selectedBookIds.length" f7="checkmark_alt" /></template></f7-list-item>
                <f7-list-item v-for="book in books.allBooks" :key="book.id" :title="`${book.icon} ${book.name}${book.archived ? ' · 已归档' : ''}`" link="#" popover-close :no-chevron="true" @click="selectBook(book.id)"><template #after><f7-icon v-if="books.selectedBookIds.includes(book.id)" f7="checkmark_alt" /></template></f7-list-item>
                <f7-list-item title="添加账本" link="/books" popover-close><template #media><f7-icon f7="plus" /></template></f7-list-item>
            </f7-list>
        </f7-popover>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { mdiNotePlusOutline } from '@mdi/js';
import { storeToRefs } from 'pinia';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import { LedgerDecimal, ledgerMoney, ledgerTotals, useMobileLedger } from '@/lib/mobile-ledger.ts';
import { useSettingsStore } from '@/stores/setting.ts';
import { calendarEvents, type CalendarEvent } from '@/lib/calendar-events.ts';
import { investmentError } from '@/lib/investments.ts';
import { getChineseYearMonthDayInfo, getChineseCalendarAlternateDisplayDate } from '@/lib/calendar/chinese_calendar.ts';
import { DEFAULT_CONTENT } from '@/locales/calendar/chinese/index.ts';
const props = defineProps<{ f7route: Router.Route }>();
const isCalendar = computed(() => props.f7route.path === '/calendar');
const { entries, displayEntries, loading, error, load } = useMobileLedger({ applyFilters: () => false });
const settings = useSettingsStore();
const preferences = computed(() => settings.appSettings);
const showMenu = ref(false), showMonth = ref(false), showBookSelector = ref(false), draftBooks = ref<string[]>([]);
const dueItems = ref<CalendarEvent[]>([]), dueError = ref('');
let dueGeneration = 0;
const visibleDue = computed(() => dueItems.value.filter(item => !item.completed && (!books.selectedBookIds.length || books.selectedBookIds.includes(item.bookId)) && (item.kind === 'repayment' ? preferences.value.calendarShowRepayments : preferences.value.calendarShowDeposits)));
const selectedDue = computed(() => visibleDue.value.filter(item => item.date === selectedDay.value));
const dayTotals = computed(() => ledgerTotals(entries.value.filter(item => item.day === selectedDay.value)));
const monthBillsLink = computed(() => `/transaction/list?dateType=255&minTime=${date(month.value).startOf('month').unix()}&maxTime=${date(month.value).endOf('month').unix()}`);
const scope = useLedgerScopeStore(), books = useBooksStore();
const { month, selectedDay } = storeToRefs(scope);
const { currentDay } = storeToRefs(scope);
const showBooks = ref(false), active = ref(false);
function date(value: string, format = 'YYYY-MM'): moment.Moment { return moment.tz(value,format,true,scope.timeZone); }
watch(() => books.selectedBookIds.join(','), () => { if (active.value) void refresh(); });
watch(() => [scope.timeZone, currentDay.value], () => { if (active.value) void refresh(); });
const addLink = computed(() => isCalendar.value ? `/transaction/add?time=${date(selectedDay.value,'YYYY-MM-DD').hour(12).unix()}&noTransactionDraft=true` : '/transaction/add');
const totals = computed(() => ledgerTotals(entries.value));
const visibleEntries = computed(() => isCalendar.value ? displayEntries.value.filter(item => item.day === selectedDay.value) : displayEntries.value);
const weekStart = computed(() => preferences.value.calendarWeekStart);
const weekDays = computed(() => Array.from({ length: 7 }, (_, index) => ['日','一','二','三','四','五','六'][(index + weekStart.value) % 7]));
const firstWeekday = computed(() => (date(month.value).day() - weekStart.value + 7) % 7);
const days = computed(() => Array.from({ length: date(month.value).daysInMonth() }, (_, index) => {
    const key = `${month.value}-${String(index + 1).padStart(2,'0')}`;
    const items = entries.value.filter(item => item.day === key);
    return { key, number: index + 1, hasFlows: items.some(item => item.type === 2 || item.type === 3), totals: ledgerTotals(items), due: visibleDue.value.filter(item => item.date === key) };
}));
function display(value: string, complete = totals.value.complete): string { return loading.value || (isCalendar.value && !complete) ? '—' : ledgerMoney(value, false); }
function compact(value: string, direction: number): string { const amount = new LedgerDecimal(value).mul(direction); return `${direction > 0 ? '+' : amount.isZero() ? '-' : ''}${amount.abs().gte(10000) ? `${amount.div(10000).toFixed(1)}万` : amount.toFixed(amount.isInteger() ? 0 : 2)}`; }
async function refresh(done?: () => void): Promise<void> {
    let requestedDate = date(isCalendar.value ? month.value : currentDay.value.slice(0,7));
    if (!requestedDate.isValid()) { requestedDate = moment().tz(scope.timeZone); month.value = requestedDate.format('YYYY-MM'); selectedDay.value = requestedDate.format('YYYY-MM-DD'); }
    await Promise.all([load(requestedDate.clone().startOf('month').unix(), requestedDate.clone().endOf('month').unix()), ...(isCalendar.value ? [loadDue()] : [])]);
    if (typeof done === 'function') done();
}
function selectBook(id: string): void { books.selectedBookIds = id ? [id] : []; }
function onPageAfterIn(): void { scope.syncClock(); active.value = true; void refresh(); }
function monthChanged(): void { if (!date(month.value).isValid()) return; selectedDay.value = month.value === currentDay.value.slice(0,7) ? currentDay.value : `${month.value}-01`; void refresh(); }
function shift(direction: number): void { const next = date(month.value).add(direction,'month'); if (next.year() < 1900 || next.year() > 9999) return; month.value = next.format('YYYY-MM'); monthChanged(); }
function today(): void { month.value = moment().tz(scope.timeZone).format('YYYY-MM'); selectedDay.value = moment().tz(scope.timeZone).format('YYYY-MM-DD'); void refresh(); }
function lunar(key: string): string {
    const [year,month,day] = key.split('-').map(Number);
    const info = getChineseYearMonthDayInfo({ year: year!, month: month!, day: day! }, DEFAULT_CONTENT);
    if (!info) return '';
    const holidays: Record<string,string> = { '1-1': '春节', '1-15': '元宵', '5-5': '端午', '7-7': '七夕', '8-15': '中秋', '9-9': '重阳', '12-8': '腊八' };
    return (!info.isLeapMonth && holidays[`${info.month}-${info.day}`]) || getChineseCalendarAlternateDisplayDate(info).displayDate;
}
type CalendarDay = typeof days.value[number];
function dayLabel(day: CalendarDay): string { return `${day.key}，${day.totals.complete ? `收入${ledgerMoney(day.totals.income,false)}，支出${ledgerMoney(day.totals.expense,false)}` : '部分金额待换算'}${day.due.length ? `，${day.due.length}项待处理` : ''}`; }
const maxDailyAmount = computed(() => days.value.reduce((max,day) => LedgerDecimal.max(max,new LedgerDecimal(day.totals.income).abs().plus(new LedgerDecimal(day.totals.expense).abs())),new LedgerDecimal(1)));
function heatStyle(day: CalendarDay): Record<string,string> {
    if (!preferences.value.calendarHeatmap || !day.hasFlows || !day.totals.complete || day.key === selectedDay.value) return {};
    const ratio = new LedgerDecimal(day.totals.income).abs().plus(new LedgerDecimal(day.totals.expense).abs()).div(maxDailyAmount.value);
    return { backgroundColor: `color-mix(in srgb, var(--cy-accent) ${ratio.mul(28).plus(4).toFixed(1)}%, transparent)` };
}
function openBookSelector(): void { draftBooks.value = [...books.selectedBookIds]; showBookSelector.value = true; }
function applyBooks(): void { books.selectedBookIds = [...draftBooks.value]; showBookSelector.value = false; }
async function loadDue(): Promise<void> {
    const version = ++dueGeneration; dueItems.value = []; dueError.value = '';
    try { const result = await calendarEvents.list(month.value); if (version === dueGeneration) dueItems.value = result; }
    catch(cause) { if (version === dueGeneration) dueError.value = investmentError(cause); }
}
let touchX = 0, touchY = 0;
function touchStart(event: TouchEvent): void { touchX = event.changedTouches[0]?.clientX || 0; touchY = event.changedTouches[0]?.clientY || 0; }
function touchEnd(event: TouchEvent): void { const touch = event.changedTouches[0]; if (touch && Math.abs(touch.clientX-touchX)>70 && Math.abs(touch.clientY-touchY)<45) shift(touch.clientX>touchX ? -1 : 1); }
</script>
<style scoped>
.cy-home-nav-left{min-width:0;max-width:calc(100% - 60px)}
.cy-book-trigger{display:flex;align-items:center;gap:10px;min-width:0;max-width:100%;min-height:44px;padding:0 4px;background:transparent;border:0;color:var(--cy-ink);font:inherit;font-size:21px;font-weight:650;text-align:left;cursor:pointer}
.cy-book-trigger span{overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.cy-book-trigger .icon{flex:none;font-size:23px}.cy-book-trigger .cy-book-chevron{font-size:12px;color:var(--cy-muted)}
.cy-primary-action{box-sizing:border-box;height:auto;min-height:56px;padding:14px 18px;line-height:1.5;white-space:normal;text-align:center}
.cy-primary-action .cy-add-icon{display:block;flex:0 0 26px;width:26px;height:26px;fill:currentColor}
.cy-primary-action span{position:static;flex:0 1 auto;min-width:0;margin:0;line-height:1.5}
.cy-hero-stats small{font-size:11px;font-weight:400}
.cy-calendar-page,.cy-calendar-navbar{--f7-navbar-height:82px}.cy-calendar-nav-left{flex:1;min-width:0}.cy-calendar-caption{min-width:0;width:100%}.cy-calendar-caption>button{background:transparent;border:0;padding:0;color:var(--cy-ink);font:inherit;font-size:23px;font-weight:600;line-height:1.5}.cy-calendar-caption>button small{font-size:12px;margin-left:7px;color:var(--cy-muted)}.cy-calendar-totals{display:flex;gap:9px;font-size:11px;line-height:1.7;color:var(--cy-muted);flex-wrap:wrap}.cy-calendar-totals b,.cy-selected-heading b{font-weight:500}.cy-calendar-tools{gap:8px;margin-left:4px!important}.cy-calendar-tools button{display:flex;align-items:center;justify-content:center;border:0;background:transparent;color:var(--cy-ink);font:inherit;font-size:23px;min-width:30px;height:44px;padding:0}.cy-calendar-tools .link{padding:0;width:32px;min-width:32px}.cy-calendar-tools svg{width:25px;height:25px;fill:currentColor}.cy-calendar-tools .icon{font-size:23px}
.cy-calendar-body{padding:8px 12px 110px}.cy-calendar-panel{margin-bottom:20px}.cy-calendar-grid{display:grid;grid-template-columns:repeat(7,minmax(0,1fr));gap:4px 2px;touch-action:pan-y}.cy-weekday{text-align:center;color:var(--cy-muted);font-size:14px;padding:11px 0 15px}.cy-calendar-grid button{position:relative;background:transparent;color:var(--cy-ink);border:0;border-radius:7px;height:73px;padding:6px 0 9px;display:flex;flex-direction:column;align-items:center;min-width:0;gap:1px}.cy-calendar-grid button strong{font-size:20px;font-weight:550;line-height:25px}.cy-calendar-grid button small{display:block;font-size:10px;line-height:14px;max-width:100%;overflow:hidden;white-space:nowrap}.cy-calendar-grid button .cy-lunar{margin-top:6px;color:var(--cy-muted);font-size:11px}.cy-calendar-grid .cy-today strong{color:var(--cy-accent);font-weight:750}.cy-calendar-grid button[aria-pressed=true]{background:var(--cy-accent);color:white}.cy-calendar-grid button[aria-pressed=true] *{color:white!important}.cy-due-dots{display:flex;gap:3px;position:absolute;bottom:3px}.cy-due-dots i{width:4px;height:4px;border-radius:50%}.cy-repay-dot{background:var(--cy-expense)}.cy-deposit-dot{background:var(--cy-accent)}.cy-calendar-grid button[aria-pressed=true] i{background:white}
.cy-selected-heading{display:flex;align-items:center;justify-content:space-between;gap:10px;margin:4px 0 10px;flex-wrap:wrap}.cy-selected-heading h2{font-size:14px;font-weight:500;margin:0}.cy-selected-heading h2 small{font-size:12px;color:var(--cy-muted);margin-left:5px}.cy-selected-heading>span{display:flex;gap:9px;font-size:11px;flex-wrap:wrap}.cy-due-card{border:1px solid var(--cy-line);border-radius:14px;background:var(--cy-card);padding:0 13px;margin-bottom:12px}.cy-due-card>a{display:flex;align-items:center;gap:12px;padding:14px 0;border-bottom:1px solid var(--cy-line);color:var(--cy-ink)}.cy-due-card>a:last-child{border:0}.cy-due-card>a>span{flex:1;min-width:0}.cy-due-card strong{font-size:13px;font-weight:500;overflow-wrap:anywhere}.cy-due-card small{display:block;font-size:11px;color:var(--cy-muted);margin-top:4px}.cy-due-card b{font-size:15px;font-weight:500;text-align:right}
.cy-calendar-sheet{height:auto;max-height:75vh;background:var(--cy-card);color:var(--cy-ink)}.cy-calendar-sheet :deep(.sheet-modal-inner){padding-top:var(--f7-toolbar-height);max-height:70vh;overflow:auto}.cy-month-choice{display:flex;align-items:center;justify-content:space-between;gap:15px;padding:30px 20px 50px}.cy-month-choice button{width:42px!important;flex:0 0 42px;height:42px;border:1px solid var(--cy-line);border-radius:9px;background:var(--cy-card);color:var(--cy-ink);font-size:24px;padding:0}.cy-month-choice input{flex:1;width:160px;text-align:center;font:inherit;border:0;background:transparent;color:inherit;min-width:0}.cy-book-options{padding:14px 20px 30px}.cy-book-options label{display:flex;align-items:center;gap:12px;padding:13px 0;font-size:16px}.cy-book-options input{accent-color:var(--cy-accent);width:18px;height:18px}
@media(min-height:850px){.cy-calendar-grid button{height:80px}}
.cy-calendar-navbar :deep(.left),.cy-calendar-navbar :deep(.right){box-shadow:none!important;backdrop-filter:none!important;background:transparent!important;border-radius:0!important}.cy-calendar-navbar :deep(.navbar-inner){padding-inline:14px}
</style>
