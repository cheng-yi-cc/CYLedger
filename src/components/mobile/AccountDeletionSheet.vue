<template>
    <f7-sheet :opened="opened" class="cy-account-picker cy-mobile-surface" backdrop :backdrop-close="!deleting" :swipe-to-close="!deleting" :close-on-escape="!deleting" @sheet:closed="closed" @sheet:open="initialize">
        <div class="cy-picker-heading"><strong>{{ target.name }}</strong><button :disabled="deleting" @click="close">关闭</button></div>
        <div class="cy-delete-content">
            <button v-if="!confirming" class="cy-delete-button" @click="prepare">删除账户</button>
            <template v-else>
                <p v-if="loading" role="status">正在检查关联记录…</p>
                <template v-else-if="preview">
                    <h2>删除“{{ preview.name }}”？</h2>
                    <p v-if="related">该账户还有 {{ preview.transactionCount }} 笔流水、{{ preview.investmentCount }} 笔投资记录<span v-if="preview.templateCount">、{{ preview.templateCount }} 个记账模板（含周期记账）</span>。可以先处理关联记录，或确认后一并删除。</p>
                    <p v-else>该账户没有关联交易，确认后将删除账户。</p>
                    <p v-if="preview.subAccountCount">同时删除 {{ preview.subAccountCount }} 个子账户。</p>
                    <p v-if="preview.dueCount || preview.depositCount">同时取消 {{ preview.dueCount }} 个到期事项与 {{ preview.depositCount }} 笔定存的后续收益和通知。</p>
                    <p v-if="preview.installmentCount">同时结束 {{preview.installmentCount}} 个分期计划，取消未入账服务费。</p>
                    <p v-if="preview.cryptoDcaPlanCount">同时停止 {{ preview.cryptoDcaPlanCount }} 个加密货币定投计划；已入账记录随投资流水处理。</p>
                    <p v-if="preview.parentAccountName">这是最后一个子账户，空主账户“{{ preview.parentAccountName }}”也将删除。</p>
                    <p v-if="related" class="cy-muted">删除会撤销关联转账的两侧流水、重算投资持仓和成本；关联模板一并删除，自动收益停止。此操作无法从界面撤回。</p>
                    <p v-if="preview.affectedAccounts.length">以下账户的余额或持仓也会更新：{{ preview.affectedAccounts.join('、') }}。</p>
                    <p v-if="preview.blockedReason" class="cy-message" role="alert">{{ preview.blockedReason }}</p>
                    <button v-if="related || preview.blockedReason" class="cy-process-button" :disabled="deleting" @click="processRelated">先处理关联交易</button>
                    <button class="cy-delete-button" :disabled="deleting || !!preview.blockedReason" @click="remove">{{ deleting ? '正在删除…' : related ? '账户与关联记录一并删除' : '确认删除账户' }}</button>
                </template>
                <p v-if="error" class="cy-message" role="alert">{{ error }}</p>
                <button v-if="error" class="cy-process-button" :disabled="deleting" @click="prepare">重新检查</button>
            </template>
        </div>
    </f7-sheet>
</template>
<script setup lang="ts">
import { computed, ref } from 'vue';
import { f7 } from 'framework7-vue';
import { investments, investmentError } from '@/lib/investments.ts';
import type { AccountDeletionPreview } from '@/models/investment.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import { useTransactionTemplatesStore } from '@/stores/transactionTemplate.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';

const props = defineProps<{ opened: boolean; target: { id:string; name:string; kind:'cash'|'portfolio'; href:string }; immediate?:boolean }>();
const emit = defineEmits<{ 'update:opened':[value:boolean]; deleted:[]; busy:[value:boolean] }>();
const confirming=ref(false), loading=ref(false), deleting=ref(false), error=ref(''), preview=ref<AccountDeletionPreview>();
const related=computed(()=>!!preview.value && preview.value.transactionCount+preview.value.investmentCount+preview.value.templateCount+(preview.value.dueCount||0)+(preview.value.depositCount||0)+(preview.value.installmentCount||0)+(preview.value.cryptoDcaPlanCount||0)>0);
let request=0;
function close():void { if(!deleting.value) emit('update:opened',false); }
function closed():void { request++; close(); }
function initialize():void { request++; confirming.value=false; error.value=''; preview.value=undefined; if(props.immediate) void prepare(); }
async function prepare():Promise<void> {
    if(deleting.value)return;
    const version=++request; confirming.value=true; loading.value=true; error.value=''; preview.value=undefined;
    try { const result=await investments.previewAccountDeletion({id:props.target.id,kind:props.target.kind}); if(version===request)preview.value=result; }
    catch(cause){if(version===request)error.value=investmentError(cause);}
    finally{if(version===request)loading.value=false;}
}
function processRelated():void { close(); f7.views.main.router.navigate(props.target.href); }
async function remove():Promise<void> {
    if(!preview.value || deleting.value || preview.value.blockedReason)return;
    deleting.value=true; emit('busy',true); error.value='';
    try {
        await investments.deleteAccount({id:props.target.id,kind:props.target.kind},preview.value.token,related.value);
        useTransactionsStore().updateStoreInvalidState({transactionList:true,reconciliationStatement:true,accountList:true,overview:true,statistics:true,explorer:true});
        const templates=useTransactionTemplatesStore(); templates.updateTransactionTemplateListInvalidState(1,true);templates.updateTransactionTemplateListInvalidState(2,true);
        const scope=useLedgerScopeStore(); if(scope.accountId && !useAccountsStore().allAccountsMap[scope.accountId])scope.accountId='';
        emit('update:opened',false); emit('deleted'); f7.toast.create({text:'账户已删除',closeTimeout:2000}).open();
    }catch(cause){error.value=investmentError(cause);preview.value=undefined;}
    finally{deleting.value=false;emit('busy',false);}
}
</script>
<style scoped>
.cy-account-picker{height:auto;max-height:88vh;border-radius:20px 20px 0 0;overflow:hidden}
.cy-picker-heading{display:flex;align-items:center;justify-content:space-between;padding:20px;border-bottom:1px solid var(--cy-line)}.cy-picker-heading button{background:none;border:0;color:var(--cy-accent);font:inherit}.cy-delete-content{padding:20px;max-height:65vh;overflow:auto}.cy-delete-content h2{font-size:18px;margin:0 0 15px}.cy-delete-content p{font-size:13px;line-height:1.8;margin:0 0 14px}.cy-delete-content button{width:100%;min-height:46px;padding:12px;border-radius:10px;font:inherit;margin:6px 0;border:1px solid var(--cy-line)}.cy-delete-button{background:var(--cy-expense);color:white;border-color:var(--cy-expense)!important}.cy-process-button{background:var(--cy-card);color:var(--cy-accent)}.cy-delete-content button:disabled{opacity:.5}
</style>
