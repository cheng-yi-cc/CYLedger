<template>
    <f7-page class="cy-mobile-surface" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left :class="{ 'disabled': loading }" :back-link="tt('Back')"></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'disabled': loading }">
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <f7-list strong inset dividers class="margin-top-half skeleton-text" v-if="loading">
            <f7-list-input label="Category Name" placeholder="Your category name"></f7-list-input>
            <f7-list-item class="list-item-with-header-and-title" header="Primary Category" title="Primary Category"></f7-list-item>
            <f7-list-item class="list-item-with-header-and-title list-item-with-multi-item">
                <template #default>
                    <div class="grid grid-cols-2">
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>Category Icon</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <f7-icon f7="app_fill"></f7-icon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>
                        </div>
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>Category Color</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <f7-icon f7="app_fill"></f7-icon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>
                        </div>
                    </div>
                </template>
            </f7-list-item>
            <f7-list-item class="list-item-toggle" header="Visible" after="True"></f7-list-item>
            <f7-list-input label="Description" type="textarea" placeholder="Your category description (optional)"></f7-list-input>
        </f7-list>

        <f7-list form strong inset dividers class="margin-top-half" v-else-if="!loading">
            <f7-list-input
                type="text"
                clear-button
                :label="tt('Category Name')"
                :placeholder="tt('Your category name')"
                v-model:value="category.name"
            ></f7-list-input>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :header="tt('Primary Category')"
                :title="getPrimaryCategoryName(category.parentId)"
                @click="showPrimaryCategorySheet = true"
                v-if="editCategoryId && category.parentId && category.parentId !== '0'"
            >
                <list-item-selection-sheet value-type="item"
                                           key-field="id" value-field="id" title-field="name"
                                           icon-field="icon" icon-type="category" color-field="color"
                                           :items="allAvailableCategories"
                                           v-model:show="showPrimaryCategorySheet"
                                           v-model="category.parentId">
                </list-item-selection-sheet>
            </f7-list-item>

            <f7-list-item class="list-item-with-header-and-title list-item-with-multi-item">
                <template #default>
                    <div class="grid grid-cols-2">
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#" @click="showIconSelectionSheet = true">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>{{ tt('Category Icon') }}</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <ItemIcon :icon-type="getCategoryIconType(category.iconType)" :icon-id="category.icon" :color="category.color"></ItemIcon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>

                            <icon-selection-sheet :all-system-icon-infos="ALL_CATEGORY_ICONS"
                                                  :color="category.color"
                                                  v-model:show="showIconSelectionSheet"
                                                  v-model:icon-type="category.iconType"
                                                  v-model="category.icon"
                            ></icon-selection-sheet>
                        </div>
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#" @click="showColorSelectionSheet = true">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>{{ tt('Category Color') }}</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <ItemIcon icon-type="fixed-f7" icon-id="app_fill" :color="category.color"></ItemIcon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>

                            <color-selection-sheet :all-system-color-infos="ALL_CATEGORY_COLORS"
                                                   v-model:show="showColorSelectionSheet"
                                                   v-model="category.color"
                            ></color-selection-sheet>
                        </div>
                    </div>
                </template>
            </f7-list-item>

            <f7-list-item :title="tt('Visible')" v-if="editCategoryId">
                <template #after>
                    <f7-toggle :checked="category.visible" @toggle:change="category.visible = $event"></f7-toggle>
                </template>
            </f7-list-item>

            <f7-list-item title="可见账本" class="cy-category-scope-item">
                <template #footer>
                    <label class="cy-category-scope-option"><input type="checkbox" :checked="!category.bookIds.length" @change="setGlobalScope(($event.target as HTMLInputElement).checked)" />所有账本通用</label>
                    <div class="cy-category-scope-options"><label v-for="book in booksStore.allBooks" :key="book.id" class="cy-category-scope-option"><input type="checkbox" :checked="category.bookIds.includes(book.id)" @change="toggleBook(book.id, ($event.target as HTMLInputElement).checked)" />{{ book.name }}{{ book.archived ? '（已归档）' : '' }}</label></div>
                    <p>只影响记账时的分类选择，已记录账单和历史统计保留。二级分类还需在一级分类的可见范围内。</p>
                    <p v-if="scopeError" role="alert">{{ scopeError }}</p>
                </template>
            </f7-list-item>

            <f7-list-input
                type="textarea"
                style="height: auto"
                :label="tt('Description')"
                :placeholder="tt('Your category description (optional)')"
                v-textarea-auto-size
                v-model:value="category.comment"
            ></f7-list-input>
        </f7-list>
    </f7-page>
</template>

<script setup lang="ts">
import { ref } from 'vue';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
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

const { tt } = useI18n();
const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();
const {
    editCategoryId,
    clientSessionId,
    loading,
    submitting,
    category,
    allAvailableCategories,
    title,
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

<style scoped>
.cy-category-scope-option{display:flex;align-items:center;gap:8px;padding:9px 0;font-size:13px;color:var(--cy-ink)}.cy-category-scope-option input{width:17px;height:17px;accent-color:var(--cy-accent)}.cy-category-scope-options{display:flex;flex-wrap:wrap;column-gap:18px}.cy-category-scope-item p{font-size:11px;line-height:1.7;color:var(--cy-muted);margin:7px 0}.cy-category-scope-item p[role=alert]{color:var(--cy-expense)}
</style>
