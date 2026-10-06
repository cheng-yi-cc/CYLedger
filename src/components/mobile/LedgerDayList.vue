<template>
    <section v-for="group in groups" :key="group.day" class="cy-day-group" :class="{ 'cy-list-simple': prefs.view === 'simple' }">
        <header v-if="showHeading" class="cy-day-heading"><h2>{{ dayLabel(group.day) }}</h2><span><span>收 <b class="cy-income">{{ ledgerMoney(group.totals.income, false) }}</b></span><span>支 <b class="cy-expense">{{ ledgerMoney(group.totals.expense, false) }}</b></span><small v-if="!group.totals.complete">部分金额</small></span></header>
        <div class="cy-day-card">
            <f7-link v-for="item in group.items" :key="item.id" class="cy-entry" :class="{ 'cy-entry-selected': selectedIds.includes(item.id) }" :href="selectionMode ? '#' : entryLink(item)" @click="onClick($event, item)" @taphold="emit('hold', item)">
                <span v-if="selectionMode" class="cy-select-bill" :aria-label="selectedIds.includes(item.id) ? '已选择' : '未选择'"><f7-icon :f7="selectedIds.includes(item.id) ? 'checkmark_circle_fill' : 'circle'" /></span>
                <span v-else class="cy-entry-icon" aria-hidden="true" :style="!prefs.lineIcons && item.color ? { color: '#'+item.color } : undefined"><ItemIcon v-if="item.icon && item.type !== 4 && !item.investment" :icon-type="getCategoryIconType(item.iconType)" :icon-id="item.icon" /><f7-icon v-else :f7="item.investment ? 'chart_bar' : item.type === 4 ? transferIcon(item) : item.type === 1 ? prefs.specialIcons['adjustment'] : item.type === 2 ? 'money_dollar_circle' : 'cart'" /></span>
                <span class="cy-entry-copy"><strong>{{ title(item) }}</strong><span v-if="prefs.view === 'detail'" class="cy-entry-subline"><small v-for="tag in item.tags.slice(0, 2)" :key="tag" class="cy-entry-tag">{{ tag }}</small><small v-if="subtitle(item)" class="cy-entry-note">{{ subtitle(item) }}</small><f7-icon v-if="item.pictures?.length" class="cy-entry-picture" f7="photo" /></span><small v-if="prefs.showTime" class="cy-entry-time">{{ moment.unix(item.time).tz(scope.timeZone).format('HH:mm') }}</small></span>
                <span class="cy-entry-value"><strong :class="item.type === 2 ? 'cy-income' : item.type === 3 ? 'cy-expense' : ''">{{ item.hidden ? '••••' : ledgerSignedAmount(item.amount, item.transferDirection ? item.transferDirection === 'in' ? 2 : 3 : item.type) }}<small v-if="item.currency !== 'CNY'">{{ item.currency }}</small></strong><small v-if="prefs.showOriginal && !item.hidden && hasDiscount(item)" class="cy-original-amount">{{ ledgerMoney(new LedgerDecimal(item.amount).plus(item.discountAmount || '0').toString(), false) }}</small><small v-if="prefs.view === 'detail'" class="cy-entry-account"><ItemIcon v-if="prefs.showAccountIcon && item.accountIcon" :icon-type="getAccountIconType(item.accountIconType)" :icon-id="item.accountIcon" />{{ accountView ? item.bookName : item.account }}</small></span>
            </f7-link>
        </div>
    </section>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import moment from 'moment-timezone';
import { ledgerMoney, ledgerTotals, ledgerSignedAmount, LedgerDecimal, type LedgerEntry } from '@/lib/mobile-ledger.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { useLedgerExperienceStore } from '@/stores/ledgerExperience.ts';
import { getCategoryIconType, getAccountIconType } from '@/lib/icon.ts';
import ItemIcon from './ItemIcon.vue';
import {useAccountsStore} from '@/stores/account.ts';
import {inferDebtOperation} from '@/lib/ledger-debt.ts';
const props = withDefaults(defineProps<{ entries: LedgerEntry[]; showHeading?: boolean; accountView?: boolean; partialLastDay?: boolean; selectionMode?: boolean; selectedIds?: string[] }>(), { showHeading: true, accountView: false, partialLastDay: false, selectionMode: false, selectedIds: () => [] });
const emit = defineEmits<{ hold: [item: LedgerEntry]; select: [item: LedgerEntry] }>();
const scope = useLedgerScopeStore(), experience = useLedgerExperienceStore();
const accounts=useAccountsStore();
function transferIcon(item:LedgerEntry){const operation=inferDebtOperation(accounts.allAccountsMap[item.accountId],accounts.allAccountsMap[item.destinationAccountId]);return prefs.value.specialIcons[operation==='repay'?'repayment':operation==='collect'?'lend':operation||'transfer'];}
const prefs = computed(() => experience.preferences);
const groups = computed(() => {
    const days = new Map<string, LedgerEntry[]>();
    for (const item of props.entries) { if (!days.has(item.day)) days.set(item.day, []); days.get(item.day)!.push(item); }
    return [...days].sort(([a], [b]) => b.localeCompare(a)).map(([day, items], index) => { const totals = ledgerTotals(props.accountView ? items.map(item => ({ ...item, cny: item.amount })) : items); if (props.partialLastDay && index === days.size - 1) totals.complete = false; return { day, items, totals }; });
});
function category(item: LedgerEntry): string { if(item.type===1)return '余额调整'; return !prefs.value.simpleCategory && item.primaryCategory && item.primaryCategory !== item.title ? `${item.primaryCategory}-${item.title}` : item.title; }
function title(item: LedgerEntry): string { return prefs.value.remarkFirst && item.comment ? item.comment : category(item); }
function subtitle(item: LedgerEntry): string { return prefs.value.remarkFirst && item.comment ? category(item) : item.comment; }
function hasDiscount(item: LedgerEntry): boolean { return !!item.discountAmount && new LedgerDecimal(item.discountAmount).gt(0); }
function entryLink(item: LedgerEntry): string { if(item.wallet)return `/crypto/entry?eventId=${encodeURIComponent(item.investmentEventId!)}`; if(item.transferFeeParentId && item.transferFeeParentId!=='0')return `/transaction/detail?id=${item.transferFeeParentId}&type=4`; return item.investmentEventId ? `/investments/record?action=revise&eventId=${encodeURIComponent(item.investmentEventId)}` : `/transaction/detail?id=${item.id}&type=${item.type}`; }
function onClick(event: MouseEvent, item: LedgerEntry) { if (props.selectionMode) { event.preventDefault(); event.stopPropagation(); emit('select', item); } }
function dayLabel(day: string): string {
    const date = moment(day, 'YYYY-MM-DD'), today = moment(scope.currentDay, 'YYYY-MM-DD');
    const relative = day === scope.currentDay ? '今天 ' : day === today.subtract(1, 'day').format('YYYY-MM-DD') ? '昨天 ' : '';
    return `${date.format(date.year()===moment(scope.currentDay).year()?'M月D日':'YYYY年M月D日')}  ${relative}${['周日','周一','周二','周三','周四','周五','周六'][date.day()]}`;
}
</script>
<style scoped>
.cy-day-group{margin-bottom:17px}.cy-day-heading{display:flex;justify-content:space-between;align-items:baseline;gap:8px;margin-bottom:7px}.cy-day-heading h2{font-size:13px;white-space:nowrap;font-weight:500}.cy-day-heading>span{display:flex;gap:12px;flex-wrap:wrap;justify-content:flex-end;font-size:12px}.cy-day-heading b{font-weight:500}.cy-day-heading small{color:var(--cy-muted)}.cy-day-card{border-radius:11px;background:var(--cy-card);padding:2px 10px;overflow:hidden}.cy-entry{display:flex;width:100%;gap:10px;padding:5px 0;color:var(--cy-ink);align-items:center;white-space:normal;box-sizing:border-box;min-height:55px}.cy-entry-icon{color:var(--cy-icon,#798582);flex:0 0 25px;display:grid;place-items:center}.cy-entry-icon :deep(.icon){font-size:24px;color:inherit!important;--ebk-icon-font-size:26px}.cy-entry-copy{flex:1;min-width:0;align-self:center}.cy-entry-copy strong{display:block;font-weight:400;font-size:15px;line-height:1.5;white-space:nowrap;overflow:hidden;text-overflow:ellipsis}.cy-entry-subline{display:flex;align-items:center;gap:5px;margin-top:2px;min-width:0;overflow:hidden}.cy-entry-tag{display:block;max-width:118px;min-width:0;flex:0 1 auto;overflow:hidden;text-overflow:ellipsis;white-space:nowrap;background:var(--cy-soft);color:var(--cy-muted);border-radius:10px;padding:1px 5px;font-size:10px;line-height:1.5}.cy-entry-note{min-width:0;flex:1;white-space:nowrap;overflow:hidden;text-overflow:ellipsis;color:var(--cy-muted);font-size:11px;line-height:1.7}.cy-entry-time{display:block;font-size:10px;color:var(--cy-muted);margin-top:2px}.cy-entry-picture{font-size:12px;color:var(--cy-muted);flex:none}.cy-entry-value{text-align:right;max-width:43%;min-width:52px;flex-shrink:0}.cy-entry-value strong{font-size:16px;font-weight:500;overflow-wrap:anywhere;line-height:1.5}.cy-entry-value strong small{display:inline;margin-left:3px;font-size:10px}.cy-entry-value small{display:block;font-size:10px;color:var(--cy-muted);font-weight:400;line-height:1.7;margin-top:2px;overflow-wrap:anywhere}.cy-entry-account :deep(.icon){font-size:11px;--ebk-icon-font-size:12px;margin-right:4px;vertical-align:middle}.cy-original-amount{text-decoration:line-through}.cy-list-simple .cy-entry{min-height:44px;padding-block:8px}.cy-entry-selected{background:var(--cy-soft)}.cy-select-bill{color:var(--cy-accent);width:25px}.cy-select-bill .icon{font-size:22px}
</style>
