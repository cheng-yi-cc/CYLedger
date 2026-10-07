<template><CategoryListPage :f7route="f7route" :f7router="f7router" /></template>
<script setup lang="ts">
import type { Router } from 'framework7/types';
import CategoryListPage from './ListPage.vue';
defineProps<{ f7route: Router.Route; f7router: Router.Router }>();
</script>
