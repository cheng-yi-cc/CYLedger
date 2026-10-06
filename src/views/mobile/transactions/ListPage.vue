<template>
    <f7-page class="cy-mobile-surface cy-ledger-search cy-asset-surface" @page:afterin="refresh">
        <f7-navbar class="cy-search-navbar"><f7-nav-left><f7-link back icon-f7="chevron_left" aria-label="返回" /></f7-nav-left><f7-nav-title><form @submit.prevent="submit"><input ref="searchInput" v-model="query" maxlength="255" enterkeyhint="search" autocomplete="off" :placeholder="pictureView ? '搜索账单图片' : '搜索账单、标签（#触发，空格结束）'" aria-label="搜索账单和标签" @input="changed" /><button v-if="query" type="button" aria-label="清除搜索词" @click="query = ''; submitted = false">×</button></form></f7-nav-title><f7-nav-right><f7-link @click="showFilters = true">筛选<sup v-if="filterCount">{{ filterCount }}</sup></f7-link></f7-nav-right></f7-navbar>
        <main class="cy-search-body">
            <div v-if="tagSuggestions.length" class="cy-search-tags" aria-label="标签建议"><button v-for="tag in tagSuggestions" :key="tag.id" @click="chooseTag(tag.name)">#{{ tag.name }}</button></div>
            <section v-if="!submitted && !pictureView" class="cy-search-history"><button v-if="experience.history.length" class="cy-clear-history" @click="clearHistory">清空历史记录</button><div v-for="value in experience.history" :key="value"><button @click="query = value; submit()"><f7-icon f7="clock" />{{ value }}</button><button aria-label="删除这条搜索历史" @click="experience.forgetSearch(value)"><f7-icon f7="xmark" /></button></div><p v-if="!experience.history.length" class="cy-search-hint">按分类、备注、账户或标签查找账单</p></section>
            <template v-else>
                <p v-if="error" class="cy-message" role="alert">{{ error }} <button @click="refresh">重试</button></p><p v-else-if="loading" class="cy-empty">正在查找账单…</p>
                <template v-else>
                    <div class="cy-search-summary"><span>共 {{ results.length }} 笔</span><span>收 <b class="cy-income">{{ totals.complete ? ledgerMoney(totals.income,false) : '—' }}</b>　支 <b class="cy-expense">{{ totals.complete ? ledgerMoney(totals.expense,false) : '—' }}</b></span></div>
                    <div class="cy-search-tools"><button @click="selection = !selection; selectedIds = []">{{ selection ? '取消多选' : '多选' }}</button><select v-model="sort" aria-label="结果排序"><option value="newest">时间从新到旧</option><option value="oldest">时间从旧到新</option><option value="large">金额从大到小</option><option value="small">金额从小到大</option></select><button :disabled="!results.length" @click="exportEntries(results)">导出</button></div>
                    <div v-if="pictureView" class="cy-picture-grid"><f7-link v-for="image in images" :key="image.picture.pictureId" :href="'/transaction/detail?id='+image.entry.id+'&type='+image.entry.type"><img :src="services.getTransactionPictureUrlWithToken(image.picture.originalUrl)" alt="账单附件" loading="lazy" /><span>{{ image.entry.day }} · {{ image.entry.title }}</span></f7-link></div>
                    <template v-else><LedgerDayList v-if="sort === 'newest'" :entries="visible" :selection-mode="selection" :selected-ids="selectedIds" @select="toggleSelected" @hold="startSelection" /><LedgerDayList v-else v-for="item in visible" :key="item.id" :entries="[item]" :selection-mode="selection" :selected-ids="selectedIds" @select="toggleSelected" @hold="startSelection" /></template>
                    <p v-if="!results.length" class="cy-empty">没有找到符合条件的账单</p><button v-if="visibleLimit < results.length" class="cy-search-more" @click="visibleLimit += 100">继续加载</button>
                </template>
            </template>
        </main>
        <template #fixed><div v-if="selection" class="cy-bill-selection"><div><button @click="selectedIds = selectedIds.length === selectable.length ? [] : selectable.map(item => item.id)">{{ selectedIds.length === selectable.length ? '全不选' : '全选' }}</button><span>已选 {{ selectedIds.length }} 笔</span><button @click="selection = false">完成</button></div><div><button :disabled="!selectedIds.length || busy" @click="showMove = true">移动账本</button><button :disabled="!selectedIds.length || busy" @click="exportEntries(selected)">导出</button><button :disabled="!selectedIds.length || busy" class="cy-expense" @click="deleteSelected">删除</button></div></div></template>
        <LedgerSearchFilters v-model:opened="showFilters" v-model="filters" />
        <f7-sheet class="cy-mobile-surface cy-search-move" v-model:opened="showMove" backdrop><f7-toolbar><div class="left">移动到账本</div><div class="right"><f7-link sheet-close>取消</f7-link></div></f7-toolbar><f7-list><f7-list-item v-for="book in books.activeBooks" :key="book.id" :title="book.name" link="#" @click="moveSelected(book.id)" /></f7-list></f7-sheet>
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import {f7} from 'framework7-vue';
import moment from 'moment-timezone';
import type { Router } from 'framework7/types';
import LedgerDayList from '@/components/mobile/LedgerDayList.vue';
import LedgerSearchFilters from '@/components/mobile/LedgerSearchFilters.vue';
import { useMobileLedger, LedgerDecimal, ledgerMoney, ledgerTotals, type LedgerEntry } from '@/lib/mobile-ledger.ts';
import { emptyLedgerSearchFilters, filterLedgerEntries, ledgerFilterCount } from '@/lib/ledger-search.ts';
import { useLedgerExperienceStore } from '@/stores/ledgerExperience.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { exportLedgerEntries, exportLedgerWorkbook, ledgerEntryRows } from '@/lib/ledger-export.ts';
import { investmentError } from '@/lib/investments.ts';
import services from '@/lib/services.ts';
const props = defineProps<{ f7route: Router.Route }>();
const route = props.f7route.query;
const experience = useLedgerExperienceStore(), scope = useLedgerScopeStore(), tags = useTransactionTagsStore(), books = useBooksStore(), transactions = useTransactionsStore();
const { showConfirm, showToast } = useI18nUIComponents();
const { entries, loading, error, load } = useMobileLedger({ applyFilters: () => false, bookIds: () => [] });
const query = ref(route['keyword'] || ''), filters = ref(emptyLedgerSearchFilters()), submitted = ref(false), showFilters = ref(false), sort = ref('newest'), visibleLimit = ref(100), selection = ref(false), selectedIds = ref<string[]>([]), busy = ref(false), showMove = ref(false);
const searchInput = ref<HTMLInputElement>();
const pictureView = computed(() => route['view'] === 'pictures');
for (const key of ['accountIds', 'categoryIds', 'bookIds'] as const) if (route[key]) filters.value[key] = route[key]!.split(',').filter(Boolean);
if (!route['bookIds'] && books.selectedBookIds.length) filters.value.bookIds = [...books.selectedBookIds];
if (route['minTime'] && Number(route['minTime']) > 0) filters.value.start = moment.unix(Number(route['minTime'])).tz(scope.timeZone).format('YYYY-MM-DD');
if (route['maxTime'] && Number(route['maxTime']) > 0) filters.value.end = moment.unix(Number(route['maxTime'])).tz(scope.timeZone).format('YYYY-MM-DD');
if (route['type'] === '2' || route['type'] === '3' || route['type'] === '4') filters.value.flags.push(route['type'] === '2' ? 'income' : route['type'] === '3' ? 'expense' : 'transfer');
if (pictureView.value) filters.value.flags.push('picture');
submitted.value = !!query.value || Object.keys(route).some(key => key !== 'search');
const filterCount = computed(() => ledgerFilterCount(filters.value));
const results = computed(() => filterLedgerEntries(entries.value, query.value, filters.value).sort((a,b) => sort.value === 'newest' ? b.time-a.time : sort.value === 'oldest' ? a.time-b.time : a.currency.localeCompare(b.currency) || new LedgerDecimal(a.amount).abs().comparedTo(new LedgerDecimal(b.amount).abs()) * (sort.value === 'large' ? -1 : 1)));
const visible = computed(() => results.value.slice(0,visibleLimit.value)), totals = computed(() => ledgerTotals(results.value));
const images = computed(() => visible.value.flatMap(entry => (entry.pictures || []).map(picture => ({ entry, picture }))));
const selectable = computed(()=>results.value.filter(item=>!item.investment&&(!item.transferFeeParentId||item.transferFeeParentId==='0')));
const selected = computed(() => results.value.filter(item => selectedIds.value.includes(item.id)));
const tagSuggestions = computed(() => { const match = query.value.match(/#([^\s#]*)$/); return match ? Object.values(tags.allTransactionTagsMap).filter(tag => tag.name.includes(match[1] || '')).slice(0,12) : []; });
watch(filters, () => { submitted.value = true; visibleLimit.value = 100; selection.value = false; selectedIds.value = []; }, { deep: true });
function chooseTag(name: string) { query.value = query.value.replace(/#[^\s#]*$/, '#'+name+' '); searchInput.value?.focus(); }
function changed() { submitted.value=!!query.value; visibleLimit.value=100; selectedIds.value=[]; }
function submit() { submitted.value = true; visibleLimit.value = 100; experience.rememberSearch(query.value); searchInput.value?.blur(); }
function clearHistory() { showConfirm('清空搜索历史？', () => experience.forgetSearch()); }
async function refresh() { await load(0,moment().tz(scope.timeZone).add(100,'years').unix()); if(route['selectId'] && entries.value.some(e=>e.id===route['selectId'])){submitted.value=true;selection.value=true;selectedIds.value=[route['selectId']];} }
function toggleSelected(item: LedgerEntry) { if (item.transferFeeParentId && item.transferFeeParentId!=='0') { showToast('关联手续费请选中原转账账单处理'); return; } if (item.investment) { showToast('投资结算请到投资流水中处理'); return; } selectedIds.value = selectedIds.value.includes(item.id) ? selectedIds.value.filter(id => id !== item.id) : [...selectedIds.value,item.id]; }
function startSelection(item: LedgerEntry) { if (item.investment) return; selection.value = true; toggleSelected(item); }
function exportEntries(items: LedgerEntry[]) { f7.actions.create({buttons:[[{text:'导出 CSV',onClick:()=>exportLedgerEntries(items,scope.timeZone)},{text:'导出 Excel',onClick:()=>exportLedgerWorkbook(ledgerEntryRows(items,scope.timeZone))}],[{text:'取消',color:'gray'}]]}).open(); }
async function moveSelected(bookId: string) { if (busy.value) return; busy.value = true; try { await books.moveTransactions(selectedIds.value,bookId); showMove.value = false; selection.value = false; selectedIds.value = []; await refresh(); } catch(cause) { showToast(investmentError(cause)); } finally { busy.value = false; } }
function deleteSelected() { const ids = [...selectedIds.value]; showConfirm(`删除选中的 ${ids.length} 笔账单？账户余额将相应恢复。`, async () => { busy.value = true; let removed = 0; try { for (const id of ids) { const result = await services.deleteTransaction({id}); if (!result.data.success || !result.data.result) throw Error('删除失败'); removed++; } showToast(`已删除 ${removed} 笔账单`); } catch(cause) { showToast(`已删除 ${removed} 笔，其余未删除：${investmentError(cause)}`); } finally { transactions.updateStoreInvalidState({transactionList:true,statistics:true,overview:true,explorer:true}); selection.value = false; selectedIds.value = []; busy.value = false; await refresh(); } }); }
</script>
<style scoped>
.cy-search-navbar :deep(.title){flex:1!important;min-width:0!important;margin:0!important}.cy-search-navbar :deep(.navbar-inner){gap:0!important}.cy-search-navbar :deep(.left),.cy-search-navbar :deep(.right){min-width:38px!important}.cy-search-navbar :deep(.right .link){font-size:14px!important;padding:0 10px!important}.cy-search-navbar form{display:flex;align-items:center;min-width:0;width:100%}.cy-search-navbar input{border:0;background:transparent;min-width:0;flex:1;font:inherit;font-size:12px;color:var(--cy-ink);height:46px}.cy-search-navbar input::placeholder{color:var(--cy-muted)}.cy-search-navbar form button{background:transparent;border:0;color:var(--cy-muted);padding:8px;font-size:20px}.cy-search-navbar sup{font-size:10px;color:var(--cy-accent);margin-left:2px}.cy-search-body{padding:0 14px 125px;max-width:720px;margin:auto}.cy-clear-history{display:block;margin:0 auto 10px;background:transparent;border:0;color:var(--cy-muted);padding:14px;font-size:12px}.cy-search-history>div{display:flex;align-items:center;min-height:49px;border-bottom:1px solid var(--cy-line)}.cy-search-history>div button{display:flex;align-items:center;gap:18px;flex:1;background:transparent;border:0;text-align:left;color:var(--cy-ink);font-size:15px;padding:12px 5px}.cy-search-history>div button:last-child{flex:0 0 40px;justify-content:center;color:var(--cy-muted)}.cy-search-history .icon{font-size:17px;color:var(--cy-muted)}.cy-search-hint{text-align:center;font-size:12px;color:var(--cy-muted);padding:50px 12px}.cy-search-tags{display:flex;flex-wrap:wrap;gap:8px;margin:8px 0}.cy-search-tags button{background:var(--cy-soft);color:var(--cy-accent);border:0;padding:7px 10px;border-radius:6px;font-size:12px}.cy-search-summary{display:flex;justify-content:space-between;gap:8px;flex-wrap:wrap;font-size:12px;padding:14px 0}.cy-search-summary b{font-weight:500}.cy-search-tools{display:flex;justify-content:space-between;align-items:center;font-size:12px;margin-bottom:18px;color:var(--cy-muted)}.cy-search-tools button,.cy-search-tools select{border:0;background:transparent;color:inherit;font:inherit;padding:5px}.cy-search-more{display:block;width:100%;border:0;background:var(--cy-card);color:var(--cy-accent);padding:14px;border-radius:8px}.cy-bill-selection{position:absolute;bottom:0;left:0;right:0;z-index:550;background:var(--cy-card);border-top:1px solid var(--cy-line);padding:8px 14px max(12px,env(safe-area-inset-bottom))}.cy-bill-selection>div{display:flex;align-items:center;justify-content:space-between;gap:14px;font-size:13px}.cy-bill-selection button{border:0;background:transparent;color:var(--cy-accent);padding:11px;font-size:14px;flex:1}.cy-search-move{height:auto;max-height:65vh;overflow:auto}.cy-search-move :deep(.sheet-modal-inner){padding-top:var(--f7-toolbar-height)}.cy-picture-grid{display:grid;grid-template-columns:repeat(3,minmax(0,1fr));gap:8px}.cy-picture-grid a{display:flex;flex-direction:column;gap:6px;min-width:0;color:var(--cy-muted)}.cy-picture-grid img{width:100%;aspect-ratio:1;object-fit:cover;border-radius:7px}.cy-picture-grid span{font-size:10px;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
</style>
