<template>
 <f7-popup :opened="opened" class="cy-mobile-surface" @popup:closed="emit('update:opened',false)"><f7-page class="cy-mobile-surface cy-asset-surface"><f7-navbar :title="credit?'选择信用卡银行':'选择银行'"><f7-nav-left><f7-link icon-f7="xmark" @click="emit('update:opened',false)"/></f7-nav-left></f7-navbar><main class="bank-picker"><label class="bank-search"><f7-icon f7="search"/><input v-model="query" placeholder="搜索银行" aria-label="搜索银行"/></label><section><button v-for="bank in filtered" :key="bank.name" @click="choose(bank)"><span :style="{background:bank.color}">{{bank.short}}</span>{{bank.name}}</button><button @click="choose({name:'自定义银行',color:'#6ca699'})"><span>＋</span>自定义银行</button></section></main></f7-page></f7-popup>
</template>
<script setup lang="ts">
import {ref,computed} from 'vue';
defineProps<{opened:boolean;credit:boolean}>();
const emit=defineEmits<{'update:opened':[value:boolean];select:[bank:{name:string;color:string}]}>();
const query=ref('');
const banks=[['工商银行','工','#c84444'],['农业银行','农','#36927a'],['中国银行','中','#bb4548'],['建设银行','建','#406eaa'],['交通银行','交','#465f9e'],['邮储银行','邮','#419865'],['招商银行','招','#bf434c'],['浦发银行','浦','#537cb8'],['兴业银行','兴','#44619b'],['民生银行','民','#4b8c82'],['中信银行','信','#cf5665'],['光大银行','光','#b186b7'],['平安银行','平','#da8745'],['华夏银行','华','#c25c64'],['广发银行','广','#c44c50'],['北京银行','京','#c65464'],['上海银行','沪','#d0a361'],['江苏银行','苏','#739bae'],['宁波银行','甬','#d79056'],['农村商业银行','农','#6c997c']].map(([name,short,color])=>({name:name!,short:short!,color:color!}));
const filtered=computed(()=>banks.filter(b=>b.name.includes(query.value.trim())));
function choose(bank:{name:string;color:string}):void{emit('select',bank);emit('update:opened',false);}
</script>
<style scoped>
.bank-picker{padding:14px}.bank-search{display:flex;align-items:center;gap:10px;background:var(--cy-card);border-radius:10px;padding:12px 14px;margin-bottom:14px}.bank-search .icon{font-size:18px;color:var(--cy-muted)}.bank-search input{border:0;background:none;font-size:15px;color:var(--cy-ink);min-width:0;flex:1}.bank-picker section{background:var(--cy-card);border-radius:12px;padding:0 16px}.bank-picker button{display:flex;align-items:center;gap:15px;background:none;border:0;min-height:53px;width:100%;text-align:left;color:var(--cy-ink);font-size:15px}.bank-picker button>span{display:grid;place-items:center;width:29px;height:29px;border-radius:50%;font-size:14px;color:white;background:var(--cy-accent)}
</style>
