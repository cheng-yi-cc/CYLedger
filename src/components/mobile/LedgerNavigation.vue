<template>
    <nav class="cy-bottom-nav" :class="['cy-nav-'+prefs.navigationStyle, { 'cy-nav-icons-only': !prefs.navigationText }]" :style="{gridTemplateColumns: `repeat(${prefs.tabs.length},minmax(0,1fr))`}" aria-label="主导航" @click.capture="skipActiveNavigation">
        <f7-link v-for="tab in prefs.tabs" :key="tab" :animate="false" :href="items[tab].href" :class="{ 'cy-active': current === tab }" :aria-current="current === tab ? 'page' : undefined" :aria-label="ledgerTabNames[tab]"><f7-icon :f7="items[tab].icon" /><span v-if="prefs.navigationText">{{ ledgerTabNames[tab] }}</span></f7-link>
    </nav>
</template>
<script setup lang="ts">
import { computed } from 'vue';
import { useLedgerExperienceStore } from '@/stores/ledgerExperience.ts';
import { ledgerTabNames } from '@/lib/ledger-preferences.ts';
const props = defineProps<{ active: 'home' | 'calendar' | 'bills' | 'statistics' | 'assets' | 'settings' }>();
const experience = useLedgerExperienceStore(), prefs = computed(() => experience.preferences);
const current = computed(() => props.active === 'bills' ? 'home' : props.active);
const items = { home: { href: '/', icon: 'house' }, calendar: { href: '/calendar', icon: 'calendar' }, assets: { href: '/investments', icon: 'creditcard' }, statistics: { href: '/statistics', icon: 'chart_pie' }, settings: { href: '/settings', icon: 'ellipsis_circle' } };

function skipActiveNavigation(event: MouseEvent): void {
    if (event.target instanceof Element && event.target.closest('a[aria-current="page"]')) {
        event.preventDefault();
        event.stopPropagation();
    }
}
</script>
<style>
.cy-bottom-nav{position:absolute;left:12px;right:12px;bottom:calc(10px + env(safe-area-inset-bottom));z-index:510;height:66px;padding:3px 7px;display:grid;grid-template-columns:repeat(5,minmax(0,1fr));background:var(--cy-card,var(--f7-navbar-bg-color,#fff));border:1px solid var(--cy-line,rgba(100,130,120,.18));border-radius:25px;box-shadow:0 4px 22px #173e3814;box-sizing:border-box}.cy-bottom-nav .link{display:flex;flex-direction:column;justify-content:center;gap:4px;min-height:54px;color:var(--cy-muted,#75827e);font-size:11px}.cy-bottom-nav .f7-icons{font-size:24px}.cy-bottom-nav .cy-active{color:var(--cy-accent,#12786f);font-weight:600}.dark .cy-bottom-nav .cy-active{color:#68bfae}.cy-bottom-nav a:focus-visible{outline:2px solid var(--cy-accent,#12786f);outline-offset:0;border-radius:15px}.cy-main-page>.page-content{padding-bottom:calc(98px + env(safe-area-inset-bottom))!important}.cy-main-page>.toolbar-bottom{bottom:calc(86px + env(safe-area-inset-bottom))}.cy-main-page:has(>.toolbar-bottom)>.page-content{padding-bottom:calc(140px + env(safe-area-inset-bottom))!important}
.dark .cy-bottom-nav{background:var(--cy-card,#202e35);border-color:var(--cy-line,#314148)}
.cy-bottom-nav{height:56px;border-radius:29px;padding:2px 7px;left:18px;right:18px;bottom:calc(14px + env(safe-area-inset-bottom));box-shadow:0 2px 12px #233b3a12}.cy-bottom-nav .link{min-height:48px;gap:3px;font-weight:400}.cy-bottom-nav .f7-icons{font-size:23px}.cy-bottom-nav.cy-nav-fixed{left:0;right:0;bottom:0;border-radius:0;border-inline:0;padding-bottom:env(safe-area-inset-bottom);height:calc(58px + env(safe-area-inset-bottom));box-shadow:none}.cy-bottom-nav.cy-nav-glass{background:color-mix(in srgb,var(--cy-card,#fff) 80%,transparent);backdrop-filter:blur(15px);border:1px solid color-mix(in srgb,var(--cy-ink,#233b3a) 14%,transparent)}.cy-bottom-nav.cy-nav-icons-only .f7-icons{font-size:26px}
</style>
