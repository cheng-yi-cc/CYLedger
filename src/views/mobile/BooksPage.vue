<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="load">
        <f7-navbar title="账本管理" back-link="返回" />
        <main class="cy-page-body">
            <p class="cy-muted">用账本区分日常、旅行等记录。各账本共用资金账户，切换账本不改变余额。</p>
            <p v-if="error" class="cy-message" role="alert">{{ error }}</p>
            <form class="cy-panel cy-new-book" @submit.prevent="create"><input v-model="name" maxlength="64" required placeholder="新账本名称" aria-label="新账本名称" /><button class="cy-button cy-primary" :disabled="busy">新建账本</button></form>
            <section v-for="book in books.allBooks" :key="book.id" class="cy-panel">
                <template v-if="editing?.id !== book.id"><div class="cy-section-head"><h2>{{ book.icon }} {{ book.name }}</h2><button class="cy-button" :disabled="busy" @click="editing = {...book}">编辑</button></div><p class="cy-muted">{{ book.archived ? '已归档 · 保留所有历史流水' : book.isDefault ? '默认账本' : '使用中' }}</p></template>
                <form v-else @submit.prevent="save">
                    <label class="cy-book-field">名称<input v-model="editing.name" required maxlength="64" /></label>
                    <label class="cy-book-field">图标<select v-model="editing.icon"><option v-for="icon in icons" :key="icon">{{ icon }}</option></select></label>
                    <label class="cy-book-field">排序<input v-model.number="editing.displayOrder" type="number" min="0" max="10000" required /></label>
                    <label class="cy-book-check"><input v-model="editing.isDefault" type="checkbox" :disabled="book.isDefault || editing.archived" />设为默认账本</label>
                    <label class="cy-book-check"><input v-model="editing.showTransfers" type="checkbox" />显示转账流水</label>
                    <label class="cy-book-check"><input v-model="editing.showInvestments" type="checkbox" />显示投资结算流水</label>
                    <label class="cy-book-check"><input v-model="editing.archived" type="checkbox" :disabled="editing.isDefault" />归档账本</label>
                    <p class="cy-muted">流水显示设置只影响列表，收支和账户余额的计算方式不变。归档后仍可查询历史。</p>
                    <div class="cy-actions"><button class="cy-button cy-primary" :disabled="busy">保存</button><button type="button" class="cy-button" :disabled="busy" @click="editing = null">取消</button></div>
                </form>
            </section>
        </main>
    </f7-page>
</template>
<script setup lang="ts">
import { ref } from 'vue';
import { useBooksStore } from '@/stores/books.ts';
import type { LedgerBook } from '@/models/book.ts';
import { investmentError } from '@/lib/investments.ts';
const books = useBooksStore(), name = ref(''), error = ref(''), busy = ref(false), editing = ref<LedgerBook | null>(null);
const icons = ['📒','🏠','✈️','🍃','🎓','💼','🎁','🚗'];
async function load(): Promise<void> { try { await books.loadBooks(true); } catch(cause) { error.value = investmentError(cause); } }
async function create(): Promise<void> { if (busy.value || !name.value.trim()) return; busy.value = true; error.value = ''; try { await books.createBook(name.value.trim()); name.value = ''; } catch(cause) { error.value = investmentError(cause); } finally { busy.value = false; } }
async function save(): Promise<void> { if (busy.value || !editing.value) return; busy.value = true; error.value = ''; try { await books.modifyBook({...editing.value, name: editing.value.name.trim()}); editing.value = null; } catch(cause) { error.value = investmentError(cause); } finally { busy.value = false; } }
</script>
<style scoped>
.cy-new-book{display:flex;gap:12px}.cy-new-book input{flex:1;min-width:0}.cy-book-field{display:flex;align-items:center;gap:14px;margin:12px 0}.cy-book-field input,.cy-book-field select{flex:1;min-width:0;padding:10px;border:1px solid var(--cy-line);border-radius:8px;background:var(--cy-card);color:inherit}.cy-book-check{display:flex;gap:10px;align-items:center;margin:16px 0}.cy-book-check input{accent-color:var(--cy-accent)}
</style>
