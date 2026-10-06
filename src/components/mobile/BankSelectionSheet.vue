<template>
 <f7-popup :opened="opened" class="cy-mobile-surface" @popup:closed="emit('update:opened',false)"><f7-page class="cy-mobile-surface cy-asset-surface"><f7-navbar :title="credit?'选择信用卡银行':'选择银行'"><f7-nav-left><f7-link icon-f7="xmark" @click="emit('update:opened',false)"/></f7-nav-left></f7-navbar><main class="bank-picker"><label class="bank-search"><f7-icon f7="search"/><input v-model="query" placeholder="搜索银行" aria-label="搜索银行"/></label><section><button v-for="bank in filtered" :key="bank.name" @click="choose(bank)"><span v-if="!bankLogo(bank.icon)" :style="{background:bank.color}">农</span><img v-else :src="bankLogo(bank.icon)" alt="" width="30" height="30" />{{bank.name}}</button><button @click="choose({name:'自定义银行',color:'#6ca699'})"><span>＋</span>自定义银行</button></section></main></f7-page></f7-popup>
</template>
<script setup lang="ts">
import {ref,computed} from 'vue';
import { BANK_BRANDS, bankLogo } from '@/lib/banks.ts';
defineProps<{opened:boolean;credit:boolean}>();
const emit=defineEmits<{'update:opened':[value:boolean];select:[bank:{name:string;color:string}]}>();
const query=ref('');
const banks=[...BANK_BRANDS,{id:'rural',name:'农村商业银行',icon:'100',color:'#6c997c',aliases:[]}];
const filtered=computed(()=>banks.filter(b=>[b.name,...b.aliases].some(name=>name.includes(query.value.trim()))));
function choose(bank:{name:string;color:string}):void{emit('select',bank);emit('update:opened',false);}
</script>
<style scoped>
.bank-picker{padding:14px}.bank-search{display:flex;align-items:center;gap:10px;background:var(--cy-card);border-radius:10px;padding:12px 14px;margin-bottom:14px}.bank-search .icon{font-size:18px;color:var(--cy-muted)}.bank-search input{border:0;background:none;font-size:15px;color:var(--cy-ink);min-width:0;flex:1}.bank-picker section{background:var(--cy-card);border-radius:12px;padding:0 16px}.bank-picker button{display:flex;align-items:center;gap:15px;background:none;border:0;min-height:53px;width:100%;text-align:left;color:var(--cy-ink);font-size:15px}.bank-picker button>img{object-fit:contain;flex:none}.bank-picker button>span{display:grid;place-items:center;width:29px;height:29px;border-radius:50%;font-size:14px;color:white;background:var(--cy-accent)}
</style>
