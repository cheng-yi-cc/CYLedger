import { describe, it, expect } from 'vitest';
import { cryptoInput, heldCryptoCoins, defaultCryptoCoin, receivedAfterQuote, calculateCashPurchase, buildCryptoEvent, type CryptoDraft } from '../crypto-entry.ts';
import type { Instrument, InvestmentPosition, InvestmentConversion } from '@/models/investment.ts';
const coins:Instrument[]=[{id:'crypto:tether',symbol:'USDT',name:'Tether',type:'CRYPTO',precision:18},{id:'crypto:bitcoin',symbol:'BTC',name:'Bitcoin',type:'CRYPTO',precision:18}];
const position=(accountId:string,instrumentId:string,quantity:string):InvestmentPosition=>({accountId,instrumentId,quantity,cost:null,costKnown:false,averageCost:null,realizedPnl:null});
const draft:CryptoDraft={mode:'cash',fromAccount:'bank',toAccount:'wallet',fromCoin:'',toCoin:'crypto:bitcoin',amount:'700',received:'0.001',bookId:'b',note:'',cashEntryMode:'amount',buyPrice:'700000'};
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
 it('calculates quantity from actual payment and the entered advertisement price',()=>{
  expect(calculateCashPurchase('amount','720','','7.20')).toEqual({amount:'720',quantity:'100'});
  expect(calculateCashPurchase('amount','1000','','7.20')).toEqual({amount:'1000',quantity:'138.888888888888888888'});
  expect(calculateCashPurchase('amount','0.01','','10000000000000000')).toEqual({amount:'0.01',quantity:'0.000000000000000001'});
  expect(()=>calculateCashPurchase('amount','0.01','','100000000000000000')).toThrow('最小精度');
 });
 it('rounds calculated CNY payment to cents without changing the entered quantity',()=>{
  expect(calculateCashPurchase('quantity','','100','7.20')).toEqual({amount:'720',quantity:'100'});
  expect(calculateCashPurchase('quantity','','0.001','7.20')).toEqual({amount:'0.01',quantity:'0.001'});
  expect(calculateCashPurchase('quantity','','1.5','0.67')).toEqual({amount:'1.01',quantity:'1.5'});
  expect(calculateCashPurchase('quantity','','9007199254740993.01','0.1')).toEqual({amount:'900719925474099.3',quantity:'9007199254740993.01'});
  expect(()=>calculateCashPurchase('quantity','','0.0001','7.2')).toThrow('0.01');
 });
 it('requires a valid positive unit price and ignores stale market estimates and derived fields',()=>{
  for(const price of ['', '0', '-1', '1e3', '1'.repeat(61)])expect(()=>calculateCashPurchase('amount','720','',price)).toThrow();
  const byPayment=buildCryptoEvent({...draft,amount:'720',received:'999',buyPrice:'7.2'},quote,1001);
  expect(byPayment).toMatchObject({amount:'720',quantity:'100'});expect(byPayment.conversion).toBeUndefined();
  const byQuantity=buildCryptoEvent({...draft,cashEntryMode:'quantity',amount:'999',received:'1.5',buyPrice:'0.67'},quote,1001);
  expect(byQuantity).toMatchObject({amount:'1.01',quantity:'1.5',exchangeRate:'1'});expect(byQuantity.conversion).toBeUndefined();
 });
 it('keeps unknown crypto settlement FX unknown',()=>{
  const e=buildCryptoEvent({...draft,mode:'coin',fromAccount:'wallet',fromCoin:'crypto:tether'},undefined,1000);
  expect(e).toMatchObject({settlementInstrumentId:'crypto:tether',exchangeRate:'',cashAccountId:'',cost:null});
 });
 it('retains a fresh matching reference without overriding actual receipt',()=>{
  const coinDraft:CryptoDraft={...draft,mode:'coin',fromCoin:'crypto:tether'};
  const coinQuote={...quote,fromInstrumentId:'crypto:tether',fromPrice:'7'};
  const e=buildCryptoEvent(coinDraft,coinQuote,1001);expect(e.conversion).toEqual(coinQuote);expect(e.quantity).toBe('0.001');
  expect(buildCryptoEvent(coinDraft,coinQuote,1121).conversion).toBeUndefined();
  expect(buildCryptoEvent({...coinDraft,amount:'701'},coinQuote,1001).conversion).toBeUndefined();
  expect(buildCryptoEvent({...coinDraft,toCoin:'crypto:ethereum'},coinQuote,1001).conversion).toBeUndefined();
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
 it('records known opening cost and historical time without inventing cost when omitted',()=>{
  const d:CryptoDraft={...draft,mode:'opening',amount:'0.1',cost:'68000.25',occurredAt:500};
  expect(buildCryptoEvent(d,quote,1000)).toMatchObject({type:'OPENING',quantity:'0.1',cost:'68000.25',occurredAt:500});
  expect(buildCryptoEvent({...d,cost:''},quote,1000).cost).toBeNull();
  expect(buildCryptoEvent({...d,cost:'0'},quote,1000).cost).toBe('0');
  expect(()=>buildCryptoEvent({...d,cost:'-1'},undefined,1000)).toThrow();
  expect(()=>buildCryptoEvent({...d,occurredAt:2000},undefined,1000)).toThrow('发生时间');
 });
 it('transfers actual receipt and network fee out of the entered total debit',()=>{
  const d:CryptoDraft={...draft,mode:'transfer',fromAccount:'exchange',toAccount:'wallet',fromCoin:'crypto:bitcoin',toCoin:'crypto:bitcoin',amount:'0.1',received:'0.099',fee:'0.001'};
  expect(buildCryptoEvent(d,undefined,1000)).toMatchObject({type:'TRANSFER',quantity:'0.099',fee:'0.001',amount:'0',cost:null});
  expect(()=>buildCryptoEvent({...d,received:'0.1'},undefined,1000)).toThrow('总扣除');
  expect(()=>buildCryptoEvent({...d,fee:'-1'},undefined,1000)).toThrow();
  expect(buildCryptoEvent({...d,received:'',fee:'0'},undefined,1000).quantity).toBe('0.1');
 });
 it('never applies a live reference to a backdated coin trade',()=>{
  const d:CryptoDraft={...draft,mode:'coin',fromCoin:'crypto:tether',occurredAt:500};
  const current={...quote,fromInstrumentId:'crypto:tether',fromPrice:'7'};
  const unknown=buildCryptoEvent(d,current,1000);
  expect(unknown.exchangeRate).toBe('');expect(unknown.conversion).toBeUndefined();
  const known=buildCryptoEvent({...d,exchangeRate:'6.5'},current,1000);
  expect(known.exchangeRate).toBe('6.5');expect(known.conversion).toBeUndefined();
 });
});
