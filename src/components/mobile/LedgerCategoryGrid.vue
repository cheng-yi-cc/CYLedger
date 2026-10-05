<template>
    <div class="cy-category-grid" role="group" aria-label="记账分类">
        <template v-for="(row, index) in categoryRows" :key="index">
            <div class="cy-category-row">
                <template v-for="category in row" :key="category?.id || 'editor'">
                    <div v-if="category" class="cy-category-tile">
                        <button class="cy-category-choice" :aria-pressed="selectedParent?.id === category.id" :disabled="disabled" @click="selectParent(category)">
                            <span class="cy-category-circle"><item-icon :icon-type="category.iconType === IconType.UserCustom ? 'user-custom' : 'category'" :icon-id="category.icon" /></span>
                            <span>{{ category.name }}</span>
                            <small v-if="selectedParent?.id === category.id && selectedChild?.name !== category.name">{{ selectedChild?.name }}</small>
                        </button>
                        <button v-if="children(category).length > 1" class="cy-category-children" :aria-label="`展开${category.name}子分类`" :aria-expanded="openParent?.id === category.id" :aria-controls="`category-children-${category.id}`" :disabled="disabled" @click="toggleParent(category)"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiDotsHorizontal" /></svg></button>
                    </div>
                    <button v-else class="cy-category-choice" :disabled="disabled" @click="newParentId = '0'; showEditor = true">
                        <span class="cy-category-circle"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiPlus" /></svg></span><span>编辑</span>
                    </button>
                </template>
            </div>
            <div v-if="openParent && row.some(category => category?.id === openParent?.id)" :id="`category-children-${openParent.id}`" class="cy-subcategory-grid" :aria-label="`${openParent.name}子分类`">
                <button v-for="child in expandedChildren" :key="child.id" class="cy-category-choice cy-subcategory-choice" :aria-pressed="modelValue === child.id" :disabled="disabled" @click="choose(child)">
                    <span class="cy-subcategory-icon"><item-icon :icon-type="child.iconType === IconType.UserCustom ? 'user-custom' : 'category'" :icon-id="child.icon" /></span><span>{{ child.name }}</span>
                </button>
                <button class="cy-category-choice cy-subcategory-choice" :disabled="disabled" @click="newParentId = openParent.id; showEditor = true"><span class="cy-subcategory-icon"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiPlus" /></svg></span><span>新增</span></button>
            </div>
        </template>
    </div>
    <f7-sheet class="cy-mobile-surface cy-category-sheet" :opened="showEditor" @sheet:closed="showEditor = false" swipe-to-close>
        <f7-toolbar><div class="left">编辑分类</div><div class="right"><f7-link sheet-close>完成</f7-link></div></f7-toolbar>
        <form class="cy-category-form" @submit.prevent="createCategory">
            <label>分类名称<input v-model="newName" maxlength="64" placeholder="例如：宠物" aria-label="新分类名称" required /></label>
            <label>所属分类<select v-model="newParentId" aria-label="新分类所属类别"><option value="0">新建一级分类</option><option v-for="category in visibleCategories" :key="category.id" :value="category.id">{{ category.name }}</option></select></label>
            <p v-if="error" role="alert">{{ error }}</p>
            <button class="cy-button cy-primary" :disabled="busy || !newName.trim()">创建并选用</button>
            <f7-link :href="`/category/list?type=${categoryType}`" sheet-close>管理全部分类</f7-link>
        </form>
    </f7-sheet>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { mdiDotsHorizontal, mdiPlus } from '@mdi/js';
import { IconType } from '@/core/icon.ts';
import type { CategoryType } from '@/core/category.ts';
import { TransactionCategory } from '@/models/transaction_category.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { generateRandomUUID } from '@/lib/misc.ts';
import { investmentError } from '@/lib/investments.ts';

const props = defineProps<{ categories: TransactionCategory[]; bookId: string; categoryType: CategoryType; preserveCategory: boolean; disabled: boolean }>();
const modelValue = defineModel<string>({ required: true });
const store = useTransactionCategoriesStore();
const openParentId = ref('');
const showEditor = ref(false), busy = ref(false), newName = ref(''), newParentId = ref('0'), error = ref('');
function available(category: TransactionCategory): boolean { return !category.hidden && (!category.bookIds.length || category.bookIds.includes(props.bookId)); }
function children(parent: TransactionCategory): TransactionCategory[] {
    return (parent.subCategories || []).filter(child => (available(parent) && available(child)) || (props.preserveCategory && child.id === modelValue.value));
}
const visibleCategories = computed(() => props.categories.filter(category => available(category) || (props.preserveCategory && category.subCategories?.some(child => child.id === modelValue.value))));
const categoryRows = computed(() => {
    const tiles: (TransactionCategory | null)[] = [...visibleCategories.value, null];
    return Array.from({ length: Math.ceil(tiles.length / 5) }, (_, index) => tiles.slice(index * 5, index * 5 + 5));
});
const openParent = computed(() => visibleCategories.value.find(category => category.id === openParentId.value));
const expandedChildren = computed(() => openParent.value ? children(openParent.value).filter(child => child.name !== openParent.value?.name) : []);
const selectedParent = computed(() => visibleCategories.value.find(category => category.subCategories?.some(child => child.id === modelValue.value)));
const selectedChild = computed(() => selectedParent.value?.subCategories?.find(child => child.id === modelValue.value));
watch(() => [props.categoryType, props.categories, props.bookId, modelValue.value], () => {
    if (!visibleCategories.value.some(parent => children(parent).some(child => child.id === modelValue.value))) {
        modelValue.value = visibleCategories.value.flatMap(children)[0]?.id || '';
    }
}, { immediate: true });
watch(() => [props.categoryType, props.bookId], () => { openParentId.value = ''; });
function choose(child: TransactionCategory): void { modelValue.value = child.id; }
function selectParent(parent: TransactionCategory): void {
    const items = children(parent);
    const general = items.find(child => child.name === parent.name) || (items.length === 1 ? items[0] : undefined);
    if (general || items[0]) choose((general || items[0])!);
    openParentId.value = items.length > 1 || !general ? parent.id : '';
}
function toggleParent(parent: TransactionCategory): void {
    if (openParentId.value === parent.id) openParentId.value = '';
    else if (selectedParent.value?.id === parent.id) openParentId.value = parent.id;
    else selectParent(parent);
}
async function createCategory(): Promise<void> {
    if (busy.value || !newName.value.trim()) return;
    busy.value = true; error.value = '';
    try {
        const parent = visibleCategories.value.find(item => item.id === newParentId.value);
        const category = TransactionCategory.createNewCategory(props.categoryType, parent?.id || '0');
        category.name = newName.value.trim();
        category.icon = parent?.icon || '1000'; category.iconType = parent?.iconType || IconType.System;
        category.color = parent?.color || '12786f'; category.bookIds = parent ? [...parent.bookIds] : [];
        let id: string;
        if (parent) {
            id = (await store.saveCategory({ category, isEdit: false, clientSessionId: generateRandomUUID() })).id;
        } else {
            const result = await store.addCategories({ categories: [{ name: category.name, type: props.categoryType, icon: category.icon,
                iconType: category.iconType, color: category.color, bookIds: [], subCategories: [category.toCreateRequest(generateRandomUUID())] }] });
            id = result[props.categoryType]?.find(item => item.name === category.name)?.subCategories?.[0]?.id || '';
        }
        await store.loadAllCategories({ force: true }).catch(cause => { if (!cause?.isUpToDate) throw cause; });
        if (id) modelValue.value = id;
        newName.value = ''; showEditor.value = false;
    } catch (cause) { error.value = investmentError(cause); }
    finally { busy.value = false; }
}
</script>

<style scoped>
.cy-category-grid{display:flex;flex-direction:column;gap:18px;padding:17px 9px;overflow:auto;min-height:0;height:100%;box-sizing:border-box}.cy-category-row,.cy-subcategory-grid{display:grid;grid-template-columns:repeat(5,minmax(0,1fr));gap:18px 3px;flex:none}.cy-subcategory-grid{padding:0 0 3px}.cy-subcategory-choice{min-height:64px!important}.cy-subcategory-icon{display:grid;place-items:center;width:36px;height:33px;color:var(--cy-muted)}.cy-subcategory-icon :deep(.icon){font-size:25px;color:inherit!important}.cy-subcategory-icon svg{width:25px;height:25px;fill:currentColor}.cy-subcategory-choice[aria-pressed=true] .cy-subcategory-icon{color:var(--cy-accent)}
.cy-category-tile{position:relative;min-width:0}.cy-category-choice{display:flex;flex-direction:column;align-items:center;gap:7px;width:100%;padding:0 1px;min-height:74px;border:0;background:transparent;color:var(--cy-ink);font-size:12px;line-height:1.4;text-align:center;border-radius:10px}.cy-category-choice>span:last-of-type{max-width:100%;overflow-wrap:anywhere}
.cy-category-circle{display:grid;place-items:center;width:42px;height:42px;border-radius:50%;background:var(--cy-soft);color:var(--cy-muted)}.cy-category-circle :deep(.icon){font-size:26px;color:inherit!important}.cy-category-circle svg{width:27px;height:27px;fill:currentColor}
.cy-category-choice[aria-pressed=true]{color:var(--cy-accent)}.cy-category-choice[aria-pressed=true] .cy-category-circle{background:var(--cy-accent);color:var(--cy-card)}.cy-category-choice small{font-size:10px;max-width:100%;overflow-wrap:anywhere}
.cy-category-children{position:absolute;top:28px;right:calc(50% - 27px);display:grid;place-items:center;padding:0;width:23px!important;height:23px;border-radius:50%;background:var(--cy-soft);color:var(--cy-muted);border:2px solid var(--cy-card)}.cy-category-children svg{width:17px;height:17px;fill:currentColor}
.cy-category-sheet{height:auto;max-height:80dvh;overflow:auto}.cy-category-sheet :deep(.sheet-modal-inner){padding-top:var(--f7-toolbar-height);box-sizing:border-box}.cy-category-sheet .toolbar{padding-inline:16px}
.cy-category-form{display:flex;flex-direction:column;gap:16px;padding:20px 18px 30px}.cy-category-form label{display:flex;align-items:center;gap:16px;font-size:14px}.cy-category-form input,.cy-category-form select{flex:1;min-width:0;padding:10px;border:1px solid var(--cy-line);border-radius:8px;background:var(--cy-card)}.cy-category-form p{color:var(--cy-expense);font-size:12px}.cy-category-form a{align-self:center}
@media(max-width:350px){.cy-category-grid{gap:14px 2px;padding-inline:5px}.cy-category-circle{width:38px;height:38px}.cy-category-choice{font-size:11px}}
</style>
