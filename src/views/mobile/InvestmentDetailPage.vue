<template>
    <f7-page class="cy-main-page" @page:afterin="workspace?.reload()"><f7-navbar title="投资理财" back-link="资产" /><InvestmentWorkspace ref="workspace" @navigate="navigate" /><template #fixed><LedgerNavigation active="assets" /></template></f7-page>
</template>
<script setup lang="ts">
import { useTemplateRef } from 'vue';
import type { Router } from 'framework7/types';
import InvestmentWorkspace from '@/components/InvestmentWorkspace.vue';
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
const workspace = useTemplateRef<InstanceType<typeof InvestmentWorkspace>>('workspace');
const props = defineProps<{ f7router: Router.Router }>();
function navigate(destination: 'accounts' | 'data'): void { props.f7router.navigate(destination === 'accounts' ? '/account/list' : '/user/data/management'); }
</script>
