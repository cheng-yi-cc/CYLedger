<template>
    <details class="cy-book-scope">
        <summary><span>{{ books.scopeName }}</span><span class="cy-muted">切换账本⌄</span></summary>
        <div class="cy-book-options">
            <label><input type="checkbox" :checked="!books.selectedBookIds.length" @change="books.selectedBookIds = []" />全部账本</label>
            <label v-for="book in books.allBooks" :key="book.id"><input type="checkbox" :checked="books.selectedBookIds.includes(book.id)" @change="toggle(book.id, ($event.target as HTMLInputElement).checked)" />{{ book.icon }} {{ book.name }}<small v-if="book.archived">已归档</small></label>
            <f7-link href="/books">管理账本</f7-link>
        </div>
        <p v-if="error" role="alert">{{ error }} <button @click="load">重试</button></p>
    </details>
</template>
<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { useBooksStore } from '@/stores/books.ts';
import { investmentError } from '@/lib/investments.ts';
const books = useBooksStore(), error = ref('');
function toggle(id: string, checked: boolean): void {
    books.selectedBookIds = checked ? [...books.selectedBookIds, id] : books.selectedBookIds.filter(value => value !== id);
}
async function load(): Promise<void> { error.value = ''; try { await books.loadBooks(); } catch(cause) { error.value = investmentError(cause); } }
onMounted(load);
</script>
<style scoped>
.cy-book-scope{border-bottom:1px solid var(--cy-line);margin-bottom:18px;padding-bottom:12px}.cy-book-scope summary{display:flex;justify-content:space-between;cursor:pointer;font-size:14px;list-style:none}.cy-book-scope summary span:last-child{font-size:12px}.cy-book-options{display:flex;flex-direction:column;gap:14px;padding:16px 2px 4px}.cy-book-options label{display:flex;gap:9px;align-items:center}.cy-book-options small{color:var(--cy-muted)}.cy-book-options input{accent-color:var(--cy-accent)}
</style>
