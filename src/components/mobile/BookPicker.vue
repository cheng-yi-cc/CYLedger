<template>
    <label class="cy-book-picker" :class="{ 'cy-book-picker-compact': compact }"><span v-if="!compact">账本</span>
        <select :value="modelValue" aria-label="本笔记账所属账本" :disabled="disabled" @change="$emit('update:modelValue', ($event.target as HTMLSelectElement).value)">
            <option disabled value="">选择账本</option>
            <option v-for="book in available" :key="book.id" :value="book.id">{{ book.icon }} {{ book.name }}{{ book.archived ? '（已归档）' : '' }}</option>
        </select>
        <small v-if="error" role="alert">{{ error }}</small>
    </label>
</template>
<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import { useBooksStore } from '@/stores/books.ts';
import { investmentError } from '@/lib/investments.ts';
const props = defineProps<{ modelValue: string; disabled?: boolean; compact?: boolean }>();
defineEmits<{ 'update:modelValue': [value: string] }>();
const books = useBooksStore(), error = ref('');
const available = computed(() => books.allBooks.filter(book => !book.archived || book.id === props.modelValue));
onMounted(() => { void books.loadBooks().catch(cause => { error.value = investmentError(cause); }); });
</script>
<style scoped>
.cy-book-picker{display:flex;align-items:center;gap:8px;font-size:13px;flex-wrap:wrap}.cy-book-picker select{background:var(--cy-card,var(--f7-list-bg-color));color:inherit;border:1px solid var(--cy-line,#ddd);border-radius:8px;padding:7px 9px;max-width:200px}.cy-book-picker small{color:var(--cy-expense,#b33)}
.cy-book-picker-compact{min-width:0;max-width:96px}.cy-book-picker-compact select{width:100%;max-width:96px;min-height:36px;padding:6px;font-size:12px;text-overflow:ellipsis;border:0;background:var(--cy-soft)}
</style>
