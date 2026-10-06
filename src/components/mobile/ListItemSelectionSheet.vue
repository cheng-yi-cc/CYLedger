<template>
    <f7-sheet swipe-to-close :swipe-handler="enableFilter ? '.flat-list-header' : '.swipe-handler'"
              :class="[heightClass, { 'list-item-selection-searchable-sheet': enableFilter }]" :opened="show"
              @sheet:open="onSheetOpen" @sheet:closed="onSheetClosed">
        <div v-if="enableFilter" class="flat-list-header"><strong>{{title}}</strong><button type="button" :aria-label="tt('Close')" @click="close">{{tt('Close')}}</button></div>
        <f7-toolbar v-else class="toolbar-with-swipe-handler">
            <div class="swipe-handler"></div>
            <div class="left">
                <f7-link sheet-close icon-f7="xmark" :aria-label="tt('Close')"></f7-link>
            </div>
        </f7-toolbar>
        <label v-if="enableFilter" class="flat-list-search"><f7-icon f7="search" /><input v-model="filterContent" type="search" :placeholder="filterPlaceholder" :aria-label="filterPlaceholder" /></label>
        <f7-page-content class="margin-top">
            <f7-list dividers class="no-margin-vertical">
                <f7-list-item link="#" no-chevron
                              :title="ti((titleField ? (item as Record<string, unknown>)[titleField] : item) as string, !!titleI18n)"
                              :value="getItemValue(item, index, valueField, valueType)"
                              :after="ti((afterField ? (item as Record<string, unknown>)[afterField] : '') as string, !!afterI18n)"
                              :footer="footerField ? (item as Record<string, unknown>)[footerField] as string : undefined"
                              :class="{ 'list-item-selected': isSelected(item, index) }"
                              :key="getItemValue(item, index, keyField, valueType)"
                              v-for="({item, index}) in filteredItems"
                              v-show="item && (!hiddenField || !(item as Record<string, unknown>)[hiddenField])"
                              @click="onItemClicked(item, index)">
                    <template #content-start>
                        <f7-icon class="list-item-checked-icon" f7="checkmark_alt" :style="{ 'color': isSelected(item, index) ? '' : 'transparent' }"></f7-icon>
                    </template>
                    <template #media v-if="iconField || imageField">
                        <img v-if="imageField && (item as Record<string, unknown>)[imageField]" class="flat-list-image" :src="(item as Record<string, unknown>)[imageField] as string" alt="" />
                        <ItemIcon v-else-if="iconField" :icon-type="getIconType(iconType, iconTypeField ? (item as Record<string, unknown>)[iconTypeField] : undefined)"
                                  :icon-id="(item as Record<string, unknown>)[iconField]"
                                  :color="colorField ? (item as Record<string, unknown>)[colorField] : undefined"></ItemIcon>
                    </template>
                </f7-list-item>
                <f7-list-item v-if="enableFilter && !filteredItems.length" :title="filterNoItemsText" />
            </f7-list>
        </f7-page-content>
    </f7-sheet>
</template>

<script setup lang="ts">
import { ref, computed, nextTick } from 'vue';

import { useI18n } from '@/locales/helpers.ts';

import { getIconType } from '@/lib/icon.ts';
import { scrollToSelectedItem } from '@/lib/ui/common.ts';
import { type Framework7Dom } from '@/lib/ui/mobile.ts';

const props = defineProps<{
    modelValue: unknown;
    valueType: 'item' | 'index'; // item or index
    keyField?: string; // for value type == item
    valueField?: string; // for value type == item
    titleField: string;
    title?: string;
    titleI18n?: boolean;
    afterField?: string;
    afterI18n?: boolean;
    footerField?: string;
    imageField?: string;
    enableFilter?: boolean;
    filterPlaceholder?: string;
    filterNoItemsText?: string;
    iconType?: string;
    iconTypeField?: string;
    iconField?: string;
    colorField?: string;
    hiddenField?: string;
    items: unknown[];
    show: boolean;
}>();

const emit = defineEmits<{
    (e: 'update:modelValue', value: unknown): void;
    (e: 'update:show', value: boolean): void;
}>();

const { tt, ti } = useI18n();

const currentValue = ref<unknown>(props.modelValue);
const filterContent = ref('');
const filteredItems = computed(() => {
    const query = filterContent.value.trim().toLocaleLowerCase();
    return props.items.map((item,index)=>({item,index})).filter(({item}) => {
        if (!props.enableFilter || !query) return true;
        const record = item as Record<string, unknown>;
        return [props.titleField,props.footerField,props.afterField].some(field=>field && String(record[field] ?? '').toLocaleLowerCase().includes(query));
    });
});

const heightClass = computed<string>(() => {
    if (props.items.length > 10) {
        return 'list-item-selection-huge-sheet';
    } else if (props.items.length > 6) {
        return 'list-item-selection-large-sheet';
    } else {
        return 'list-item-selection-default-sheet';
    }
});

function isSelected(item: unknown, index: number): boolean {
    if (props.valueType === 'index') {
        return currentValue.value === index;
    } else {
        if (props.valueField) {
            return currentValue.value === (item as Record<string, unknown>)[props.valueField];
        } else {
            return currentValue.value === item;
        }
    }
}

function getItemValue(item: unknown, index: number, fieldName: string | undefined, valueType: 'item' | 'index'): unknown {
    if (valueType === 'index') {
        return index;
    } else if (fieldName) {
        return (item as Record<string, unknown>)[fieldName];
    } else {
        return item;
    }
}

function close(): void {
    emit('update:show', false);
}

function onItemClicked(item: unknown, index: number): void {
    if (props.valueType === 'index') {
        currentValue.value = index;
    } else {
        if (props.valueField) {
            currentValue.value = (item as Record<string, unknown>)[props.valueField];
        } else {
            currentValue.value = item;
        }
    }

    emit('update:modelValue', currentValue.value);
    close();
}

async function onSheetOpen(event: { $el: Framework7Dom }): Promise<void> {
    currentValue.value = props.modelValue;
    filterContent.value = '';
    await nextTick();
    scrollToSelectedItem(event.$el[0], '.sheet-modal-inner', '.page-content', 'li.list-item-selected');
}

function onSheetClosed(): void {
    filterContent.value = '';
    close();
}
</script>

<style>
.flat-list-search{display:flex;align-items:center;gap:10px;margin:8px 16px 0;padding:10px 12px;background:var(--cy-bg,#f5f7f8);border-radius:10px;color:var(--cy-muted,#75817d)}
.flat-list-search .icon{font-size:18px}.flat-list-search input{width:100%;min-width:0;border:0;outline:none;background:transparent;color:var(--cy-ink,#25312e);font:inherit;font-size:15px}
.flat-list-image{width:28px;height:28px;object-fit:contain}
.list-item-selection-searchable-sheet{height:min(560px,82dvh)!important;background:var(--cy-card,#fff)!important;color:var(--cy-ink,#25312e);--f7-sheet-bg-color:var(--cy-card,#fff);--f7-list-bg-color:var(--cy-card,#fff);border-radius:18px 18px 0 0;backdrop-filter:none!important}
.list-item-selection-searchable-sheet .sheet-modal-inner{display:flex;flex-direction:column;background:var(--cy-card,#fff);border-radius:inherit}
.flat-list-header{box-sizing:border-box;width:100%;flex:none;margin:0;padding:16px 18px 10px;display:flex;justify-content:space-between;align-items:center;gap:16px}
.flat-list-header strong{flex:none;white-space:nowrap;font-size:16px;font-weight:500}.flat-list-header button{flex:none;width:auto;margin-left:auto;border:0;background:none;color:var(--cy-accent,#12786f);font:inherit;font-size:14px;padding:4px}
.list-item-selection-searchable-sheet .flat-list-search{flex:none;margin-top:0}
.list-item-selection-searchable-sheet .page-content{flex:1;min-height:0;padding-top:0!important;padding-bottom:env(safe-area-inset-bottom);margin-top:8px!important}
.list-item-selection-default-sheet {
    height: 310px;
}

@media (min-height: 630px) {
    .list-item-selection-large-sheet {
        height: 370px;
    }

    .list-item-selection-huge-sheet {
        height: 500px;
    }
}

@media (max-height: 629px) {
    .list-item-selection-large-sheet,
    .list-item-selection-huge-sheet {
        height: 320px;
    }
}
</style>
