import Decimal from 'decimal.js';
import { AccountCategory } from '@/core/account.ts';

export type DebtOperation = 'borrow' | 'lend' | 'repay' | 'collect';
export interface DebtAccount { id: string; category: number; currency: string; balance: string }
export const DEBT_OPERATIONS: { value: DebtOperation; label: string }[] = [
    { value: 'borrow', label: '借入' }, { value: 'lend', label: '借出' },
    { value: 'repay', label: '还款' }, { value: 'collect', label: '收回' }
];
export function debtAccountCategory(operation: DebtOperation): number {
    return operation === 'borrow' || operation === 'repay' ? AccountCategory.DebtAccount.type : AccountCategory.Receivables.type;
}
export function debtIsSource(operation: DebtOperation): boolean { return operation === 'borrow' || operation === 'collect'; }
export function isDebtCashAccount(account: Pick<DebtAccount, 'category'>): boolean {
    return [AccountCategory.Cash.type, AccountCategory.CheckingAccount.type, AccountCategory.VirtualAccount.type, AccountCategory.SavingsAccount.type].includes(account.category);
}
export function inferDebtOperation(source?: Pick<DebtAccount, 'category'>, destination?: Pick<DebtAccount, 'category'>): DebtOperation | null {
    if (!source || !destination) return null;
    if (source.category === AccountCategory.DebtAccount.type && isDebtCashAccount(destination)) return 'borrow';
    if (source.category === AccountCategory.Receivables.type && isDebtCashAccount(destination)) return 'collect';
    if (destination.category === AccountCategory.DebtAccount.type && isDebtCashAccount(source)) return 'repay';
    if (destination.category === AccountCategory.Receivables.type && isDebtCashAccount(source)) return 'lend';
    return null;
}
export function debtPrincipal(account: Pick<DebtAccount, 'category' | 'balance'>): string {
    return new Decimal(account.balance).mul(account.category === AccountCategory.DebtAccount.type ? -1 : 1).toString();
}
export function debtImpact(operation: DebtOperation, cents: string): string {
    return new Decimal(cents).mul(operation === 'borrow' || operation === 'lend' ? 1 : -1).toString();
}
export function projectedDebtPrincipal(account: Pick<DebtAccount, 'category' | 'balance'>, operation: DebtOperation, cents: string, previousImpact = '0'): string {
    return new Decimal(debtPrincipal(account)).minus(previousImpact).plus(debtImpact(operation, cents)).toString();
}
export function validateDebtEntry(operation: DebtOperation, source: DebtAccount | undefined, destination: DebtAccount | undefined, cents: string, previousImpact = '0'): string {
    if (!source || !destination) return '请选择往来对象和资金账户。';
    if (inferDebtOperation(source, destination) !== operation) return '请选择正确的债务／应收对象和资金账户。';
    if (source.currency !== destination.currency) return '往来对象与资金账户需使用相同币种。';
    if (!/^\d{1,15}$/.test(cents) || new Decimal(cents).lte(0)) return '请输入大于零的本金金额。';
    const account = debtIsSource(operation) ? source : destination;
    if (new Decimal(projectedDebtPrincipal(account, operation, cents, previousImpact)).lt(0)) return '本次金额会使已偿还本金超过借款本金，请核对。';
    return '';
}
