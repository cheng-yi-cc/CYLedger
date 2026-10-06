<template>
    <div ref="dialogElement" class="dialog stat-month-dialog cy-mobile-surface" role="dialog" aria-modal="true" aria-label="选择年月">
        <div ref="pickerElement" class="stat-month-picker" />
        <div class="stat-month-actions">
            <button class="dialog-button" @click="close">取消</button>
            <button class="dialog-button stat-month-confirm" @click="confirm">确定</button>
        </div>
    </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, useTemplateRef, watch } from 'vue';
import { f7 } from 'framework7-vue';
import type { Dialog, Picker } from 'framework7/types';

const props = defineProps<{ open: boolean; modelValue: string }>();
const emit = defineEmits<{ 'update:open': [value: boolean]; 'update:modelValue': [value: string] }>();
const dialogElement = useTemplateRef<HTMLElement>('dialogElement');
const pickerElement = useTemplateRef<HTMLElement>('pickerElement');
let dialog: Dialog.Dialog | undefined;
let picker: Picker.Picker | undefined;

function destroyPicker(): void {
    picker?.destroy(); picker = undefined;
    pickerElement.value?.replaceChildren();
}
function close(): void { dialog?.close(); }
function confirm(): void {
    const values = picker?.getValue();
    if (Array.isArray(values) && values.length === 2) {
        emit('update:modelValue', `${values[0]}-${values[1]}`);
        close();
    }
}

watch(() => props.open, open => {
    if (!open) { close(); return; }
    if (!dialogElement.value || !pickerElement.value) return;
    dialog ??= f7.dialog.create({
        el: dialogElement.value,
        closeByBackdropClick: true,
        on: {
            open() {
                picker = f7.picker.create({
                    containerEl: pickerElement.value!, toolbar: false, rotateEffect: true,
                    value: props.modelValue.split('-'),
                    cols: [
                        { values: Array.from({ length: 231 }, (_, i) => String(1970 + i)), textAlign: 'center', cssClass: 'stat-year-column' },
                        { divider: true, content: '年' },
                        { values: Array.from({ length: 12 }, (_, i) => String(i + 1).padStart(2, '0')), displayValues: Array.from({ length: 12 }, (_, i) => String(i + 1)), textAlign: 'center', cssClass: 'stat-month-column' },
                        { divider: true, content: '月' }
                    ]
                });
            },
            closed() {
                destroyPicker();
                emit('update:open', false);
            }
        }
    });
    dialog.open();
}, { flush: 'post' });

onBeforeUnmount(() => { destroyPicker(); dialog?.destroy(); });
</script>

<style scoped>
.stat-month-dialog{width:min(360px,calc(100% - 64px));max-width:none;margin-left:0;transform:translate(-50%,-50%);border-radius:8px;background:var(--cy-card);color:var(--cy-ink);padding:24px 18px 10px;box-sizing:border-box;--f7-picker-inline-height:190px;--f7-picker-item-height:36px;--f7-picker-column-font-size:24px;--f7-picker-item-text-color:var(--cy-muted);--f7-picker-item-selected-text-color:var(--cy-ink);--f7-picker-divider-text-color:var(--cy-muted)}
.stat-month-dialog.modal-in{transform:translate(-50%,-50%)}
.stat-month-picker :deep(.picker-columns){gap:8px;--f7-picker-mask-bg-color:var(--cy-card)}
.stat-month-picker :deep(.picker-center-highlight){display:none}
.stat-month-picker :deep(.stat-year-column){width:108px}
.stat-month-picker :deep(.stat-month-column){width:66px}
.stat-month-picker :deep(.picker-column-divider){font-size:18px}
.stat-month-actions{display:flex;justify-content:space-around;margin-top:25px}
.stat-month-actions button{border:0;background:none;font:inherit;font-size:17px;color:var(--cy-muted);height:48px;width:45%;border-radius:4px}
.stat-month-actions .stat-month-confirm{color:var(--cy-accent)}
</style>
