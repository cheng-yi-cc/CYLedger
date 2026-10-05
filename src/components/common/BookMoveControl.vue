<template>
    <details class="cy-book-move" @toggle="onToggle">
        <summary>批量移动账本 <span>{{ eligible.length }} 笔可选</span></summary>
        <p class="cy-book-move-hint">从当前已加载的流水中选择。转账两端会一起移动，金额和账户余额保持不变。</p>
        <p v-if="error" class="cy-book-move-error" role="alert">{{ error }}</p>
        <p v-if="message" role="status">{{ message }}</p>
        <form v-if="eligible.length" @submit.prevent="reviewing ? move() : review()">
            <fieldset :disabled="disabled || moving">
                <template v-if="!reviewing">
                    <label class="cy-book-move-target">目标账本
                        <select v-model="targetBookId" aria-label="批量移动的目标账本">
                            <option value="" disabled>请选择账本</option>
                            <option v-for="book in books.activeBooks" :key="book.id" :value="book.id">{{ book.name }}</option>
                        </select>
                    </label>
                    <div class="cy-book-move-actions">
                        <button type="button" @click="selectedIds = eligible.slice(0, 1000).map(item => item.id)">{{ eligible.length > 1000 ? '选择前 1000 笔' : '全选已加载流水' }}</button>
                        <button type="button" @click="selectedIds = []">清空</button>
                    </div>
                    <div class="cy-book-move-list">
                        <label v-for="item in eligible" :key="item.id">
                            <input v-model="selectedIds" type="checkbox" :value="item.id" :disabled="selectedIds.length >= 1000 && !selectedIds.includes(item.id)" />
                            <span>{{ item.category?.name || transactionName(item.type) }}<small>{{ transactionDate(item) }} · {{ item.sourceAccount?.name || '资金账户' }}{{ item.comment ? ` · ${item.comment}` : '' }}</small></span>
                        </label>
                    </div>
                    <button class="cy-book-move-submit" type="submit" :disabled="!selectedIds.length || !targetBookId">移动已选 {{ selectedIds.length }} 笔</button>
                </template>
                <template v-else>
                    <p>确认将所选 {{ selectedIds.length }} 笔流水移动到“{{ targetBookName }}”？</p>
                    <div class="cy-book-move-actions">
                        <button class="cy-book-move-submit" type="submit">{{ moving ? '正在移动…' : '确认移动' }}</button>
                        <button type="button" @click="reviewing = false">返回选择</button>
                    </div>
                </template>
            </fieldset>
        </form>
        <p v-else class="cy-book-move-hint">暂无可移动流水。投资结算请在对应投资记录中修改账本。</p>
    </details>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import type { Transaction } from '@/models/transaction.ts';
import { useBooksStore } from '@/stores/books.ts';
import { investmentError } from '@/lib/investments.ts';
import { parseDateTimeFromUnixTimeWithTimezoneOffset } from '@/lib/datetime.ts';

const props = defineProps<{ transactions: Transaction[]; disabled?: boolean }>();
const emit = defineEmits<{ moved: [] }>();
const books = useBooksStore();
const eligible = computed(() => props.transactions.filter(item => item.editable && !item.investmentEventId));
const selectedIds = ref<string[]>([]), targetBookId = ref(''), reviewing = ref(false), moving = ref(false);
const error = ref(''), message = ref('');
const targetBookName = computed(() => books.activeBooks.find(book => book.id === targetBookId.value)?.name || '');
watch(() => eligible.value.map(item => item.id).join(','), () => {
    selectedIds.value = selectedIds.value.filter(id => eligible.value.some(item => item.id === id));
    reviewing.value = false;
});
watch(() => books.selectedBookIds.join(','), () => { selectedIds.value = []; reviewing.value = false; message.value = ''; });

function transactionName(type: number): string { return ({ 1: '余额调整', 2: '收入', 3: '支出', 4: '转账' }[type] || '流水'); }
function transactionDate(item: Transaction): string { return parseDateTimeFromUnixTimeWithTimezoneOffset(item.time, item.utcOffset).getGregorianCalendarYearDashMonthDashDay(); }
async function onToggle(event: Event): Promise<void> {
    if (!(event.currentTarget as HTMLDetailsElement).open) return;
    error.value = '';
    try { await books.loadBooks(); if (!targetBookId.value || !targetBookName.value) targetBookId.value = books.defaultBookId; }
    catch (cause) { error.value = investmentError(cause); }
}
function review(): void {
    if (props.disabled || moving.value || !selectedIds.value.length || !targetBookName.value) return;
    error.value = ''; message.value = ''; reviewing.value = true;
}
async function move(): Promise<void> {
    if (props.disabled || moving.value || !selectedIds.value.length || !targetBookName.value) return;
    const ids = [...selectedIds.value], name = targetBookName.value;
    moving.value = true; error.value = '';
    try {
        await books.moveTransactions(ids, targetBookId.value);
        message.value = `已将 ${ids.length} 笔流水移动到“${name}”。`;
        selectedIds.value = []; reviewing.value = false; emit('moved');
    } catch (cause) { error.value = investmentError(cause); }
    finally { moving.value = false; }
}
</script>

<style scoped>
.cy-book-move{font-size:13px;padding:12px;border-bottom:1px solid var(--cy-line,rgba(127,127,127,.2));color:inherit}.cy-book-move summary{display:flex;justify-content:space-between;gap:12px;cursor:pointer;list-style:none}.cy-book-move summary span,.cy-book-move-hint{font-size:11px;opacity:.7}.cy-book-move p{margin:12px 0;line-height:1.6}.cy-book-move fieldset{border:0;padding:0;margin:0;min-width:0}.cy-book-move-target{display:flex;flex-direction:column;gap:7px}.cy-book-move select{max-width:100%;border:1px solid var(--cy-line,#aaa);border-radius:6px;padding:7px;background:var(--cy-card,transparent);color:inherit}.cy-book-move-actions{display:flex;gap:14px;flex-wrap:wrap;margin:12px 0}.cy-book-move button{background:none;border:0;color:inherit;padding:6px 0;cursor:pointer}.cy-book-move button:disabled{opacity:.5;cursor:default}.cy-book-move-list{max-height:260px;overflow:auto;display:flex;flex-direction:column;gap:13px;margin:14px 0}.cy-book-move-list label{display:flex;gap:8px;align-items:flex-start}.cy-book-move-list input{width:16px;height:16px;flex-shrink:0;margin-top:2px;accent-color:var(--cy-accent,#12786f)}.cy-book-move-list small{display:block;font-size:11px;opacity:.7;line-height:1.5;overflow-wrap:anywhere}.cy-book-move .cy-book-move-submit{border:1px solid var(--cy-accent,#12786f);border-radius:6px;padding:8px 12px}.cy-book-move-error{color:var(--cy-expense,#b33)}
</style>
