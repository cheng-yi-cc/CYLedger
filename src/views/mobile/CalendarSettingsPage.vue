<template>
    <f7-page class="cy-main-page cy-mobile-surface">
        <f7-navbar title="偏好设置" back-link="返回" />
        <main class="cy-page-body">
            <section class="cy-panel cy-preferences">
                <label class="cy-preference"><span><strong>周起始日</strong><small>设置日历视图中每周的起始日期</small></span><select aria-label="周起始日" :value="settings.appSettings.calendarWeekStart" @change="setWeek"><option v-for="(day,index) in weekdays" :key="index" :value="index">{{ day }}</option></select></label>
                <div v-for="option in options" :key="option.key" class="cy-preference"><span><strong>{{ option.title }}</strong><small>{{ option.subtitle }}</small></span><button type="button" class="cy-preference-toggle" role="switch" :aria-checked="settings.appSettings[option.key]" :aria-label="option.title" @click="settings.setCalendarPreference(option.key, !settings.appSettings[option.key])" /></div>
            </section>
            <f7-link class="cy-panel cy-due-link" href="/calendar/due"><span>管理到期事项<small>手动添加还款与定存到期日期</small></span><f7-icon f7="chevron_right" /></f7-link>
        </main>
    </f7-page>
</template>
<script setup lang="ts">
import { useSettingsStore } from '@/stores/setting.ts';
const settings = useSettingsStore();
const weekdays = ['周日','周一','周二','周三','周四','周五','周六'];
const options = [
    { key: 'calendarShowRepayments', title: '显示还款信息', subtitle: '在日历视图中显示信贷账户的待还事项' },
    { key: 'calendarShowDeposits', title: '显示定存信息', subtitle: '在日历视图中显示定期存款到期信息' },
    { key: 'calendarShowLunar', title: '显示农历信息', subtitle: '在日历视图中显示农历与节气' }
] as const;
function setWeek(event: Event): void { settings.setCalendarPreference('calendarWeekStart', Number((event.target as HTMLSelectElement).value)); }
</script>
<style scoped>
.cy-preferences{padding:5px 16px!important}.cy-preference{display:flex;align-items:center;justify-content:space-between;gap:15px;padding:17px 0}.cy-preference>span{flex:1;min-width:0}.cy-preference strong{font-weight:500;font-size:17px}.cy-preference small,.cy-due-link small{display:block;color:var(--cy-muted);font-size:12px;line-height:1.6;margin-top:5px}.cy-preference select{border:0;background:transparent;color:var(--cy-muted);font-size:15px;padding:8px 0;max-width:84px}.cy-preference .toggle{flex:none;--f7-toggle-active-color:var(--cy-accent)}.cy-due-link{display:flex;justify-content:space-between;align-items:center;color:var(--cy-ink);font-size:15px}
.cy-preference-toggle{position:relative;flex:none;width:46px!important;height:27px;border:0;border-radius:20px;background:var(--cy-line);padding:0}.cy-preference-toggle::after{content:"";position:absolute;top:3px;left:3px;width:21px;height:21px;border-radius:50%;background:#fff;box-shadow:0 1px 3px #0002;transition:transform .15s}.cy-preference-toggle[aria-checked=true]{background:var(--cy-accent)}.cy-preference-toggle[aria-checked=true]::after{transform:translateX(19px)}
</style>
