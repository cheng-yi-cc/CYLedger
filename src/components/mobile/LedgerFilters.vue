<template>
    <details class="cy-ledger-filters">
        <summary>筛选 <span>{{ activeCount ? `${activeCount} 项条件` : '账户 · 分类 · 标签 · 收支' }}</span></summary>
        <div class="cy-filter-fields">
            <label>资金账户<select v-model="scope.accountId"><option value="">全部账户</option><option v-for="account in accountList" :key="account.id" :value="account.id">{{ account.name }}</option></select></label>
            <label>分类<select v-model="scope.categoryId"><option value="">全部分类</option><option v-for="category in categoryList" :key="category.id" :value="category.id">{{ category.name }}</option></select></label>
            <label>标签<select v-model="scope.tagId"><option value="">全部标签</option><option v-for="tag in tagList" :key="tag.id" :value="tag.id">{{ tag.name }}</option></select></label>
            <label>收支<select v-model.number="scope.type"><option :value="0">全部类型</option><option :value="3">支出</option><option :value="2">收入</option><option :value="4">转账</option></select></label>
            <button class="cy-button" @click="scope.resetFilters">清除筛选</button>
        </div>
    </details>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
const scope = useLedgerScopeStore(), accounts = useAccountsStore(), categories = useTransactionCategoriesStore(), tags = useTransactionTagsStore();
const accountList = computed(() => Object.values(accounts.allAccountsMap).filter(item => !item.subAccounts?.length));
const categoryList = computed(() => Object.values(categories.allTransactionCategoriesMap));
const tagList = computed(() => Object.values(tags.allTransactionTagsMap));
const activeCount = computed(() => [scope.accountId,scope.categoryId,scope.tagId,scope.type].filter(Boolean).length);
</script>
<style scoped>
.cy-ledger-filters{font-size:12px;margin:0 0 20px}.cy-ledger-filters summary{cursor:pointer;color:var(--cy-ink)}.cy-ledger-filters summary span{color:var(--cy-muted);margin-left:8px}.cy-filter-fields{display:grid;grid-template-columns:1fr 1fr;gap:12px;padding:14px 0}.cy-filter-fields label{min-width:0;color:var(--cy-muted)}.cy-filter-fields select{display:block;width:100%;border:1px solid var(--cy-line);border-radius:7px;padding:8px;margin-top:5px;background:var(--cy-card);color:var(--cy-ink)}
</style>
