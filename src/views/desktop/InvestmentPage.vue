<template>
    <main-page-layout :no-navbar="true"><template #content><InvestmentWorkspace @navigate="navigate" /></template></main-page-layout>
</template>
<script setup lang="ts">
import { useRouter } from 'vue-router';
import InvestmentWorkspace from '@/components/InvestmentWorkspace.vue';
const router = useRouter();
function navigate(destination: 'accounts' | 'data'): void { void router.push(destination === 'accounts' ? '/account/list' : '/settings/user/data_management'); }
</script>
