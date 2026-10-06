<template>
    <f7-popup class="cy-mobile-surface cy-search-filter" :opened="opened" @popup:closed="emit('update:opened', false)">
        <div class="cy-filter-scroll">
            <section><h2>日期 <button @click="draft.start = ''; draft.end = ''">清除</button></h2><div class="cy-filter-pair"><input v-model="draft.start" type="date" aria-label="起始日期" /><span>—</span><input v-model="draft.end" type="date" aria-label="截止日期" /></div><div class="cy-filter-presets"><button v-for="preset in ['今天','本周','本月','今年']" :key="preset" @click="setPeriod(preset)">{{ preset }}</button></div></section>
            <section><h2>金额 <button @click="draft.minimum = ''; draft.maximum = ''">清除</button></h2><div class="cy-filter-pair"><input v-model="draft.minimum" inputmode="decimal" maxlength="16" placeholder="最低金额" aria-label="最低金额" /><span>—</span><input v-model="draft.maximum" inputmode="decimal" maxlength="16" placeholder="最高金额" aria-label="最高金额" /></div></section>
            <section><h2>备注</h2><input v-model="draft.remark" maxlength="255" placeholder="多个关键词可用空格分隔" aria-label="备注关键词" /></section>
            <section><h2>地点</h2><input v-model="draft.location" maxlength="128" placeholder="输入地点或坐标关键词" aria-label="地点关键词" /></section>
            <section v-for="group in groups" :key="group.key"><h2>{{ group.title }} <button @click="expanded = expanded === group.key ? '' : group.key"><f7-icon f7="plus" size="12" /> 添加{{ group.title }}</button></h2><div class="cy-filter-chips"><button v-for="id in draft[group.key]" :key="id" class="active" @click="toggle(group.key, id)">{{ group.items.find(item => item.id === id)?.name || '已删除' }} ×</button></div><div v-if="expanded === group.key" class="cy-filter-options"><label v-for="item in group.items" :key="item.id"><input type="checkbox" :checked="draft[group.key].includes(item.id)" @change="toggle(group.key,item.id)" />{{ item.name }}</label><p v-if="!group.items.length">还没有可选的{{ group.title }}</p></div></section>
            <section><h2>其他 <span class="cy-filter-match"><button :class="{active:!draft.matchAll}" @click="draft.matchAll = false">满足任一</button><button :class="{active:draft.matchAll}" @click="draft.matchAll = true">同时满足</button></span></h2><div class="cy-filter-chips"><button v-for="(name,id) in ledgerSearchFlags" :key="id" :class="{active:draft.flags.includes(id)}" @click="toggle('flags',id)">{{ name }}</button></div></section>
        </div>
        <footer><p v-if="error" role="alert">{{ error }}</p><div><button @click="draft = emptyLedgerSearchFilters()">重置</button><button class="cy-filter-confirm" @click="confirm">确认</button></div></footer>
    </f7-popup>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import moment from 'moment-timezone';
import { useBooksStore } from '@/stores/books.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { emptyLedgerSearchFilters, ledgerSearchFlags, validateLedgerSearch, type LedgerSearchFilters } from '@/lib/ledger-search.ts';
const props = defineProps<{ opened: boolean; modelValue: LedgerSearchFilters }>();
const emit = defineEmits<{ 'update:opened': [value: boolean]; 'update:modelValue': [value: LedgerSearchFilters] }>();
const draft = ref<LedgerSearchFilters>(emptyLedgerSearchFilters()), error = ref(''), expanded = ref('');
const books = useBooksStore(), accounts = useAccountsStore(), categories = useTransactionCategoriesStore(), tags = useTransactionTagsStore(), scope = useLedgerScopeStore();
type SelectionKey = 'bookIds' | 'accountIds' | 'reimbursementIds' | 'categoryIds' | 'tagIds';
const groups = computed<{ key: SelectionKey; title: string; items: { id: string; name: string }[] }[]>(() => [
    { key: 'bookIds', title: '账本', items: books.allBooks },
    { key: 'accountIds', title: '账户', items: Object.values(accounts.allAccountsMap).filter(item => !item.subAccounts?.length) },
    { key: 'reimbursementIds', title: '报销', items: Object.values(accounts.allAccountsMap).filter(item => item.assetProfile.kind === 'reimbursement') },
    { key: 'categoryIds', title: '分类', items: Object.values(categories.allTransactionCategoriesMap).map(item => ({ id: item.id, name: item.parentId && item.parentId !== '0' ? `${categories.allTransactionCategoriesMap[item.parentId]?.name || ''} / ${item.name}` : item.name })) },
    { key: 'tagIds', title: '标签', items: Object.values(tags.allTransactionTagsMap) }
]);
watch(() => props.opened, value => { if (value) { draft.value = JSON.parse(JSON.stringify(props.modelValue)); error.value = ''; expanded.value = ''; } });
function toggle(key: SelectionKey | 'flags', id: string) { draft.value[key] = draft.value[key].includes(id) ? draft.value[key].filter(value => value !== id) : [...draft.value[key], id]; }
function setPeriod(preset: string) { const now = moment().tz(scope.timeZone), period = preset === '今天' ? 'day' : preset === '本周' ? 'isoWeek' : preset === '本月' ? 'month' : 'year'; draft.value.start = now.clone().startOf(period).format('YYYY-MM-DD'); draft.value.end = now.clone().endOf(period).format('YYYY-MM-DD'); }
function confirm() { error.value = validateLedgerSearch(draft.value); if (error.value) return; emit('update:modelValue', JSON.parse(JSON.stringify(draft.value))); emit('update:opened', false); }
</script>
<style scoped>
.cy-search-filter{left:auto!important;right:0!important;top:0!important;margin:0!important;width:min(86vw,430px)!important;height:100%!important;border-radius:0!important;background:var(--cy-bg);display:flex;flex-direction:column}.cy-filter-scroll{overflow:auto;flex:1;padding:12px 15px 20px}.cy-filter-scroll section{margin-bottom:20px}.cy-filter-scroll h2{display:flex;align-items:center;gap:12px;font-size:15px;font-weight:500;min-height:34px;margin:0 0 9px}.cy-filter-scroll h2>button{color:var(--cy-accent);font-size:12px;background:transparent;border:0;padding:5px}.cy-filter-pair{display:flex;align-items:center;gap:8px}.cy-filter-scroll input:not([type=checkbox]){min-width:0;width:100%;padding:10px 8px;border:0;border-radius:4px;background:var(--cy-card);font:inherit;font-size:12px;color:var(--cy-ink);box-sizing:border-box}.cy-filter-pair input{flex:1;width:40%!important}.cy-filter-presets{display:flex;gap:7px;margin-top:7px}.cy-filter-presets button{font-size:11px;padding:4px 8px;background:var(--cy-card);border:0;border-radius:4px;color:var(--cy-muted)}.cy-filter-chips{display:flex;flex-wrap:wrap;gap:8px}.cy-filter-chips button{padding:8px 10px;border:0;border-radius:6px;background:var(--cy-card);color:var(--cy-ink);font-size:12px}.cy-filter-chips button.active{background:var(--cy-soft);color:var(--cy-accent);box-shadow:inset 0 0 0 1px var(--cy-accent)}.cy-filter-options{display:flex;flex-direction:column;padding:8px 0;gap:0;max-height:240px;overflow:auto}.cy-filter-options label{display:flex;align-items:center;gap:10px;padding:10px 0;font-size:13px}.cy-filter-options input{accent-color:var(--cy-accent);width:17px;height:17px}.cy-filter-options p{font-size:12px;color:var(--cy-muted)}.cy-filter-match{display:flex;background:var(--cy-card);border-radius:5px;overflow:hidden}.cy-filter-match button{padding:5px 7px;border:0;background:transparent;color:var(--cy-muted);font-size:11px}.cy-filter-match button.active{background:var(--cy-soft);color:var(--cy-accent)}footer{flex:none;padding:12px 18px max(15px,env(safe-area-inset-bottom));background:var(--cy-bg)}footer>div{display:flex;gap:18px}footer button{flex:1;border:0;border-radius:7px;background:var(--cy-card);padding:12px;font-size:15px;color:var(--cy-ink)}footer .cy-filter-confirm{background:var(--cy-accent);color:white}footer p{color:var(--cy-expense);font-size:12px;margin-bottom:8px}
</style>
