<template>
    <f7-link :id="menuId" icon-f7="ellipsis_vertical" aria-label="账户更多操作" @click="showMenu=true" />
    <f7-popover :target-el="`#${menuId}`" v-model:opened="showMenu"><f7-list><f7-list-item v-for="item in items" :key="item.title" :title="item.title" :link="item.href||'#'" popover-close @click="item.action&&emit('action',item.action)"/><f7-list-item :title="editTitle||'编辑账户'" :link="editHref" popover-close /><f7-list-item :title="deleteTitle||'删除账户'" link="#" popover-close @click="showDeletion=true" /></f7-list></f7-popover>
    <AccountDeletionSheet v-model:opened="showDeletion" :target="target" immediate @deleted="emit('deleted')" />
</template>
<script setup lang="ts">
import {ref} from 'vue';
import {generateRandomUUID} from '@/lib/misc.ts';
import AccountDeletionSheet from '@/components/mobile/AccountDeletionSheet.vue';
defineProps<{editHref:string;editTitle?:string;deleteTitle?:string;items?:{title:string;href?:string;action?:string}[];target:{id:string;name:string;kind:'cash'|'portfolio';href:string}}>();
const emit=defineEmits<{deleted:[];action:[value:string]}>();
const menuId='account-menu-'+generateRandomUUID(),showMenu=ref(false),showDeletion=ref(false);
</script>
