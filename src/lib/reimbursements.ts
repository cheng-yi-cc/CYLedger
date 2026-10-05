import axios from 'axios';
import '@/lib/services.ts';
import type { ApiResponse } from '@/core/api.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
export interface ReimbursementReceipt {id:string;transactionId:string;accountId:string;amount:string;time:number;comment:string}
export interface ReimbursementClaim {transactionId:string;accountId:string;sourceAccountId:string;bookId:string;categoryId:string;currency:string;amount:string;paid:string;pending:string;time:number;comment:string;closed:boolean;ended:boolean;receipts:ReimbursementReceipt[]}
async function request<T>(method:'get'|'post',path:string,data?:unknown):Promise<T>{
 const r=await axios.request<ApiResponse<T>&{errorMessage?:string}>({method,url:`v1/assets/reimbursements${path}`,data});
 if(!r.data.success)throw Error(r.data.errorMessage||'报销操作失败');
 if(method==='post')useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});
 return r.data.result;
}
export const reimbursements={
 list:()=>request<ReimbursementClaim[]>('get',''),
 receive:(data:{expenseId:string;accountId:string;amount:string;time:number;timeZone:string;comment:string;requestId:string})=>request<ReimbursementReceipt>('post','/receive',data),
 state:(expenseId:string,action:'end'|'reopen'|'unassign')=>request<boolean>('post','/state',{expenseId,action})
};
