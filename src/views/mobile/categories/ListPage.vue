<template>
    <f7-page class="cy-mobile-surface cy-asset-surface" @page:afterin="load">
        <f7-navbar back-link="返回">
            <f7-nav-title>
                <nav v-if="!parentId" class="management-tabs" aria-label="分类类型"><button v-for="tab in types" :key="tab.id" :aria-pressed="categoryType === tab.id" @click="switchType(tab.id)">{{ tab.name }}</button></nav>
                <span v-else>{{ parentCategory?.name || '二级分类' }}</span>
            </f7-nav-title>
            <f7-nav-right><f7-link icon-f7="ellipsis_vertical" aria-label="分类管理更多操作" @click="menuOpen = true" /></f7-nav-right>
        </f7-navbar>
        <main class="management-body">
            <label v-if="searchOpen" class="management-search"><f7-icon f7="search" /><input v-model="query" type="search" placeholder="搜索分类" aria-label="搜索分类" /><button class="management-action" aria-label="关闭搜索" @click="query = ''; searchOpen = false"><f7-icon f7="xmark" /></button></label>
            <label v-if="scopeOpen || scopeBookId" class="management-filter">查看范围<select v-model="scopeBookId" aria-label="分类所属账本"><option value="">全部账本</option><option v-for="book in books.allBooks" :key="book.id" :value="book.id">{{ book.name }}</option></select></label>
            <p v-if="showHidden" class="management-state"><f7-icon f7="eye_slash" size="14" />正在显示隐藏分类<button @click="showHidden = false">收起</button></p>
            <p v-if="error" class="cy-message" role="alert">{{ error }}<button @click="load">重试</button></p>
            <f7-link class="management-card management-add" :href="addLink(parentCategory)"><f7-icon f7="plus_circle" />{{ parentId ? '新增二级分类' : '新增分类' }}</f7-link>
            <p v-if="loading" class="cy-empty">正在加载分类…</p>
            <template v-else>
                <section v-for="category in visibleCategories" :key="category.id" class="management-card">
                    <div class="management-head">
                        <button class="management-main" :class="{ 'management-hidden': category.hidden }" :aria-expanded="!parentId ? expanded(category) : undefined" @click="parentId ? openActions(category) : toggle(category.id)">
                            <f7-icon v-if="!parentId" class="category-chevron" :f7="expanded(category) ? 'chevron_down' : 'chevron_right'" />
                            <span class="management-icon"><ItemIcon :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" :color="category.color" /></span>
                            <span class="management-name">{{ category.name }}<small v-if="category.hidden">已隐藏</small></span>
                        </button>
                        <button class="management-action" :aria-label="`${category.name}的更多操作`" @click="openActions(category)"><f7-icon f7="text_alignleft" /></button>
                    </div>
                    <div v-if="!parentId && expanded(category)" class="category-children">
                        <button v-for="child in children(category)" :key="child.id" :class="{ 'management-hidden': child.hidden }" :aria-label="`${child.name}的更多操作`" @click="openActions(child)">
                            <ItemIcon :icon-type="getCategoryIconType(child.iconType)" :icon-id="child.icon" :color="child.color" /><span>{{ child.name }}</span><small v-if="child.hidden">已隐藏</small>
                        </button>
                        <f7-link :href="addLink(category)" :aria-label="`为${category.name}新增二级分类`"><f7-icon f7="plus_circle" /><span>新增</span></f7-link>
                    </div>
                </section>
                <p v-if="!visibleCategories.length" class="cy-empty">{{ query ? '没有找到匹配的分类' : '此范围还没有分类' }}<br /><f7-link v-if="!categories.length && !parentId" :href="`/category/preset?type=${categoryType}`">添加默认分类</f7-link></p>
            </template>
        </main>
        <f7-actions :opened="menuOpen" @actions:closed="menuOpen = false">
            <f7-actions-group>
                <f7-actions-button @click="searchOpen = true">搜索分类</f7-actions-button>
                <f7-actions-button @click="scopeOpen = !scopeOpen; scopeBookId = ''">按账本查看</f7-actions-button>
                <f7-actions-button @click="showHidden = !showHidden">{{ showHidden ? '收起隐藏分类' : '显示隐藏分类' }}</f7-actions-button>
                <f7-actions-button :class="{ disabled: categories.length < 2 }" @click="openOrder(parentCategory)">调整分类顺序</f7-actions-button>
                <f7-actions-button v-if="!parentId" @click="expandAll = !expandAll; openIds = new Set()">{{ expandAll ? '收起全部分类' : '展开全部分类' }}</f7-actions-button>
                <f7-actions-button v-if="!parentId" @click="f7router.navigate(`/category/preset?type=${categoryType}`)">添加默认分类</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group><f7-actions-button bold>取消</f7-actions-button></f7-actions-group>
        </f7-actions>
        <f7-actions :opened="actionsOpen" @actions:closed="actionsOpen = false">
            <f7-actions-group>
                <f7-actions-label>{{ selected?.name }}</f7-actions-label>
                <f7-actions-button @click="selected && f7router.navigate(`/category/edit?id=${selected.id}`)">编辑分类</f7-actions-button>
                <f7-actions-button v-if="selected?.parentId === '0'" @click="selected && f7router.navigate(addLink(selected))">新增二级分类</f7-actions-button>
                <f7-actions-button v-if="(selected?.subCategories?.length || 0) > 1" @click="openOrder(selected)">调整二级分类顺序</f7-actions-button>
                <f7-actions-button @click="viewBills">查看账单</f7-actions-button>
                <f7-actions-button :class="{ disabled: busy }" @click="toggleHidden">{{ selected?.hidden ? '显示分类' : '隐藏分类' }}</f7-actions-button>
                <f7-actions-button color="red" :class="{ disabled: busy }" @click="confirmDelete">删除分类</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group><f7-actions-button bold>取消</f7-actions-button></f7-actions-group>
        </f7-actions>
        <ManagementOrderPopup v-model:open="orderOpen" :title="orderTitle" :items="orderItems" :busy="busy" :error="orderError" @save="saveOrder" />
    </f7-page>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import type { Router } from 'framework7/types';
import ManagementOrderPopup from '@/components/mobile/ManagementOrderPopup.vue';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useI18nUIComponents } from '@/lib/ui/mobile.ts';
import { useManagementFeedback } from '@/lib/use-management-feedback.ts';
const { errorText } = useManagementFeedback();
import { getCategoryIconType } from '@/lib/icon.ts';
import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import services from '@/lib/services.ts';
const { f7route, f7router } = defineProps<{ f7route: Router.Route; f7router: Router.Router }>();
const store = useTransactionCategoriesStore(), books = useBooksStore();
const { showConfirm, showToast } = useI18nUIComponents();
const types = [{ id: CategoryType.Expense, name: '支出' }, { id: CategoryType.Income, name: '收入' }, { id: CategoryType.Transfer, name: '转账' }];
const routeType = Number(f7route.query['type']);
const categoryType = ref<CategoryType>(types.some(type => type.id === routeType) ? routeType : CategoryType.Expense);
const parentId = f7route.query['id'] && f7route.query['id'] !== '0' ? f7route.query['id'] : '';
const parentCategory = computed(() => store.allTransactionCategoriesMap[parentId]);
const loading = ref(true), busy = ref(false), error = ref(''), orderError = ref('');
const query = ref(''), searchOpen = ref(false), scopeOpen = ref(false), showHidden = ref(false), expandAll = ref(false);
const scopeBookId = ref(books.selectedBookIds.length === 1 ? books.selectedBookIds[0]! : '');
const openIds = ref(new Set<string>()), menuOpen = ref(false), actionsOpen = ref(false), orderOpen = ref(false);
const selected = ref<TransactionCategory>(), orderItems = ref<TransactionCategory[]>([]), orderTitle = ref('分类排序');
const categories = computed(() => parentId ? parentCategory.value?.subCategories || [] : store.allTransactionCategories[categoryType.value] || []);
function withinScope(category: TransactionCategory) { return (showHidden.value || !category.hidden) && (!scopeBookId.value || !category.bookIds.length || category.bookIds.includes(scopeBookId.value)); }
function matches(category: TransactionCategory) { return `${category.name} ${category.comment}`.toLocaleLowerCase().includes(query.value.trim().toLocaleLowerCase()); }
function children(category: TransactionCategory) { return (category.subCategories || []).filter(child => withinScope(child) && (matches(category) || matches(child))); }
const visibleCategories = computed(() => categories.value.filter(category => withinScope(category) && (matches(category) || children(category).length > 0)));
function expanded(category: TransactionCategory) { return !!query.value.trim() || (expandAll.value ? !openIds.value.has(category.id) : openIds.value.has(category.id)); }
function toggle(id: string) { const next = new Set(openIds.value); if (next.has(id)) next.delete(id); else next.add(id); openIds.value = next; }
function switchType(type: CategoryType) { categoryType.value = type; query.value = ''; openIds.value = new Set(); expandAll.value = false; }
function addLink(parent?: TransactionCategory) {
    return '/category/add?' + new URLSearchParams({ type: String(categoryType.value), parentId: parent?.id || '0', ...(parent ? { icon: parent.icon, color: parent.color } : {}) });
}
function openActions(category: TransactionCategory) { selected.value = category; actionsOpen.value = true; }
async function load() {
    if (busy.value) return; error.value = '';
    try { await Promise.all([store.loadAllCategories({ force: false }), books.loadBooks()]); }
    catch (cause) { error.value = errorText(cause); }
    finally { loading.value = false; }
}
async function toggleHidden() {
    const category = selected.value; if (!category || busy.value) return;
    busy.value = true; error.value = '';
    try { await store.hideCategory({ category, hidden: !category.hidden }); }
    catch (cause) { error.value = errorText(cause); }
    finally { busy.value = false; }
}
function confirmDelete() {
    const category = selected.value; if (!category || busy.value) return;
    showConfirm(`确定删除「${category.name}」${category.subCategories?.length ? `及其 ${category.subCategories.length} 个二级分类` : ''}？已有账单引用的分类不能直接删除，可选择隐藏。`, () => { void remove(category); });
}
async function remove(category: TransactionCategory) {
    if (busy.value) return; busy.value = true; error.value = '';
    try { await store.deleteCategory({ category }); showToast('分类已删除'); }
    catch (cause) { error.value = errorText(cause); }
    finally { busy.value = false; }
}
function viewBills() { const category = selected.value; if (category) f7router.navigate('/transaction/list?' + new URLSearchParams({ categoryIds: category.id, ...(scopeBookId.value ? { bookIds: scopeBookId.value } : {}) })); }
function openOrder(parent?: TransactionCategory) {
    const items = parent ? parent.subCategories || [] : categories.value; if (items.length < 2) return;
    orderItems.value = [...items]; orderTitle.value = parent ? `${parent.name} · 二级排序` : '分类排序'; orderError.value = ''; orderOpen.value = true;
}
async function saveOrder(ids: string[]) {
    if (busy.value) return; busy.value = true; orderError.value = '';
    try {
        const response = await services.moveTransactionCategory({ newDisplayOrders: ids.map((id, index) => ({ id, displayOrder: index + 1 })) });
        if (!response.data.success || !response.data.result) throw new Error('分类顺序保存失败，请重试');
        store.updateTransactionCategoryListInvalidState(true); await store.loadAllCategories({ force: false }); orderOpen.value = false;
    } catch (cause) { orderError.value = errorText(cause); }
    finally { busy.value = false; }
}
void load();
</script>
<style scoped src="@/styles/mobile/management.css"></style>
<style scoped>
.category-chevron{font-size:11px;color:var(--cy-muted);margin-right:1px;width:12px;flex-shrink:0}
.category-children{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:8px 0;padding:2px 10px 17px}
.category-children button,.category-children a{display:flex;flex-direction:column;align-items:center;justify-content:flex-start;gap:9px;min-height:69px;min-width:0;padding:10px 2px 4px;border:0;background:none;color:var(--cy-ink)}
.category-children :deep(.icon){font-size:25px}.category-children>a>.icon{color:var(--cy-muted)}
.category-children span{font-size:11px;line-height:1.5;text-align:center;overflow-wrap:anywhere;max-width:100%}.category-children small{font-size:9px;margin-top:-7px;color:var(--cy-muted)}
@media(max-width:350px){.category-children{grid-template-columns:repeat(4,minmax(0,1fr))}}
</style>
