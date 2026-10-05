<template>
    <details class="cy-book-scope-control">
        <summary>{{ books.scopeName }} <span>切换账本</span></summary>
        <fieldset :disabled="disabled"><legend class="visually-hidden">选择参与查询的账本</legend>
            <label><input type="checkbox" :checked="!books.selectedBookIds.length" @change="change([])" />全部账本</label>
            <label v-for="book in books.allBooks" :key="book.id"><input type="checkbox" :checked="books.selectedBookIds.includes(book.id)" @change="toggle(book.id, ($event.target as HTMLInputElement).checked)" />{{ book.name }}{{ book.archived ? '（已归档）' : '' }}</label>
        </fieldset>
        <p v-if="error" role="alert">{{ error }}<button @click="load">重试</button></p>
    </details>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useBooksStore } from '@/stores/books.ts';
import { investmentError } from '@/lib/investments.ts';
defineProps<{ disabled?: boolean }>();
const emit = defineEmits<{ change: [] }>();
const books = useBooksStore(), error = ref('');
function change(ids: string[]): void {
    books.selectedBookIds = ids;
    emit('change');
}
function toggle(id: string, checked: boolean): void { change(checked ? [...new Set([...books.selectedBookIds, id])] : books.selectedBookIds.filter(item => item !== id)); }
async function load(): Promise<void> { error.value = ''; try { await books.loadBooks(); } catch(cause) { error.value = investmentError(cause); } }
onMounted(load);
</script>
<style scoped>
.cy-book-scope-control{font-size:13px;padding:12px;border-bottom:1px solid var(--cy-line,rgba(127,127,127,.2));color:inherit}.cy-book-scope-control summary{display:flex;justify-content:space-between;gap:12px;cursor:pointer;list-style:none}.cy-book-scope-control summary span{opacity:.65;font-size:11px}.cy-book-scope-control fieldset{margin:10px 0 0;padding:0;border:0;display:flex;flex-direction:column;gap:12px;max-height:250px;overflow:auto}.cy-book-scope-control label{display:flex;gap:8px;align-items:center}.cy-book-scope-control input{accent-color:var(--cy-accent,#12786f);width:16px;height:16px}.cy-book-scope-control p{font-size:12px;color:#c23c48}.cy-book-scope-control p button{background:none;border:0;color:inherit;text-decoration:underline}.visually-hidden{position:absolute;width:1px;height:1px;overflow:hidden;clip-path:inset(50%)}
</style>
