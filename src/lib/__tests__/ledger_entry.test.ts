import { describe, expect, it } from 'vitest';
import { calculateEntryAmount } from '../ledger-entry.ts';
import { inferDebtOperation, debtPrincipal, projectedDebtPrincipal, validateDebtEntry } from '../ledger-debt.ts';

const cash = { id: 'cash', category: 1, currency: 'CNY', balance: '100000' };
const debt = { id: 'debt', category: 5, currency: 'CNY', balance: '-10000' };
const receivable = { id: 'receivable', category: 6, currency: 'CNY', balance: '10000' };

describe('entry calculator', () => {
    it('calculates decimals, negative refunds and rounded division without floating point', () => {
        expect(calculateEntryAmount('0.1', '+', '0.2')).toBe('0.3');
        expect(calculateEntryAmount('0', '−', '12.34')).toBe('-12.34');
        expect(calculateEntryAmount('12.34', '×', '3')).toBe('37.02');
        expect(calculateEntryAmount('1', '÷', '6')).toBe('0.17');
    });
    it('rejects division by zero and amounts beyond the legacy integer cents limit', () => {
        expect(() => calculateEntryAmount('10', '÷', '0')).toThrow('除数不能为零');
        expect(() => calculateEntryAmount('9999999999999.99', '+', '0.01')).toThrow('超出允许范围');
    });
});
describe('debt principal', () => {
    it('recognizes all four cash/debt directions and excludes ordinary transfers', () => {
        expect(inferDebtOperation(debt, cash)).toBe('borrow');
        expect(inferDebtOperation(cash, debt)).toBe('repay');
        expect(inferDebtOperation(cash, receivable)).toBe('lend');
        expect(inferDebtOperation(receivable, cash)).toBe('collect');
        expect(inferDebtOperation(cash, cash)).toBeNull();
    });
    it('reduces outstanding principal with each installment and reverses the original impact during editing', () => {
        expect(debtPrincipal(debt)).toBe('10000');
        expect(projectedDebtPrincipal(debt, 'repay', '3000')).toBe('7000');
        expect(projectedDebtPrincipal({ ...debt, balance: '-7000' }, 'repay', '2000')).toBe('5000');
        expect(projectedDebtPrincipal({ ...debt, balance: '-7000' }, 'repay', '4000', '-3000')).toBe('6000');
        expect(projectedDebtPrincipal(receivable, 'collect', '10000')).toBe('0');
    });
    it('rejects overpayment, mismatched currencies, invalid roles and non-positive principal', () => {
        expect(validateDebtEntry('repay', cash, debt, '10000')).toBe('');
        expect(validateDebtEntry('repay', cash, debt, '10001')).not.toBe('');
        expect(validateDebtEntry('borrow', debt, { ...cash, currency: 'USD' }, '100')).not.toBe('');
        expect(validateDebtEntry('lend', debt, cash, '100')).not.toBe('');
        expect(validateDebtEntry('borrow', debt, cash, '0')).not.toBe('');
        expect(validateDebtEntry('borrow', debt, cash, '-1')).not.toBe('');
        expect(validateDebtEntry('borrow', debt, cash, '1.5')).not.toBe('');
    });
});
