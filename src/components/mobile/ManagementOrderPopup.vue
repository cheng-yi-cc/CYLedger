<template>
    <f7-popup class="cy-mobile-surface cy-asset-surface" :opened="open" :close-by-backdrop-click="!busy" :close-on-escape="!busy" @popup:closed="emit('update:open', false)">
        <f7-page>
            <f7-navbar :title="title">
                <f7-nav-left><f7-link :class="{ disabled: busy }" @click="emit('update:open', false)">取消</f7-link></f7-nav-left>
                <f7-nav-right><f7-link :class="{ disabled: busy }" @click="emit('save', draft.map(item => item.id))">保存</f7-link></f7-nav-right>
            </f7-navbar>
            <main class="cy-page-body">
                <p class="order-hint">拖动右侧把手调整顺序，也可点击箭头移动。</p>
                <div v-if="allowNameSort" class="order-tools"><button :disabled="busy" @click="sortNames(false)">名称升序</button><button :disabled="busy" @click="sortNames(true)">名称降序</button></div>
                <p v-if="error" class="cy-message" role="alert">{{ error }}</p>
                <draggable v-model="draft" item-key="id" handle=".order-handle" :disabled="busy" :animation="150">
                    <template #item="{ element, index }">
                        <div class="order-row">
                            <span class="order-name">{{ element.name }}<small v-if="element.hidden">已隐藏</small></span>
                            <button :aria-label="`上移${element.name}`" :disabled="busy || index === 0" @click="move(index, index - 1)"><f7-icon f7="arrow_up" /></button>
                            <button :aria-label="`下移${element.name}`" :disabled="busy || index === draft.length - 1" @click="move(index, index + 1)"><f7-icon f7="arrow_down" /></button>
                            <span class="order-handle" :aria-label="`拖动${element.name}排序`"><f7-icon f7="line_horizontal_3" /></span>
                        </div>
                    </template>
                </draggable>
            </main>
        </f7-page>
    </f7-popup>
</template>
<script setup lang="ts">
import { ref, watch } from 'vue';
import draggable from 'vuedraggable';
const props = defineProps<{ open: boolean; title: string; items: { id: string; name: string; hidden?: boolean }[]; busy: boolean; error: string; allowNameSort?: boolean }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; save: [ids: string[]] }>();
const draft = ref<{ id: string; name: string; hidden?: boolean }[]>([]);
watch(() => props.open, open => { if (open) draft.value = props.items.map(item => ({ ...item })); });
function sortNames(descending: boolean) { draft.value.sort((a, b) => a.name.localeCompare(b.name, 'zh-CN', { numeric: true }) * (descending ? -1 : 1)); }
function move(from: number, to: number) {
    const item = draft.value[from];
    if (!item || to < 0 || to >= draft.value.length || props.busy) return;
    draft.value.splice(from, 1);
    draft.value.splice(to, 0, item);
}
</script>
<style scoped>
.order-hint{font-size:12px;color:var(--cy-muted);line-height:1.7;margin:4px 0 18px}
.order-tools{display:flex;gap:18px;margin-bottom:14px}.order-tools button{border:0;background:none;color:var(--cy-accent);font-size:12px;padding:7px 0}
.order-row{display:flex;align-items:center;gap:2px;padding:8px 10px 8px 16px;background:var(--cy-card);border-radius:12px;margin-bottom:10px;min-height:48px}
.order-name{flex:1;min-width:0;overflow-wrap:anywhere;font-size:15px}.order-name small{display:block;color:var(--cy-muted);font-size:11px;margin-top:3px}
.order-row button,.order-handle{display:grid;place-items:center;width:40px;height:44px;border:0;background:transparent;color:var(--cy-muted);flex-shrink:0}
.order-row .icon{font-size:18px}.order-handle{touch-action:none;cursor:grab}
</style>
