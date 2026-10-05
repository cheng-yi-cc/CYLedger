import axios from 'axios';
import '@/lib/services.ts';
import type {ApiResponse} from '@/core/api.ts';
import {useTransactionsStore} from '@/stores/transaction.ts';
export interface DebtReport {accountId:string;paidPrincipal:string;interest:string;nextDate:string}
export interface DebtMovementInput {debtAccountId:string;cashAccountId:string;action:'borrow'|'lend'|'repay'|'collect';principal:string;interest:string;time:number;timeZone:string;bookId:string;note:string;requestId:string}
async function request<T>(method:'get'|'post',path:string,data?:unknown):Promise<T>{const r=await axios.request<ApiResponse<T>&{errorMessage?:string}>({method,url:`v1/assets/debts${path}`,data});if(!r.data.success)throw Error(r.data.errorMessage||'借还款操作失败');if(method==='post')useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});return r.data.result;}
export const debtAccounts={reports:()=>request<DebtReport[]>('get',''),record:(input:DebtMovementInput)=>request<{principalTransactionId:string;interestTransactionId:string}>('post','/record',input)};
