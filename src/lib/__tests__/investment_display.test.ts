import {describe,expect,it} from 'vitest';
import {investmentDecimalText,investmentProfitClass,quoteChange,quoteStatus} from '@/lib/investment-display.ts';
import type {InvestmentQuote} from '@/models/investment.ts';

describe('investment detail with unavailable market data',()=>{
    it('renders an unavailable quote with an empty price without treating it as zero',()=>{
        const quote={state:'unavailable',price:'',currency:'USD',changePercent:''} as InvestmentQuote;
        expect(investmentDecimalText(quote.price,6)).toBe('—');
        expect(investmentProfitClass(quote.changePercent)).toBe('');
        expect(quoteChange(quote)).toBe('—');
        expect(quoteStatus(quote)).toBe('缺少报价');
        expect(investmentDecimalText(null)).toBe('—');
        expect(investmentDecimalText(undefined)).toBe('—');
    });
    it('retains known zero and small decimal values',()=>{
        expect(investmentDecimalText('0')).toBe('0');
        expect(investmentDecimalText('0.000012345678',8)).toBe('0.00001235');
        expect(investmentProfitClass('0')).toBe('');
        expect(investmentProfitClass('0.00000001')).toBe('inv-up');
        expect(investmentProfitClass('-0.00000001')).toBe('inv-down');
    });
});
