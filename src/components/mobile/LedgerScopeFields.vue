<template>
    <details><summary>账本 · {{ bookIds.length ? `已选 ${bookIds.length} 个` : '全部' }}</summary><div class="cy-choice-grid"><label v-for="b in books.allBooks" :key="b.id"><input v-model="bookIds" type="checkbox" :value="b.id" />{{ b.name }}</label></div></details>
    <details v-if="accountsEnabled"><summary>账户 · {{ accountIds.length ? `已选 ${accountIds.length} 个` : '全部' }}</summary><div class="cy-choice-grid"><label v-for="a in accounts.allPlainAccounts.filter(a=>!a.subAccounts?.length)" :key="a.id"><input v-model="accountIds" type="checkbox" :value="a.id" />{{ a.name }}</label></div></details>
    <details v-if="categoriesEnabled"><summary>分类 · {{ categoryIds.length ? `已选 ${categoryIds.length} 个` : '全部' }}</summary><div class="cy-choice-grid"><label v-for="c in Object.values(categories.allTransactionCategoriesMap)" :key="c.id"><input v-model="categoryIds" type="checkbox" :value="c.id" />{{ c.name }}</label></div></details>
    <details v-if="tagsEnabled"><summary>标签 · {{ tagIds.length ? `已选 ${tagIds.length} 个` : '全部' }}</summary><div class="cy-choice-grid"><label v-for="t in Object.values(tags.allTransactionTagsMap)" :key="t.id"><input v-model="tagIds" type="checkbox" :value="t.id" />{{ t.name }}</label></div></details>
</template>
<script setup lang="ts">
import { useBooksStore } from '@/stores/books.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
defineProps<{accountsEnabled?:boolean;categoriesEnabled?:boolean;tagsEnabled?:boolean}>();
const bookIds=defineModel<string[]>('bookIds',{default:()=>[]}),accountIds=defineModel<string[]>('accountIds',{default:()=>[]}),categoryIds=defineModel<string[]>('categoryIds',{default:()=>[]}),tagIds=defineModel<string[]>('tagIds',{default:()=>[]});
const books=useBooksStore(),accounts=useAccountsStore(),categories=useTransactionCategoriesStore(),tags=useTransactionTagsStore();
</script>
<style scoped>
details{padding:14px 0;border-bottom:1px solid var(--cy-line)}summary{cursor:pointer;font-size:14px}.cy-choice-grid{display:grid;grid-template-columns:repeat(2,minmax(0,1fr));gap:12px;padding-top:14px}.cy-choice-grid label{display:flex;align-items:center;gap:8px;font-size:13px}.cy-choice-grid input{accent-color:var(--cy-accent)}
</style>
