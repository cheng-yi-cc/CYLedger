<template>
    <section v-for="group in groups" :key="group.day" class="cy-day-group">
        <header v-if="showHeading" class="cy-day-heading"><h2>{{ dayLabel(group.day) }}</h2><span><span class="cy-income">收 {{ ledgerMoney(group.totals.income, false) }}</span><span class="cy-expense">支 {{ ledgerMoney(group.totals.expense, false) }}</span><small v-if="!group.totals.complete">部分金额</small></span></header>
        <div class="cy-day-card">
            <f7-link v-for="item in group.items" :key="item.id" class="cy-entry" :href="item.investmentEventId ? `/investments/record?action=revise&eventId=${encodeURIComponent(item.investmentEventId)}` : `/transaction/detail?id=${item.id}&type=${item.type}`">
                <span class="cy-entry-icon" aria-hidden="true"><ItemIcon v-if="accountView && item.icon" :icon-type="getCategoryIconType(item.iconType || 0)" :icon-id="item.icon" /><f7-icon v-else :f7="item.type === 2 ? 'arrow_down_left' : item.type === 3 ? 'arrow_up_right' : 'arrow_right_arrow_left'" /></span>
                <span class="cy-entry-copy"><strong>{{ item.title }}</strong><span v-if="item.tags.length" class="cy-entry-tags"><small v-for="tag in item.tags" :key="tag">{{ tag }}</small></span><small v-if="item.comment" class="cy-entry-note">{{ item.comment }}</small></span>
                <span class="cy-entry-value"><strong :class="item.type === 2 ? 'cy-income' : item.type === 3 ? 'cy-expense' : ''">{{ item.hidden ? '••••' : ledgerSignedAmount(item.amount,item.transferDirection ? item.transferDirection==='in' ? 2 : 3 : item.type) }}<small v-if="item.currency !== 'CNY'">{{ item.currency }}</small></strong><small>{{ accountView ? item.bookName : item.account }}</small></span>
            </f7-link>
        </div>
    </section>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import moment from 'moment-timezone';
import { ledgerMoney, ledgerTotals, ledgerSignedAmount, type LedgerEntry } from '@/lib/mobile-ledger.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { getCategoryIconType } from '@/lib/icon.ts';
const props = withDefaults(defineProps<{ entries: LedgerEntry[]; showHeading?: boolean; accountView?:boolean; partialLastDay?:boolean }>(), { showHeading: true, accountView:false, partialLastDay:false });
const scope = useLedgerScopeStore();
const groups = computed(() => {
    const days = new Map<string, LedgerEntry[]>();
    for (const item of props.entries) { if (!days.has(item.day)) days.set(item.day, []); days.get(item.day)!.push(item); }
    return [...days].sort(([a], [b]) => b.localeCompare(a)).map(([day, items],index) => {const totals=ledgerTotals(props.accountView ? items.map(item=>({...item,cny:item.amount})) : items);if(props.partialLastDay && index===days.size-1)totals.complete=false;return {day,items,totals};});
});
function dayLabel(day: string): string {
    const date = moment(day, 'YYYY-MM-DD');
    const today = moment(scope.currentDay, 'YYYY-MM-DD');
    const relative = day === scope.currentDay ? '今天 ' : day === today.subtract(1, 'day').format('YYYY-MM-DD') ? '昨天 ' : '';
    const weekday = ['周日','周一','周二','周三','周四','周五','周六'][date.day()];
    return `${date.format('M月D日')} ${relative}${weekday}`;
}
</script>
<style scoped>
.cy-day-group{margin-bottom:24px}.cy-day-heading{display:flex;justify-content:space-between;align-items:baseline;gap:8px;margin-bottom:10px}.cy-day-heading h2{font-size:14px;white-space:nowrap}.cy-day-heading>span{display:flex;gap:9px;flex-wrap:wrap;justify-content:flex-end;font-size:11px}.cy-day-heading small{color:var(--cy-muted)}.cy-day-card{border-radius:16px;background:var(--cy-card);padding:2px 15px;border:1px solid var(--cy-line)}.cy-entry{display:flex;width:100%;gap:11px;padding:18px 0;border-bottom:1px solid var(--cy-line);color:var(--cy-ink);align-items:flex-start;white-space:normal}.cy-entry:last-child{border:0}.cy-entry-icon{color:var(--cy-accent);padding-top:2px;flex-shrink:0}.cy-entry-icon .f7-icons{font-size:24px}.cy-entry-copy{flex:1;min-width:0}.cy-entry-copy strong{font-weight:500;font-size:15px;overflow-wrap:anywhere}.cy-entry-tags{display:flex;gap:4px;flex-wrap:wrap;margin-top:5px}.cy-entry-tags small{background:var(--cy-soft);color:var(--cy-accent);border-radius:6px;padding:2px 5px;font-size:10px}.cy-entry-note{display:block;color:var(--cy-muted);font-size:11px;line-height:1.5;margin-top:5px;overflow-wrap:anywhere}.cy-entry-value{text-align:right;max-width:43%;flex-shrink:0}.cy-entry-value strong{font-size:17px;font-weight:600;overflow-wrap:anywhere}.cy-entry-value small{display:block;font-size:11px;color:var(--cy-muted);font-weight:400;line-height:1.5;margin-top:5px;overflow-wrap:anywhere}
</style>
