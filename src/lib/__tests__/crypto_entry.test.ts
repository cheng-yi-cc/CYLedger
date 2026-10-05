import { describe, it, expect } from 'vitest';
import { cryptoInput, heldCryptoCoins, defaultCryptoCoin, receivedAfterQuote, buildCryptoEvent, type CryptoDraft } from '../crypto-entry.ts';
import type { Instrument, InvestmentPosition, InvestmentConversion } from '@/models/investment.ts';
const coins:Instrument[]=[{id:'crypto:tether',symbol:'USDT',name:'Tether',type:'CRYPTO',precision:18},{id:'crypto:bitcoin',symbol:'BTC',name:'Bitcoin',type:'CRYPTO',precision:18}];
const position=(accountId:string,instrumentId:string,quantity:string):InvestmentPosition=>({accountId,instrumentId,quantity,cost:null,costKnown:false,averageCost:null,realizedPnl:null});
const draft:CryptoDraft={mode:'cash',fromAccount:'bank',toAccount:'wallet',fromCoin:'',toCoin:'crypto:bitcoin',amount:'700',received:'0.001',bookId:'b',note:''};
const quote:InvestmentConversion={fromInstrumentId:'',toInstrumentId:'crypto:bitcoin',fromQuantity:'700',toQuantity:'0.002',fromPrice:'1',toPrice:'350000',observedAt:1000,expiresAt:1120};
describe('crypto entry facts',()=>{
 it('normalizes keypad input without losing precision',()=>{
  expect(cryptoInput('.000000000000000001')).toBe('0.000000000000000001');
  expect(cryptoInput(' 000123.4500 ')).toBe('123.45'); expect(cryptoInput('1.')).toBe('1');
  expect(cryptoInput('9007199254740993.01',2)).toBe('9007199254740993.01');
  for(const value of ['0','-1','1e3','1,000','1.0000000000000000001','1'.repeat(61)])expect(()=>cryptoInput(value)).toThrow();
  expect(()=>cryptoInput('1.001',2)).toThrow('2');
 });
 it('selects holdings from only the chosen account instead of defaulting to absent USDT',()=>{
  const held=heldCryptoCoins(coins,[position('wallet','crypto:bitcoin','0.02'),position('wallet','crypto:tether','0'),position('exchange','crypto:tether','100')],'wallet');
  expect(held.map(c=>c.symbol)).toEqual(['BTC']);expect(defaultCryptoCoin(held,'crypto:tether')).toBe('crypto:bitcoin');expect(defaultCryptoCoin([])).toBe('');
 });
 it('does not replace an explicitly edited received quantity with a late quote',()=>{
  expect(receivedAfterQuote('0.0015',true,'0.002')).toBe('0.0015');
  expect(receivedAfterQuote('',false,'0.002')).toBe('0.002');
 });
 it('records actual CNY settlement without a network quote',()=>{
  const e=buildCryptoEvent(draft,undefined,1000);expect(e).toMatchObject({type:'BUY',amount:'700',quantity:'0.001',cashAccountId:'bank',exchangeRate:'1',cost:null,occurredAt:1000});expect(e.conversion).toBeUndefined();
 });
 it('keeps unknown crypto settlement FX unknown',()=>{
  const e=buildCryptoEvent({...draft,mode:'coin',fromAccount:'wallet',fromCoin:'crypto:tether'},undefined,1000);
  expect(e).toMatchObject({settlementInstrumentId:'crypto:tether',exchangeRate:'',cashAccountId:'',cost:null});
 });
 it('retains a fresh matching reference without overriding actual receipt',()=>{
  const e=buildCryptoEvent(draft,quote,1001);expect(e.conversion).toEqual(quote);expect(e.quantity).toBe('0.001');
  expect(buildCryptoEvent(draft,quote,1121).conversion).toBeUndefined();
  expect(buildCryptoEvent({...draft,amount:'701'},quote,1001).conversion).toBeUndefined();
  expect(buildCryptoEvent({...draft,toCoin:'crypto:tether'},quote,1001).conversion).toBeUndefined();
 });
 it('records sell amounts and enforces cash precision',()=>{
  const d:CryptoDraft={...draft,mode:'redeem',fromAccount:'wallet',toAccount:'bank',fromCoin:'crypto:bitcoin',amount:'.001',received:'699.99'};
  expect(buildCryptoEvent(d,undefined,1000)).toMatchObject({type:'SELL',accountId:'wallet',cashAccountId:'bank',quantity:'0.001',amount:'699.99'});
  expect(()=>buildCryptoEvent({...d,received:'699.999'},undefined,1000)).toThrow();
 });
 it('blocks same-account transfers and same-coin swaps',()=>{
  expect(()=>buildCryptoEvent({...draft,mode:'transfer',fromAccount:'wallet',fromCoin:'crypto:bitcoin'},undefined,1000)).toThrow('另一个');
  expect(()=>buildCryptoEvent({...draft,mode:'coin',fromCoin:'crypto:bitcoin'},undefined,1000)).toThrow('不同币种');
 });
});
