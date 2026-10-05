import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
export interface CreditStatement {month:string;startDate:string;endDate:string;statementDate:string;dueDate:string;charges:string;fees:string;futureFees:string;remaining:string;unbilled:boolean;transactionIds:string[]}
export interface CreditReport {accountId:string;accountName:string;currency:string;outstanding:string;statements:CreditStatement[];annualFee:string;annualDate:string;annualSpend:string;annualCount:number;annualWaived:boolean}
export interface InstallmentPayment {date:string;principal:string;fee:string;accrued:boolean;feeTransactionId:string}
export interface InstallmentData {principal:string;totalFee:string;periods:number;firstDate:string;method:'monthly'|'first_fee'|'immediate_fee'|'balloon';remainder:'first'|'last'|'except_first'|'except_last';bookId:string;timeZone:string;note:string;payments:InstallmentPayment[]}
export interface CreditInstallment {id:string;accountId:string;expenseId:string;statementMonth:string;version:string;closed:boolean;data:InstallmentData}
export interface InstallmentInput extends InstallmentData {id:string;accountId:string;expenseId:string;statementMonth:string;version:string;requestId:string}
async function request<T>(method:'get'|'post',path:string,data?:unknown,params?:unknown):Promise<T>{
 const r=await axios.request<ApiResponse<T>&{errorMessage?:string}>({method,url:`v1/assets/${path}`,data,params});
 if(!r.data.success)throw Error(r.data.errorMessage||'信贷操作失败');
 if(method==='post'&&path!=='installments/preview')useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});
 return r.data.result;
}
export const creditAccounts={
 reports:(timeZone:string,accountId='')=>request<CreditReport[]>('get','credit-reports',undefined,{timeZone,accountId}),
 installments:()=>request<CreditInstallment[]>('get','installments'),
 preview:(input:InstallmentInput)=>request<InstallmentData>('post','installments/preview',input),
 save:(input:InstallmentInput)=>request<CreditInstallment>('post','installments/save',input),
 close:(id:string,version:string)=>request<boolean>('post','installments/close',{id,version}),
 sync:()=>request<number>('post','installments/sync',{})
};
