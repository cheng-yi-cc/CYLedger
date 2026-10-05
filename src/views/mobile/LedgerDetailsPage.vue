<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="refresh">
        <f7-navbar :title="title" back-link="统计" />
        <main class="cy-page-body">
            <p class="cy-muted">{{ books.scopeName }} · {{ rangeLabel }}</p>
            <section class="cy-panel cy-three-stats"><div>收入<strong>{{ loading || error ? '—' : ledgerMoney(totals.income) }}</strong></div><div>支出<strong>{{ loading || error ? '—' : ledgerMoney(totals.expense) }}</strong></div><div>结余<strong>{{ loading || error ? '—' : ledgerMoney(totals.balance) }}</strong></div></section>
            <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh">重试</button></p>
            <p v-else-if="loading" class="cy-empty">正在加载流水…</p>
            <template v-else><p v-if="!totals.complete" class="cy-message">部分原币金额缺少历史折算值，合计暂不包含这些金额。</p><LedgerDayList :entries="filtered" /><p v-if="!filtered.length" class="cy-empty">所选条件下没有流水。</p></template>
        </main>
    </f7-page>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import { useMobileLedger, ledgerMoney, ledgerTotals } from '@/lib/mobile-ledger.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
const props = defineProps<{ f7route: Router.Route }>();
const books = useBooksStore(), scope = useLedgerScopeStore();
const { entries, loading, error, load } = useMobileLedger();
const query = computed(() => props.f7route.query);
const title = computed(() => query.value['title'] || '统计流水');
const rangeLabel = computed(() => `${moment.unix(Number(query.value['start'])).tz(scope.timeZone).format('YYYY-MM-DD')} 至 ${moment.unix(Number(query.value['end'])).tz(scope.timeZone).format('YYYY-MM-DD')}`);
const filtered = computed(() => entries.value.filter(item => (!query.value['type'] || item.type === Number(query.value['type'])) && (!query.value['categoryId'] || item.categoryId === query.value['categoryId'] || (query.value['primary'] === 'true' && item.primaryCategoryId === query.value['categoryId'])) && (!query.value['tagId'] || item.tagIds.includes(query.value['tagId']))));
const totals = computed(() => ledgerTotals(filtered.value));
async function refresh(): Promise<void> { const start=Number(query.value['start']), end=Number(query.value['end']); if (!Number.isSafeInteger(start) || !Number.isSafeInteger(end) || start <= 0 || end < start) { error.value='日期范围无效，请返回统计重新选择。'; loading.value=false; return; } await load(start,end); }
</script>
