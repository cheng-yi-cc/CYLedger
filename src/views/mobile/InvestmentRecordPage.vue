<template><f7-page class="cy-main-page cy-mobile-surface" @page:afterin="start"><f7-navbar :title="f7route.query['action'] === 'account' ? f7route.query['accountId'] ? '编辑投资账户' : '添加投资账户' : '记录投资'" back-link="返回" /><InvestmentWorkspace ref="workspace" editor-only @changed="saved" @close="f7router.back()" /></f7-page></template>
<script setup lang="ts">
import { useTemplateRef } from 'vue';
import type { Router } from 'framework7/types';
import { investments } from '@/lib/investments.ts';
import { isCryptoAccount } from '@/lib/crypto-platforms.ts';
import InvestmentWorkspace from '@/components/InvestmentWorkspace.vue';
const props=defineProps<{f7route:Router.Route;f7router:Router.Router}>();
const workspace=useTemplateRef<InstanceType<typeof InvestmentWorkspace>>('workspace');
let initialized=false;
async function start():Promise<void>{if(initialized)return;initialized=true;const q=props.f7route.query;const action=q['action']||'OPENING';if(q['eventId']){const event=(await investments.events()).find(e=>e.id===q['eventId']);if(event?.wallet){props.f7router.navigate('/crypto/entry?'+new URLSearchParams({eventId:event.id,action}),{reloadCurrent:true});return;}}if(action==='account' && isCryptoAccount(q['kind'])){props.f7router.navigate('/crypto/add?kind='+q['kind'],{reloadCurrent:true});return;}if(q['accountId'] && ['OPENING','BUY','SELL','TRANSFER'].includes(action)){const account=(await investments.accounts()).find(a=>a.id===q['accountId']);if(isCryptoAccount(account?.kind)){const mode={OPENING:'opening',BUY:q['instrumentId'] && !['crypto:tether','crypto:usd-coin'].includes(q['instrumentId'])?'coin':'cash',SELL:'redeem',TRANSFER:'transfer'}[action]||'cash';props.f7router.navigate('/crypto/convert?'+new URLSearchParams({mode,accountId:q['accountId']||'',instrumentId:q['instrumentId']||''}),{reloadCurrent:true});return;}}await workspace.value?.startEditor(props.f7route.query['action']||'OPENING',props.f7route.query['instrumentId'],props.f7route.query['accountId'],props.f7route.query['eventId'],props.f7route.query['kind']);}
function saved():void{props.f7router.back();}
</script>
