import { computed, ref, watch } from 'vue';
import { defineStore } from 'pinia';
import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import type { LedgerBook } from '@/models/book.ts';
import { useUserStore } from '@/stores/user.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';

async function request<T>(method: 'get' | 'post', path: string, data?: unknown): Promise<T> {
    const response = await axios.request<ApiResponse<T>>({ method, url: `v1/books/${path}`, data });
    if (!response.data.success) throw new Error('账本操作失败，请重试。');
    return response.data.result;
}

export const useBooksStore = defineStore('books', () => {
    const users = useUserStore();
    const allBooks = ref<LedgerBook[]>([]);
    const selectedBookIds = ref<string[]>([]);
    const loaded = ref(false);
    let pending: Promise<void> | null = null;
    let generation = 0;
    let reportVersion = 0;
    const activeBooks = computed(() => allBooks.value.filter(book => !book.archived));
    const defaultBookId = computed(() => {
        const selected = selectedBookIds.value.length === 1 ? activeBooks.value.find(book => book.id === selectedBookIds.value[0]) : undefined;
        return selected?.id || activeBooks.value.find(book => book.isDefault)?.id || activeBooks.value[0]?.id || '';
    });
    const scopeName = computed(() => !selectedBookIds.value.length ? '全部账本' : selectedBookIds.value.length === 1
        ? allBooks.value.find(book => book.id === selectedBookIds.value[0])?.name || '账本' : `${selectedBookIds.value.length} 个账本`);
    function invalidateBookReports(): void {
        reportVersion++;
        useTransactionsStore().updateStoreInvalidState({ transactionList: true, statistics: true, overview: true, explorer: true });
    }
    function captureReportScope(): () => boolean {
        const version = reportVersion;
        return () => version === reportVersion;
    }
    watch(() => [...selectedBookIds.value].sort().join(','), invalidateBookReports, { flush: 'sync' });
    watch(() => users.currentUserBasicInfo?.username, () => {
        generation++; allBooks.value = []; selectedBookIds.value = []; loaded.value = false; pending = null;
        invalidateBookReports();
    });
    async function loadBooks(force = false): Promise<void> {
        if (pending) return pending;
        if (loaded.value && !force) return;
        const version = generation;
        pending = request<LedgerBook[]>('get', 'list').then(items => {
            if (version !== generation) return;
            allBooks.value = items.slice().sort((a, b) => a.displayOrder - b.displayOrder);
            selectedBookIds.value = selectedBookIds.value.filter(id => items.some(book => book.id === id));
            loaded.value = true;
        }).finally(() => { if (version === generation) pending = null; });
        return pending;
    }
    async function createBook(name: string): Promise<void> {
        await request('post', 'create', { name }); await loadBooks(true);
    }
    async function modifyBook(book: LedgerBook): Promise<void> {
        await request('post', 'modify', book); await loadBooks(true);
    }
    async function moveTransactions(transactionIds: string[], bookId: string): Promise<void> {
        await request('post', 'move', { transactionIds, bookId });
        invalidateBookReports();
    }
    return { allBooks, activeBooks, selectedBookIds, loaded, defaultBookId, scopeName, captureReportScope, loadBooks, createBook, modifyBook, moveTransactions };
});
