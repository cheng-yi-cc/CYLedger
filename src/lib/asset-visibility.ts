import {ref} from 'vue';
import {ledgerMoney} from '@/lib/ledger-display.ts';

// 同一会话内所有资产阅读页共用；表单中的可编辑事实不做遮挡。
export const assetAmountsVisible = ref(true);
export function assetMoney(value: string | null | undefined, currencySymbol = false): string {
    return assetAmountsVisible.value ? ledgerMoney(value, currencySymbol) : '••••';
}
export function assetText(value: string | number | null | undefined): string {
    return assetAmountsVisible.value ? String(value ?? '—') : '••••';
}
