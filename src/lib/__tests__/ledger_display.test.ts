import { describe, expect, it } from 'vitest';
import { ledgerMoney, ledgerSignedAmount, ledgerTotals } from '../ledger-display.ts';

describe('mobile ledger financial display', () => {
    it('excludes transfers and opening balances, nets expense refunds without floating point loss', () => {
        expect(ledgerTotals([{type:1,cny:'20000'},{type:4,cny:'601'},{type:2,cny:'0.1'},{type:2,cny:'0.2'},{type:3,cny:'12.34'},{type:3,cny:'-2.34'}])).toEqual({income:'0.3',expense:'10',balance:'-9.7',complete:true});
    });
    it('preserves unknown currency conversion without invalidating totals for excluded transfers', () => {
        expect(ledgerTotals([{type:2,cny:'100'},{type:3,cny:null}])).toEqual({income:'100',expense:'0',balance:'100',complete:false});
        expect(ledgerTotals([{type:4,cny:null}]).complete).toBe(true);
        expect(ledgerMoney(null)).toBe('—');
    });
    it('shows refunds and reversals with their actual direction', () => {
        expect(ledgerSignedAmount('-2.34',3)).toBe('+2.34');
        expect(ledgerSignedAmount('-0.10',2)).toBe('-0.10');
        expect(ledgerSignedAmount('12.34',3)).toBe('-12.34');
        expect(ledgerMoney('9007199254740993.01',false)).toBe('9,007,199,254,740,993.01');
    });
});
