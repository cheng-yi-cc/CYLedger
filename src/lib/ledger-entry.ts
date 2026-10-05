import Decimal from 'decimal.js';
import { TRANSACTION_MAX_AMOUNT, TRANSACTION_MIN_AMOUNT } from '@/consts/transaction.ts';

const Money = Decimal.clone({ precision: 40 });
export type EntryOperation = '+' | '−' | '×' | '÷';

export function calculateEntryAmount(left: string, operation: EntryOperation, right: string): string {
    const a = new Money(left), b = new Money(right);
    if (operation === '÷' && b.isZero()) throw new Error('除数不能为零。');
    const value = (operation === '+' ? a.plus(b) : operation === '−' ? a.minus(b) : operation === '×' ? a.mul(b) : a.div(b)).toDecimalPlaces(2, Decimal.ROUND_HALF_UP);
    if (!value.isFinite() || value.mul(100).lt(TRANSACTION_MIN_AMOUNT) || value.mul(100).gt(TRANSACTION_MAX_AMOUNT)) throw new Error('计算结果超出允许范围。');
    return value.toString();
}
