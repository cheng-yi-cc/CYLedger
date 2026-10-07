<template>
    <f7-page class="cy-mobile-surface cy-asset-surface" @page:afterin="onPageAfterIn">
        <f7-navbar :title="editCategoryId ? '编辑分类' : '新增分类'" back-link="返回">
            <f7-nav-right><f7-link icon-f7="checkmark_alt" aria-label="保存分类" :class="{ disabled: loading || inputIsEmpty || submitting }" @click="save" /></f7-nav-right>
        </f7-navbar>
        <main class="management-body">
            <p v-if="loading" class="cy-empty">正在加载分类…</p>
            <form v-else class="management-form" @submit.prevent="save">
                <label class="management-field"><span>分类名称</span><input v-model="category.name" :disabled="submitting" maxlength="64" placeholder="填写分类名称" aria-label="分类名称" /></label>
                <button type="button" class="category-field-button" :disabled="submitting" @click="showIconSelectionSheet = true"><span>分类图标</span><ItemIcon :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" :color="category.color" /><f7-icon f7="chevron_right" /></button>
                <button type="button" class="category-field-button" :disabled="submitting" @click="showColorSelectionSheet = true"><span>图标颜色</span><ItemIcon icon-type="fixed-f7" icon-id="circle_fill" :color="category.color" /><f7-icon f7="chevron_right" /></button>
                <button v-if="category.parentId && category.parentId !== '0'" type="button" class="category-field-button" :disabled="submitting || !editCategoryId" @click="showPrimaryCategorySheet = true"><span>一级分类</span><small>{{ getPrimaryCategoryName(category.parentId) }}</small><f7-icon v-if="editCategoryId" f7="chevron_right" /></button>
                <div v-if="editCategoryId" class="category-field-button"><span>显示分类</span><f7-toggle :disabled="submitting" :checked="category.visible" @toggle:change="category.visible = $event" /></div>
                <button type="button" class="category-field-button" :disabled="submitting" @click="showBookSelection = true"><span>生效账本</span><small>{{ !category.bookIds.length ? '全部账本' : category.bookIds.length === 1 ? booksStore.allBooks.find(book => book.id === category.bookIds[0])?.name || '已选 1 个账本' : `已选 ${category.bookIds.length} 个账本` }}</small><f7-icon f7="chevron_right" /></button>
                <label class="category-note"><span>备注</span><textarea v-model="category.comment" :disabled="submitting" rows="3" maxlength="255" placeholder="填写备注（选填）" aria-label="分类备注" /></label>
            </form>
        </main>
        <icon-selection-sheet :all-system-icon-infos="ALL_CATEGORY_ICONS" :color="category.color" v-model:show="showIconSelectionSheet" v-model:icon-type="category.iconType" v-model="category.icon" />
        <color-selection-sheet :all-system-color-infos="ALL_CATEGORY_COLORS" v-model:show="showColorSelectionSheet" v-model="category.color" />
        <list-item-selection-sheet value-type="item" key-field="id" value-field="id" title-field="name" icon-field="icon" icon-type="category" color-field="color" :items="allAvailableCategories" v-model:show="showPrimaryCategorySheet" v-model="category.parentId" />
        <StatisticsSheet v-model:open="showBookSelection" title="生效账本">
            <template #action><button @click="showBookSelection = false">确定</button></template>
            <p class="management-caption">控制记账时可以选择此分类的账本。二级分类同时遵循一级分类的可见范围。</p>
            <label class="category-book"><input type="checkbox" :checked="!category.bookIds.length" @change="setGlobalScope(($event.target as HTMLInputElement).checked)" />全部账本</label>
            <label v-for="book in booksStore.allBooks" :key="book.id" class="category-book"><input type="checkbox" :checked="category.bookIds.includes(book.id)" @change="toggleBook(book.id, ($event.target as HTMLInputElement).checked)" />{{ book.name }}{{ book.archived ? '（已归档）' : '' }}</label>
            <p v-if="scopeError" class="cy-message" role="alert">{{ scopeError }}</p>
        </StatisticsSheet>
    </f7-page>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import StatisticsSheet from '@/components/mobile/StatisticsSheet.vue';
import type { Router } from 'framework7/types';

import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useCategoryEditPageBase } from '@/views/base/categories/CategoryEditPageBase.ts';

import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useBooksStore } from '@/stores/books.ts';

import type { ColorValue } from '@/core/color.ts';
import { CategoryType } from '@/core/category.ts';
import { ALL_CATEGORY_ICONS } from '@/consts/icon.ts';
import { ALL_CATEGORY_COLORS } from '@/consts/color.ts';
import { TransactionCategory } from '@/models/transaction_category.ts';

import { getCategoryIconType } from '@/lib/icon.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const query = props.f7route.query;

const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editCategoryId,
    clientSessionId,
    loading,
    submitting,
    category,
    allAvailableCategories,
    inputEmptyProblemMessage,
    inputIsEmpty
} = useCategoryEditPageBase(query['type'] ? parseInt(query['type']) as CategoryType : undefined, query['parentId']);

const transactionCategoriesStore = useTransactionCategoriesStore();
const booksStore = useBooksStore();
const scopeError = ref('');
function setGlobalScope(global: boolean): void { category.value.bookIds = global ? [] : booksStore.defaultBookId ? [booksStore.defaultBookId] : []; }
function toggleBook(id: string, checked: boolean): void {
    category.value.bookIds = checked ? [...new Set([...category.value.bookIds, id])] : category.value.bookIds.filter(bookId => bookId !== id);
}
void booksStore.loadBooks().catch(() => { scopeError.value = '账本列表加载失败，请返回后重试。'; });

const loadingError = ref<unknown | null>(null);
const showPrimaryCategorySheet = ref<boolean>(false);
const showBookSelection = ref(false);
const showIconSelectionSheet = ref<boolean>(false);
const showColorSelectionSheet = ref<boolean>(false);

function getPrimaryCategoryName(parentId: string): string | null {
    return TransactionCategory.findNameById(allAvailableCategories.value, parentId);
}

function init(): void {
    if (!query['id'] && !query['parentId']) {
        showToast('Parameter Invalid');
        loadingError.value = 'Parameter Invalid';
        return;
    }

    if (query['id']) {
        loading.value = true;

        editCategoryId.value = query['id'];
        transactionCategoriesStore.getCategory({
            categoryId: editCategoryId.value
        }).then(response => {
            category.value.fillFrom(response);
            loading.value = false;
        }).catch(error => {
            if (error.processed) {
                loading.value = false;
            } else {
                loadingError.value = error;
                showToast(error.message || error);
            }
        });
    } else if (query['parentId']) {
        const categoryType = query['type'] ? parseInt(query['type']) as CategoryType : undefined;

        if (categoryType !== CategoryType.Income &&
            categoryType !== CategoryType.Expense &&
            categoryType !== CategoryType.Transfer) {
            showToast('Parameter Invalid');
            loadingError.value = 'Parameter Invalid';
            return;
        }

        if (query['color']) {
            category.value.color = query['color'] as ColorValue;
        }

        if (query['icon']) {
            category.value.icon = query['icon'];
        }

        clientSessionId.value = generateRandomUUID();
        loading.value = false;
    }
}

function save(): void {
    if (submitting.value || loading.value) return;
    category.value.name = category.value.name.trim();
    const router = props.f7router;
    const problemMessage = inputEmptyProblemMessage.value;

    if (problemMessage) {
        showAlert(problemMessage);
        return;
    }

    submitting.value = true;
    showLoading(() => submitting.value);

    transactionCategoriesStore.saveCategory({
        category: category.value,
        isEdit: !!editCategoryId.value,
        clientSessionId: clientSessionId.value
    }).then(() => {
        submitting.value = false;
        hideLoading();

        if (!editCategoryId.value) {
            showToast('You have added a new category');
        } else {
            showToast('You have saved this category');
        }

        router.back();
    }).catch(error => {
        submitting.value = false;
        hideLoading();

        if (!error.processed) {
            showToast(error.message || error);
        }
    });
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
}

init();
</script>
<style scoped src="@/styles/mobile/management.css"></style>
<style scoped>
.category-field-button{display:flex;align-items:center;gap:12px;min-height:64px;width:100%;padding:12px 0;background:none;border:0;border-bottom:1px solid var(--cy-line);font-size:15px;text-align:left}
.category-field-button>span{flex:1}.category-field-button small{font-size:13px;color:var(--cy-muted);max-width:55%;text-align:right;overflow-wrap:anywhere}.category-field-button :deep(.icon){font-size:25px}.category-field-button>.icon:last-child{font-size:12px;color:var(--cy-muted)}
.category-note{display:block;padding:20px 0 8px;font-size:15px}.category-note textarea{box-sizing:border-box;display:block;resize:vertical;width:100%;border:0;background:none;color:var(--cy-ink);font:inherit;line-height:1.8;margin-top:12px;padding:0}.category-note textarea::placeholder{color:var(--cy-muted);font-size:14px}
.category-book{display:flex;align-items:center;gap:12px;min-height:54px;border-bottom:1px solid var(--cy-line);font-size:15px}.category-book input{width:18px;height:18px;accent-color:var(--cy-accent)}
</style>
