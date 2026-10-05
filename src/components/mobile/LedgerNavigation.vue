<template>
    <nav class="cy-bottom-nav" aria-label="主导航" @click.capture="skipActiveNavigation">
        <f7-link :animate="false" href="/" :class="{ 'cy-active': active === 'home' || active === 'bills' }" :aria-current="active === 'home' || active === 'bills' ? 'page' : undefined"><f7-icon f7="house" /><span>首页</span></f7-link>
        <f7-link :animate="false" href="/calendar" :class="{ 'cy-active': active === 'calendar' }" :aria-current="active === 'calendar' ? 'page' : undefined"><f7-icon f7="calendar" /><span>日历</span></f7-link>
        <f7-link :animate="false" href="/investments" :class="{ 'cy-active': active === 'assets' }" :aria-current="active === 'assets' ? 'page' : undefined"><f7-icon f7="creditcard" /><span>资产</span></f7-link>
        <f7-link :animate="false" href="/statistics" :class="{ 'cy-active': active === 'statistics' }" :aria-current="active === 'statistics' ? 'page' : undefined"><f7-icon f7="chart_pie" /><span>统计</span></f7-link>
        <f7-link :animate="false" href="/settings" :class="{ 'cy-active': active === 'settings' }" :aria-current="active === 'settings' ? 'page' : undefined"><f7-icon f7="person_crop_circle" /><span>我的</span></f7-link>
    </nav>
</template>
<script setup lang="ts">
defineProps<{ active: 'home' | 'calendar' | 'bills' | 'statistics' | 'assets' | 'settings' }>();

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
</style>
