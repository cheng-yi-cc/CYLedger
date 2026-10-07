<template><f7-page class="cy-main-page cy-mobile-surface"><f7-navbar title="每日定投" back-link="交易所账户" /><main class="cy-page-body"><CryptoDCAWorkspace :account-id="f7route.query['accountId']" /></main></f7-page></template>
<script setup lang="ts">
import type { Router } from 'framework7/types';
import CryptoDCAWorkspace from '@/components/CryptoDCAWorkspace.vue';
defineProps<{f7route:Router.Route}>();
</script>
