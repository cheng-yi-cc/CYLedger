<template>
    <f7-page class="cy-mobile-surface cy-category-management" :ptr="!sortable" @ptr:refresh="reload" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')" v-if="!sortable"></f7-nav-left>
            <f7-nav-left v-else-if="sortable">
                <f7-link icon-f7="xmark" :class="{ 'disabled': displayOrderSaving }" :aria-label="tt('Cancel')" @click="cancelSort"></f7-link>
            </f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" :class="{ 'disabled': !categories.length || sortable }" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="plus" :aria-label="tt('Add')" :href="'/category/add?type=' + categoryType + '&parentId=' + primaryCategoryId + (currentPrimaryCategory ? `&color=${currentPrimaryCategory.color}&icon=${currentPrimaryCategory.icon}` : '')" v-if="!sortable"></f7-link>
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': displayOrderSaving || !displayOrderModified }" :aria-label="tt('Save')" @click="saveSortResult" v-else-if="sortable"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <section v-if="!loading" class="cy-category-tools">
            <nav v-if="hasSubCategories" class="cy-segments" aria-label="分类类型"><f7-link :class="{ 'cy-type-active': categoryType === CategoryType.Expense }" href="/category/list?type=2">支出</f7-link><f7-link :class="{ 'cy-type-active': categoryType === CategoryType.Income }" href="/category/list?type=1">收入</f7-link><f7-link :class="{ 'cy-type-active': categoryType === CategoryType.Transfer }" href="/category/list?type=3">转账</f7-link></nav>
            <p v-else class="cy-parent-label">{{ currentPrimaryCategory?.name }} <span>· 二级分类</span></p>
            <label class="cy-category-scope-filter">查看范围<select v-model="scopeBookId" :disabled="sortable" aria-label="分类所属账本"><option value="">全部账本</option><option v-for="book in booksStore.allBooks" :key="book.id" :value="book.id">{{ book.name }}</option></select></label>
            <label class="cy-management-search"><f7-icon f7="search" /><input v-model="searchText" :disabled="sortable" type="search" placeholder="查找分类" aria-label="查找分类" /></label>
            <div class="cy-management-actions"><button :aria-pressed="showHidden" :disabled="sortable" @click="showHidden = !showHidden">{{ showHidden ? '收起隐藏分类' : '显示隐藏分类' }}</button><button :disabled="sortable || categories.length < 2" @click="searchText = ''; setSortable()">调整顺序</button><f7-link href="/tag/list">管理标签</f7-link></div>
            <p class="cy-management-hint">{{ sortable ? '拖动右侧把手排序，完成后点右上角保存。' : '点击铅笔编辑名称、图标和颜色；滑动分类可删除。' }}</p>
            <p v-if="searchText && !filteredCategoryCount" class="cy-management-hint">没有找到匹配的分类。</p>
        </section>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-item title="Category Name"
                          :link="hasSubCategories ? '#' : null"
                          :key="itemIdx" v-for="itemIdx in [ 1, 2, 3 ]">
                <template #media>
                    <f7-icon f7="app_fill"></f7-icon>
                </template>
            </f7-list-item>
        </f7-list>

        <f7-list strong inset dividers class="margin-top-half" v-if="!loading && noAvailableCategory">
            <f7-list-item :title="tt('No available category')"></f7-list-item>
            <f7-list-button v-if="hasSubCategories && noCategory"
                            :title="tt('Add Default Categories')"
                            :href="'/category/preset?type=' + categoryType"></f7-list-button>
        </f7-list>

        <f7-list strong inset dividers sortable class="margin-top-half category-list"
                 :sortable-enabled="sortable"
                 v-if="!loading"
                 @sortable:sort="onSort">
            <f7-list-item swipeout
                          :class="{ 'actual-first-child': category.id === firstShowingId, 'actual-last-child': category.id === lastShowingId }"
                          :id="getCategoryDomId(category)"
                          :title="category.name"
                          :footer="category.comment"
                          :link="hasSubCategories ? '/category/list?type=' + categoryType + '&id=' + category.id : null"
                          :key="category.id"
                          v-for="category in categories"
                          v-show="(showHidden || !category.hidden) && matchesSearch(category)"
                          @taphold="setSortable()">
                <template #media>
                    <ItemIcon :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" :color="category.color">
                        <f7-badge color="gray" class="right-bottom-icon" v-if="category.hidden">
                            <f7-icon f7="eye_slash_fill"></f7-icon>
                        </f7-badge>
                    </ItemIcon>
                </template>
                <template #after v-if="!sortable">
                    <button class="cy-category-edit" :aria-label="`编辑${category.name}`" @click.stop.prevent="edit(category)"><f7-icon f7="pencil" /></button>
                    <button class="cy-category-edit" :aria-label="`${category.hidden ? '显示' : '隐藏'}${category.name}`" @click.stop.prevent="hide(category, !category.hidden)"><f7-icon :f7="category.hidden ? 'eye_slash' : 'eye'" /></button>
                </template>
                <f7-swipeout-actions :left="textDirection === TextDirection.LTR"
                                     :right="textDirection === TextDirection.RTL"
                                     v-if="sortable">
                    <f7-swipeout-button class="padding-horizontal" overswipe close
                                        :aria-label="category.hidden ? tt('Show') : tt('Hide')"
                                        :color="category.hidden ? 'blue' : 'gray'"
                                        @click="hide(category, !category.hidden)">
                        <f7-icon :f7="category.hidden ? 'eye' : 'eye_slash'"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
                <f7-swipeout-actions :left="textDirection === TextDirection.RTL"
                                     :right="textDirection === TextDirection.LTR"
                                     v-if="!sortable">
                    <f7-swipeout-button color="orange" close :text="tt('Edit')" @click="edit(category)"></f7-swipeout-button>
                    <f7-swipeout-button color="red" class="padding-horizontal" :aria-label="tt('Delete')" @click="remove(category, false)">
                        <f7-icon f7="trash"></f7-icon>
                    </f7-swipeout-button>
                </f7-swipeout-actions>
            </f7-list-item>
        </f7-list>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button :class="{ 'disabled': !categories || categories.length < 2 }" @click="setSortable()">{{ tt('Sort') }}</f7-actions-button>
                <f7-actions-button v-if="!showHidden" @click="showHidden = true">{{ tt('Show Hidden Transaction Categories') }}</f7-actions-button>
                <f7-actions-button v-if="showHidden" @click="showHidden = false">{{ tt('Hide Hidden Transaction Categories') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to delete this category?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="remove(categoryToDelete, true)">{{ tt('Delete') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading, onSwipeoutDeleted } from '@/lib/ui/mobile.ts';
import { useCategoryListPageBase } from '@/views/base/categories/CategoryListPageBase.ts';

import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useBooksStore } from '@/stores/books.ts';

import { TextDirection } from '@/core/text.ts';
import { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';

import { getCategoryIconType } from '@/lib/icon.ts';
import {
    isNoAvailableCategory,
    getFirstShowingId,
    getLastShowingId
} from '@/lib/category.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const { tt, getCurrentLanguageTextDirection } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const { loading, primaryCategoryId, currentPrimaryCategory } = useCategoryListPageBase();

const transactionCategoriesStore = useTransactionCategoriesStore();
const booksStore = useBooksStore();
const scopeBookId = ref(booksStore.selectedBookIds.length === 1 ? booksStore.selectedBookIds[0]! : '');
void booksStore.loadBooks().catch(() => showToast('账本列表加载失败，请返回后重试。'));

const hasSubCategories = ref<boolean>(false);
const categoryType = ref<CategoryType | 0>(0);
const loadingError = ref<unknown | null>(null);
const showHidden = ref<boolean>(false);
const searchText = ref('');
const sortable = ref<boolean>(false);
const categoryToDelete = ref<TransactionCategory | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);
const displayOrderModified = ref<boolean>(false);
const displayOrderSaving = ref<boolean>(false);

const textDirection = computed<TextDirection>(() => getCurrentLanguageTextDirection());

const categories = computed<TransactionCategory[]>(() => {
    if (!primaryCategoryId.value || primaryCategoryId.value === '' || primaryCategoryId.value === '0') {
        if (!transactionCategoriesStore.allTransactionCategories || !transactionCategoriesStore.allTransactionCategories[categoryType.value]) {
            return [];
        }

        return transactionCategoriesStore.allTransactionCategories[categoryType.value] ?? [];
    } else if (primaryCategoryId.value && primaryCategoryId.value !== '' && primaryCategoryId.value !== '0') {
        if (!transactionCategoriesStore.allTransactionCategoriesMap || !transactionCategoriesStore.allTransactionCategoriesMap[primaryCategoryId.value]) {
            return [];
        }

        return transactionCategoriesStore.allTransactionCategoriesMap[primaryCategoryId.value]?.subCategories ?? [];
    } else {
        return [];
    }
});

const title = computed<string>(() => {
    let title = '';

    switch (categoryType.value) {
        case CategoryType.Income:
            title = 'Income';
            break;
        case CategoryType.Expense:
            title = 'Expense';
            break;
        case CategoryType.Transfer:
            title = 'Transfer';
            break;
        default:
            title = 'Transaction';
            break;
    }

    switch (hasSubCategories.value) {
        case true:
            title += ' Primary';
            break;
        case false:
            title += ' Secondary';
            break;
    }

    return title + ' Categories';
});

const firstShowingId = computed<string | null>(() => getFirstShowingId(categories.value, showHidden.value));
const lastShowingId = computed<string | null>(() => getLastShowingId(categories.value, showHidden.value));
const noAvailableCategory = computed<boolean>(() => isNoAvailableCategory(categories.value, showHidden.value));
const noCategory = computed<boolean>(() => categories.value.length < 1);
const filteredCategoryCount = computed(() => categories.value.filter(category => (showHidden.value || !category.hidden) && matchesSearch(category)).length);
function matchesSearch(category: TransactionCategory): boolean {
    if (scopeBookId.value && category.bookIds.length && !category.bookIds.includes(scopeBookId.value)) return false;
    const query = searchText.value.trim().toLocaleLowerCase();
    return !query || `${category.name} ${category.comment}`.toLocaleLowerCase().includes(query);
}

function getCategoryDomId(category: TransactionCategory): string {
    return 'category_' + category.id;
}

function parseCategoryIdFromDomId(domId: string): string | null {
    if (!domId || domId.indexOf('category_') !== 0) {
        return null;
    }

    return domId.substring(9); // category_
}

function init(): void {
    const query = props.f7route.query;

    categoryType.value = parseInt(query['type'] || '0');

    if (categoryType.value !== CategoryType.Income &&
        categoryType.value !== CategoryType.Expense &&
        categoryType.value !== CategoryType.Transfer) {
        showToast('Parameter Invalid');
        loadingError.value = 'Parameter Invalid';
        return;
    }

    if (query['id'] && query['id'] !== '0') {
        primaryCategoryId.value = query['id'];
        hasSubCategories.value = false;
    } else {
        primaryCategoryId.value = '0';
        hasSubCategories.value = true;
    }

    loading.value = true;

    transactionCategoriesStore.loadAllCategories({
        force: false
    }).then(() => {
        loading.value = false;
    }).catch(error => {
        if (error.processed) {
            loading.value = false;
        } else {
            loadingError.value = error;
            showToast(error.message || error);
        }
    });
}

function reload(done?: () => void): void {
    if (sortable.value) {
        done?.();
        return;
    }

    const force = !!done;

    transactionCategoriesStore.loadAllCategories({
        force: force
    }).then(() => {
        done?.();

        if (force) {
            showToast('Category list has been updated');
        }
    }).catch(error => {
        done?.();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function edit(category: TransactionCategory): void {
    props.f7router.navigate('/category/edit?id=' + category.id);
}

function hide(category: TransactionCategory, hidden: boolean): void {
    showLoading();

    transactionCategoriesStore.hideCategory({
        category: category,
        hidden: hidden
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function remove(category: TransactionCategory | null, confirm: boolean): void {
    if (!category) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        categoryToDelete.value = category;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    categoryToDelete.value = null;
    showLoading();

    transactionCategoriesStore.deleteCategory({
        category: category,
        beforeResolve: (done) => {
            onSwipeoutDeleted(getCategoryDomId(category), done);
        }
    }).then(() => {
        hideLoading();
    }).catch(error => {
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function setSortable(): void {
    if (sortable.value) {
        return;
    }

    searchText.value = '';
    scopeBookId.value = '';
    showHidden.value = true;
    sortable.value = true;
    displayOrderModified.value = false;
}

function saveSortResult(): void {
    if (!displayOrderModified.value) {
        showHidden.value = false;
        sortable.value = false;
        return;
    }

    displayOrderSaving.value = true;
    showLoading();

    transactionCategoriesStore.updateCategoryDisplayOrders({
        type: categoryType.value as CategoryType,
        parentId: primaryCategoryId.value
    }).then(() => {
        displayOrderSaving.value = false;
        hideLoading();

        showHidden.value = false;
        sortable.value = false;
        displayOrderModified.value = false;
    }).catch(error => {
        displayOrderSaving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function cancelSort(): void {
    if (!displayOrderModified.value) {
        showHidden.value = false;
        sortable.value = false;
        return;
    }

    displayOrderSaving.value = true;
    showLoading();

    transactionCategoriesStore.loadAllCategories({
        force: false
    }).then(() => {
        displayOrderSaving.value = false;
        hideLoading();

        showHidden.value = false;
        sortable.value = false;
        displayOrderModified.value = false;
    }).catch(error => {
        displayOrderSaving.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onSort(event: { el: { id: string }; from: number; to: number }): void {
    if (!event || !event.el || !event.el.id) {
        showToast('Unable to move category');
        return;
    }

    const id = parseCategoryIdFromDomId(event.el.id);

    if (!id) {
        showToast('Unable to move category');
        return;
    }

    transactionCategoriesStore.changeCategoryDisplayOrder({
        categoryId: id,
        from: event.from,
        to: event.to
    }).then(() => {
        displayOrderModified.value = true;
    }).catch(error => {
        showToast(error.message || error);
    });
}

function onPageAfterIn(): void {
    if (transactionCategoriesStore.transactionCategoryListStateInvalid && !loading.value) {
        reload();
    }

    routeBackOnError(props.f7router, loadingError);
}

init();
</script>

<style scoped>
.cy-category-scope-filter{display:flex;align-items:center;gap:9px;color:var(--cy-muted);font-size:12px;margin-bottom:12px}.cy-category-scope-filter select{min-width:0;flex:1;background:var(--cy-card);border:1px solid var(--cy-line);border-radius:8px;padding:7px 9px;color:var(--cy-ink)}
.cy-category-tools{padding:12px 16px 0}.cy-category-tools .cy-segments{justify-content:space-around;margin-bottom:14px}.cy-category-tools .cy-segments a{flex:1;text-align:center;padding:7px 10px;border-radius:8px;color:var(--cy-muted);font-size:13px}.cy-category-tools .cy-segments a.cy-type-active{background:var(--cy-accent);color:var(--cy-card)}.cy-parent-label{font-size:17px;font-weight:600;margin-bottom:12px!important}.cy-parent-label span{font-size:12px;color:var(--cy-muted);font-weight:400}.cy-management-search{display:flex;gap:8px;align-items:center;border:1px solid var(--cy-line);border-radius:11px;background:var(--cy-card);padding:0 12px}.cy-management-search .f7-icons{font-size:17px;color:var(--cy-muted)}.cy-management-search input{min-width:0;width:100%;height:42px;border:0;background:transparent;font-size:14px}.cy-management-actions{display:flex;justify-content:space-between;gap:8px;margin:12px 0}.cy-management-actions button,.cy-management-actions a{border:0;background:transparent;color:var(--cy-accent);font-size:12px;padding:5px 0}.cy-management-hint{font-size:11px;line-height:1.6;color:var(--cy-muted);margin-bottom:10px!important}.cy-category-edit{border:0;background:transparent;color:var(--cy-accent);padding:8px;min-height:36px}.cy-category-edit .f7-icons{font-size:17px}
</style>

<style>
.category-list {
    --f7-list-item-footer-font-size: var(--ebk-large-footer-font-size);
}

.category-list .item-footer {
    padding-top: 4px;
}
</style>
