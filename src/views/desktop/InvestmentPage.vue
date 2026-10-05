<template>
    <main-page-layout :no-navbar="true">
        <template #content>
            <v-alert v-if="route.query['fromTransaction'] === '1'" type="info" variant="tonal" class="ma-4">
                已打开投资流水，请在对应记录中查看、修订或撤销。
            </v-alert>
            <InvestmentWorkspace :key="route.query['view'] === 'history' ? 'history' : 'overview'"
                                 :initial-tab="route.query['view'] === 'history' ? 'history' : 'overview'"
                                 @navigate="navigate" />
        </template>
    </main-page-layout>
</template>
<script setup lang="ts">
import { useRoute, useRouter } from 'vue-router';
import InvestmentWorkspace from '@/components/InvestmentWorkspace.vue';
const router = useRouter();
const route = useRoute();
function navigate(destination: 'accounts' | 'data'): void { void router.push(destination === 'accounts' ? '/account/list' : '/settings/user/data_management'); }
</script>
