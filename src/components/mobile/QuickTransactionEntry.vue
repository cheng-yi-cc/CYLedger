<template>
    <section class="cy-quick-entry" aria-label="快速记账">
        <div class="cy-entry-selection"><slot name="selection"><LedgerCategoryGrid :categories="categories" :category-type="categoryType" :book-id="bookId" :preserve-category="preserveCategory" :disabled="disabled" v-model="categoryId" /></slot></div>
        <div class="cy-entry-details">
            <div class="cy-note-amount-row">
                <button class="cy-tag-button" :disabled="disabled" @click="$emit('tags')"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiTagOutline" /></svg><span>{{ tags.length ? `${tags.length} 个标签` : '标签' }}</span></button>
                <input v-model="comment" :maxlength="TRANSACTION_MAX_COMMENT_LENGTH" aria-label="备注" placeholder="添加备注" />
                <output :class="amountClass" aria-label="记账金额" aria-live="polite"><small>{{ activeCurrency === 'CNY' ? '¥' : activeCurrency }}</small>{{ input || '0.00' }}</output>
            </div>
            <div v-if="tags.length" class="cy-selected-tags"><button v-for="tag in tags" :key="tag.id" @click="$emit('tags')">{{ tag.name }}</button></div>
            <slot name="attachments" />
            <div v-if="transfer" class="cy-amount-tabs"><button :aria-pressed="!destinationActive" @click="selectAmount(false)">转出 · {{ currency }}</button><button :aria-pressed="destinationActive" @click="selectAmount(true)">转入 · {{ destinationCurrency }}</button></div>
            <div class="cy-entry-meta"><slot name="context" /><button class="cy-meta-more" aria-label="更多记账信息" @click="openMore"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiDotsHorizontal" /></svg></button></div>
            <p v-if="operand !== null" class="cy-calculation">{{ operand }} {{ operation }} {{ input || '…' }}</p>
            <p v-if="amountError || validationMessage" class="cy-amount-error" role="alert">{{ amountError || validationMessage }}</p>
        </div>
        <div class="cy-entry-keyboard" aria-label="金额键盘">
            <button v-for="key in ['1','2','3']" :key="key" :disabled="disabled" @click="press(key)">{{ key }}</button>
            <button :disabled="disabled" aria-label="删除一位" @click="press('⌫')"><svg viewBox="0 0 24 24" aria-hidden="true"><path :d="mdiBackspaceOutline" /></svg></button>
            <button v-for="key in ['4','5','6']" :key="key" :disabled="disabled" @click="press(key)">{{ key }}</button>
            <div class="cy-key-operations"><button v-for="key in ['+','−','×','÷']" :key="key" :disabled="disabled" :aria-label="({'+':'加','−':'减','×':'乘','÷':'除'})[key]" @click="press(key)">{{ key }}</button></div>
            <button v-for="key in ['7','8','9']" :key="key" :disabled="disabled" @click="press(key)">{{ key }}</button>
            <button class="cy-key-next" :disabled="disabled || !canSave || !!validationMessage" @click="allowContinue ? saveAndContinue() : $emit('cancel')">{{ allowContinue ? '再记' : '取消' }}</button>
            <button :disabled="disabled" @click="press('0')">0</button>
            <button :disabled="disabled" aria-label="小数点" @click="press('.')">.</button>
            <button class="cy-key-save" :disabled="disabled || (!operation && (!canSave || !!validationMessage))" @click="complete">{{ operation ? '=' : '保存' }}</button>
        </div>
    </section>
</template>

<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import Decimal from 'decimal.js';
import { mdiTagOutline, mdiDotsHorizontal, mdiBackspaceOutline } from '@mdi/js';
import LedgerCategoryGrid from './LedgerCategoryGrid.vue';
import type { CategoryType } from '@/core/category.ts';
import type { TransactionCategory } from '@/models/transaction_category.ts';
import { calculateEntryAmount, type EntryOperation } from '@/lib/ledger-entry.ts';
import { TRANSACTION_MAX_AMOUNT, TRANSACTION_MIN_AMOUNT, TRANSACTION_MAX_COMMENT_LENGTH } from '@/consts/transaction.ts';

const props = defineProps<{
    categories: TransactionCategory[]; bookId: string; preserveCategory: boolean; categoryType: CategoryType;
    currency: string; destinationCurrency: string; transfer: boolean; canSave: boolean; disabled: boolean; allowContinue: boolean;
    tags: { id: string; name: string }[]; amountClass?: string; validationMessage?: string;
}>();
const amount = defineModel<number>('amount', { required: true });
const destinationAmount = defineModel<number>('destinationAmount', { required: true });
const categoryId = defineModel<string>('categoryId', { required: true });
const comment = defineModel<string>('comment', { required: true });
const emit = defineEmits<{ (e: 'save'): void; (e: 'continue'): void; (e: 'more'): void; (e: 'tags'): void; (e: 'cancel'): void }>();
const Money = Decimal.clone({ precision: 40 });
const destinationActive = ref(false), input = ref(''), operand = ref<string | null>(null), operation = ref<EntryOperation | ''>(''), amountError = ref('');
const activeCurrency = computed(() => destinationActive.value ? props.destinationCurrency : props.currency);
const activeAmount = computed({ get: () => destinationActive.value ? destinationAmount.value : amount.value, set: (value: number) => { if (destinationActive.value) destinationAmount.value = value; else amount.value = value; } });
watch(() => props.transfer, transfer => { if (!transfer) destinationActive.value = false; });
watch([activeAmount, destinationActive], () => {
    if (operation.value) return;
    const value = new Money(activeAmount.value).div(100);
    if (new Money(input.value && input.value !== '-' ? input.value : '0').eq(value)) return;
    input.value = value.isZero() ? '' : value.toString();
}, { immediate: true });

function accept(value: string): boolean {
    const decimal = new Money(value && value !== '-' ? value : '0');
    const cents = decimal.mul(100);
    if (cents.lt(TRANSACTION_MIN_AMOUNT) || cents.gt(TRANSACTION_MAX_AMOUNT)) { amountError.value = '金额超出允许范围。'; return false; }
    amountError.value = ''; input.value = value;
    if (!operation.value) activeAmount.value = cents.toNumber();
    return true;
}
function calculate(): boolean {
    if (!operation.value || operand.value === null) return true;
    try {
        const result = calculateEntryAmount(operand.value, operation.value, input.value && input.value !== '-' ? input.value : '0');
        operation.value = ''; operand.value = null;
        return accept(result);
    } catch (cause) { amountError.value = (cause as Error).message; return false; }
}
function press(key: string): void {
    if (['+', '−', '×', '÷'].includes(key)) {
        if (operation.value && !input.value) { operation.value = key as EntryOperation; return; }
        if (!calculate()) return;
        operand.value = input.value && input.value !== '-' ? input.value : '0'; operation.value = key as EntryOperation; input.value = ''; return;
    }
    if (key === '⌫') { accept(input.value.slice(0, -1)); return; }
    if (key === '.') { if (!input.value.includes('.')) accept(`${input.value === '-' ? '-0' : input.value || '0'}.`); return; }
    if (input.value.includes('.') && input.value.split('.')[1]!.length >= 2) return;
    accept((input.value === '0' ? '' : input.value) + key);
}
function complete(): void { if (operation.value) calculate(); else emit('save'); }
function saveAndContinue(): void { if (calculate()) emit('continue'); }
function selectAmount(destination: boolean): void {
    if (!calculate()) return;
    destinationActive.value = destination;
    input.value = new Money(activeAmount.value).div(100).toString();
}
function openMore(): void { if (calculate()) emit('more'); }
defineExpose({ prepareSave: calculate });
</script>

<style scoped>
.cy-quick-entry{display:flex;flex-direction:column;height:calc(100dvh - var(--f7-navbar-height) - var(--f7-safe-area-top,0px));min-height:470px;max-width:720px;margin:auto;padding:8px 6px max(10px,env(safe-area-inset-bottom));gap:6px;box-sizing:border-box;color:var(--cy-ink)}
.cy-entry-selection{flex:1;min-height:110px;border-radius:13px;background:var(--cy-card);overflow:auto;border:1px solid var(--cy-line)}
.cy-entry-details{flex:none;background:var(--cy-card);border:1px solid var(--cy-line);border-radius:13px;padding:10px 12px;display:flex;flex-direction:column;gap:8px}
.cy-note-amount-row{display:flex;align-items:center;gap:10px;min-height:37px}.cy-tag-button{display:flex;align-items:center;gap:5px;border:0;padding:6px 0;background:transparent;color:var(--cy-muted);white-space:nowrap;font-size:13px}.cy-tag-button svg{width:19px;height:19px;fill:currentColor;flex:none}
.cy-note-amount-row>input{flex:1;min-width:35px;width:60px;border:0;background:transparent;padding:6px 0;font-size:13px;color:var(--cy-ink)}.cy-note-amount-row>input::placeholder{color:var(--cy-muted)}
.cy-note-amount-row output{max-width:58%;min-width:0;text-align:right;font-size:clamp(22px,6vw,29px);font-weight:600;line-height:1.25;overflow-wrap:anywhere;font-variant-numeric:tabular-nums}.cy-note-amount-row output small{font-size:12px;font-weight:400;padding-right:3px;color:var(--cy-muted)}
.cy-note-amount-row output.cy-expense{color:var(--cy-expense)}.cy-note-amount-row output.cy-income{color:var(--cy-accent)}.cy-note-amount-row output.cy-expense small,.cy-note-amount-row output.cy-income small{color:inherit}
.cy-selected-tags{display:flex;gap:5px;overflow:auto}.cy-selected-tags button{border:0;flex:none;border-radius:6px;padding:3px 8px;background:var(--cy-soft);color:var(--cy-accent);font-size:11px}
.cy-entry-meta{display:flex;flex-wrap:wrap;align-items:center;gap:8px;min-height:30px}.cy-meta-more{margin-left:auto;border:0;padding:4px;background:transparent;color:var(--cy-muted);line-height:1}.cy-meta-more svg{width:20px;height:20px;fill:currentColor}
.cy-amount-tabs{display:flex;gap:8px}.cy-amount-tabs button{background:transparent;border:0;border-radius:6px;color:var(--cy-muted);padding:5px 8px;font-size:11px}.cy-amount-tabs button[aria-pressed=true]{background:var(--cy-soft);color:var(--cy-accent)}
.cy-calculation{font-size:12px;color:var(--cy-muted);text-align:right}.cy-amount-error{font-size:12px;color:var(--cy-expense);line-height:1.5}
.cy-entry-keyboard{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));grid-template-rows:repeat(4,48px);gap:5px;flex:none}.cy-entry-keyboard>button,.cy-key-operations{border:1px solid var(--cy-line);border-radius:10px;background:var(--cy-card);color:var(--cy-ink);font-size:24px;padding:0;touch-action:manipulation}.cy-entry-keyboard>button svg{width:27px;height:27px;fill:currentColor;vertical-align:middle}
.cy-key-operations{grid-column:4;grid-row:2 / 4;display:grid;grid-template-columns:1fr 1fr;overflow:hidden}.cy-key-operations button{border:0;padding:0;background:transparent;color:var(--cy-ink);font-size:22px;touch-action:manipulation}.cy-entry-keyboard button:active{background:var(--cy-soft)}.cy-entry-keyboard .cy-key-save{background:var(--cy-accent);color:var(--cy-card);font-size:16px;border-color:var(--cy-accent)}.cy-entry-keyboard .cy-key-next{font-size:16px}
.cy-quick-entry button:disabled{opacity:.45}.cy-quick-entry button:focus-visible{outline:2px solid var(--cy-accent);outline-offset:-2px}
@media(min-height:850px){.cy-entry-keyboard{grid-template-rows:repeat(4,53px)}}@media(max-height:600px){.cy-quick-entry{height:560px}}@media(max-width:350px){.cy-entry-details{padding-inline:8px}.cy-note-amount-row{gap:7px}.cy-tag-button{font-size:12px}}
</style>
