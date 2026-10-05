<template>
    <f7-page class="cy-main-page cy-mobile-surface" @page:afterin="refreshProfile">
        <f7-navbar title="我的"><f7-nav-right><f7-link href="/user/profile" aria-label="编辑个人资料" icon-f7="person_crop_circle_badge_checkmark" /></f7-nav-right></f7-navbar>

        <main class="cy-page-body cy-profile-body">
            <f7-link href="/user/profile" class="cy-panel cy-hero cy-profile-card"><img v-if="userStore.currentUserAvatar" :src="userStore.currentUserAvatar" alt="个人头像" /><span v-else class="cy-avatar"><f7-icon f7="person_fill" /></span><h1>{{ currentNickName }}</h1><p>把每一笔生活，记在心里。</p><div class="cy-profile-counts"><span>已保存 <strong>{{ dataStats?.totalTransactionCount ?? '—' }}</strong> 条账务记录</span><span><strong>{{ dataStats?.totalAccountCount ?? '—' }}</strong> 个日常账户</span></div></f7-link>
            <p v-if="profileError" class="cy-message" role="alert">{{ profileError }} <button @click="refreshProfile">重试</button></p>
            <nav class="cy-panel cy-feature-grid" aria-label="账本管理"><f7-link href="/template/list"><f7-icon f7="doc_on_doc" />记账模板</f7-link><f7-link v-if="isUserScheduledTransactionEnabled()" href="/schedule/list"><f7-icon f7="clock" />周期记账</f7-link><f7-link href="/account/list"><f7-icon f7="book" />账户管理</f7-link><f7-link href="/investments/ledger"><f7-icon f7="chart_bar" />投资理财</f7-link><f7-link href="/tag/list"><f7-icon f7="tag" />标签管理</f7-link><f7-link href="/category/all"><f7-icon f7="square_grid_2x2" />分类管理</f7-link><f7-link href="/books"><f7-icon f7="book_closed" />账本管理</f7-link><button @click="openMoreSettings"><f7-icon f7="gear_alt" />更多设置</button></nav>
            <section class="cy-panel cy-profile-links"><h2>数据管理</h2><f7-link href="/user/data/management"><f7-icon f7="tray_arrow_down" /><span>数据管理与导出<small>账单数据、图片与清理</small></span><f7-icon f7="chevron_right" size="14" /></f7-link><f7-link href="/transaction/list?view=pictures"><f7-icon f7="photo_on_rectangle" /><span>账单图片</span><f7-icon f7="chevron_right" size="14" /></f7-link><f7-link href="/settings/sync"><f7-icon f7="cloud" /><span>设置同步<small>同步应用偏好设置</small></span><f7-icon f7="chevron_right" size="14" /></f7-link></section>
            <section class="cy-panel cy-profile-links"><h2>外观与安全</h2><button @click="showThemePopup = true"><f7-icon f7="paintbrush" /><span>主题外观<small>{{ findNameByValue(allThemes,currentTheme) }}</small></span><f7-icon f7="chevron_right" size="14" /></button><f7-link v-if="!nativePersonal" href="/user/2fa"><f7-icon f7="lock_shield" /><span>双重认证</span><f7-icon f7="chevron_right" size="14" /></f7-link><f7-link v-if="!nativePersonal" href="/user/sessions"><f7-icon f7="device_phone_portrait" /><span>登录设备与会话</span><f7-icon f7="chevron_right" size="14" /></f7-link></section>
        </main>

        <div ref="moreSettingsAnchor" /><f7-block-title><f7-link @click="showMoreSettings = !showMoreSettings">更多设置 {{ showMoreSettings ? '⌃' : '⌄' }}</f7-link></f7-block-title>
        <f7-list strong inset dividers class="settings-list" v-show="showMoreSettings">
            <f7-list-item
                link="#"
                :title="tt('Theme')"
                :after="findNameByValue(allThemes, currentTheme)"
                @click="showThemePopup = true"
            >
                <list-item-selection-popup value-type="item"
                                           key-field="value" value-field="value"
                                           title-field="name"
                                           :title="tt('Theme')"
                                           :enable-filter="true"
                                           :filter-placeholder="tt('Theme')"
                                           :filter-no-items-text="tt('No results')"
                                           :items="allThemes"
                                           v-model:show="showThemePopup"
                                           v-model="currentTheme">
                </list-item-selection-popup>
            </f7-list-item>

            <f7-list-item :title="tt('Text Size')" link="/settings/textsize"></f7-list-item>

            <f7-list-item
                class="item-truncate-after-text"
                link="#"
                @click="showTimezonePopup = true"
            >
                <template #after-title>
                    <div class="item-actual-title">
                        <span>{{ tt('Timezone') }}</span>
                    </div>
                </template>
                <template #after>
                    {{ currentTimezoneName }}
                </template>
                <list-item-selection-popup value-type="item"
                                           key-field="name" value-field="name"
                                           title-field="displayNameWithUtcOffset"
                                           :title="tt('Timezone')"
                                           :enable-filter="true"
                                           :filter-placeholder="tt('Timezone')"
                                           :filter-no-items-text="tt('No results')"
                                           :items="allTimezones"
                                           v-model:show="showTimezonePopup"
                                           v-model="timeZone">
                </list-item-selection-popup>
            </f7-list-item>

            <f7-list-item v-if="!nativePersonal" :title="tt('Application Lock')" :after="isEnableApplicationLock ? tt('Enabled') : tt('Disabled')" link="/app_lock"></f7-list-item>

            <f7-list-item :title="tt('Exchange Rates Data')" :after="exchangeRatesLastUpdateDate" link="/exchange_rates"></f7-list-item>

            <f7-list-item :title="tt('Preferences')" link="/settings/preferences"></f7-list-item>
            <f7-list-item :title="tt('Statistics Settings')" link="/statistic/settings"></f7-list-item>
            <f7-list-item :title="tt('Settings Sync')" link="/settings/sync"></f7-list-item>

            <f7-list-item>
                <template #after-title>
                    {{ tt('Enable Swipe Back') }}
                </template>
                <template #after>
                    <f7-toggle :checked="isEnableSwipeBack" @toggle:change="isEnableSwipeBack = $event"></f7-toggle>
                </template>
            </f7-list-item>

            <f7-list-item>
                <template #after-title>
                    {{ tt('Enable Animation') }}
                </template>
                <template #after>
                    <f7-toggle :checked="isEnableAnimate" @toggle:change="isEnableAnimate = $event"></f7-toggle>
                </template>
            </f7-list-item>

            <f7-list-item :title="tt('Browser Cache Management')" link="/settings/browser_caches"></f7-list-item>
            <f7-list-item v-if="!nativePersonal" link="#" no-chevron :title="tt('Switch to Desktop Version')" @click="switchToDesktopVersion"></f7-list-item>

            <f7-list-item :title="tt('About')" link="/about" :after="version"></f7-list-item>
        </f7-list>
        <f7-list v-if="!nativePersonal" strong inset><f7-list-button :class="{ 'disabled': logouting }" @click="logout">{{ tt('Log Out') }}</f7-list-button></f7-list>
    <template #fixed><LedgerNavigation active="settings" /></template>
    </f7-page>
</template>

<script setup lang="ts">
import LedgerNavigation from '@/components/mobile/LedgerNavigation.vue';
import { isNativePersonalMode } from '@/lib/native.ts';
import { ref, computed, nextTick } from 'vue';
import type { DataStatisticsResponse } from '@/models/data_management.ts';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useAppSettingPageBase } from '@/views/base/settings/AppSettingsPageBase.ts';

import { useRootStore } from '@/stores/index.ts';
import { useSettingsStore } from '@/stores/setting.ts';
import { useUserStore } from '@/stores/user.ts';
import { useExchangeRatesStore } from '@/stores/exchangeRates.ts';

import { findNameByValue } from '@/lib/common.ts';
import { parseDateTimeFromUnixTime } from '@/lib/datetime.ts';
import { getClientDisplayVersion, getDesktopVersionPath } from '@/lib/version.ts';
import { isUserScheduledTransactionEnabled } from '@/lib/server_settings.ts';
import { setExpenseAndIncomeAmountColor } from '@/lib/ui/common.ts';

const props = defineProps<{
    f7router: Router.Router;
}>();

const { tt, formatDateTimeToLongDate, initLocale } = useI18n();
const { showToast, showConfirm } = useI18nUIComponents();
const { allThemes, allTimezones, timeZone } = useAppSettingPageBase();

const rootStore = useRootStore();
const settingsStore = useSettingsStore();
const userStore = useUserStore();
const exchangeRatesStore = useExchangeRatesStore();

const version = `${getClientDisplayVersion()}`;
const nativePersonal = isNativePersonalMode();

const logouting = ref<boolean>(false);
const showThemePopup = ref<boolean>(false);
const showTimezonePopup = ref<boolean>(false);
const showMoreSettings = ref(false);
const moreSettingsAnchor = ref<HTMLElement>();
async function openMoreSettings(): Promise<void> {
    showMoreSettings.value = true;
    await nextTick();
    moreSettingsAnchor.value?.scrollIntoView({ block: 'start' });
}
const dataStats = ref<DataStatisticsResponse>();
const profileError = ref('');
async function refreshProfile(): Promise<void> {
    profileError.value = '';
    try { dataStats.value = await userStore.getUserDataStatistics(); }
    catch { profileError.value = '暂时无法加载账本统计。'; }
}

const currentNickName = computed<string>(() => userStore.currentUserNickname || tt('User'));

const currentTheme = computed<string>({
    get: () => settingsStore.appSettings.theme,
    set: value => {
        if (value !== settingsStore.appSettings.theme) {
            settingsStore.setTheme(value);
            location.reload();
        }
    }
});

const currentTimezoneName = computed<string>(() => {
    for (const item of allTimezones.value) {
        if (item.name === timeZone.value) {
            return item.displayNameWithUtcOffset;
        }
    }

    return '';
});

const isEnableSwipeBack = computed<boolean>({
    get: () => settingsStore.appSettings.swipeBack,
    set: value => {
        if (value !== settingsStore.appSettings.swipeBack) {
            settingsStore.setEnableSwipeBack(value);
            location.reload();
        }
    }
});

const isEnableAnimate = computed<boolean>({
    get: () => settingsStore.appSettings.animate,
    set: value => {
        if (value !== settingsStore.appSettings.animate) {
            settingsStore.setEnableAnimate(value);
            location.reload();
        }
    }
});

const isEnableApplicationLock = computed<boolean>(() => settingsStore.appSettings.applicationLock);

const exchangeRatesLastUpdateDate = computed<string>(() => {
    if (!exchangeRatesStore.exchangeRatesLastUpdateTime) {
        return '';
    }

    const exchangeRatesLastUpdateTime = parseDateTimeFromUnixTime(exchangeRatesStore.exchangeRatesLastUpdateTime);
    return formatDateTimeToLongDate(exchangeRatesLastUpdateTime);
});

function switchToDesktopVersion(): void {
    showConfirm('Are you sure you want to switch to desktop version?', () => {
        window.location.replace(getDesktopVersionPath());
    });
}

function logout(): void {
    showConfirm('Are you sure you want to log out?', () => {
        logouting.value = true;
        showLoading(() => logouting.value);

        rootStore.logout().then(() => {
            logouting.value = false;
            hideLoading();

            settingsStore.clearAppSettings();

            const localeDefaultSettings = initLocale(userStore.currentUserLanguage, settingsStore.appSettings.timeZone);
            settingsStore.updateLocalizedDefaultSettings(localeDefaultSettings);

            setExpenseAndIncomeAmountColor(userStore.currentUserExpenseAmountColor, userStore.currentUserIncomeAmountColor);

            props.f7router.navigate('/');
        }).catch(error => {
            logouting.value = false;
            hideLoading();

            if (!error.processed) {
                showToast(error.message || error);
            }
        });
    });
}
</script>
<style scoped>
.cy-profile-body{padding-bottom:0!important}.cy-profile-card{display:flex;flex-direction:column;align-items:center;text-align:center;padding:26px 16px!important}.cy-profile-card img,.cy-avatar{width:62px;height:62px;border-radius:50%;object-fit:cover;background:#dcece6;color:#145d58;display:grid;place-items:center;margin-bottom:16px}.cy-avatar .icon{font-size:30px}.cy-profile-card h1{font-size:22px;font-weight:650}.cy-profile-card>p{font-size:12px;color:#ffffffb0;margin:10px 0 20px}.cy-profile-counts{display:flex;gap:18px;font-size:11px;color:#ffffffb0;flex-wrap:wrap;justify-content:center}.cy-profile-counts strong{color:white;font-size:16px}.cy-feature-grid{display:grid;grid-template-columns:repeat(4,minmax(0,1fr));gap:26px 8px;padding:24px 12px!important}.cy-feature-grid a,.cy-feature-grid button{display:flex;flex-direction:column;align-items:center;gap:12px;font-size:12px;color:var(--cy-ink);border:0;background:transparent;padding:0;white-space:nowrap}.cy-feature-grid .icon{color:var(--cy-accent);font-size:25px}.cy-profile-links h2{font-size:12px;color:var(--cy-muted);font-weight:500;margin-bottom:8px}.cy-profile-links>a,.cy-profile-links>button{display:flex;align-items:center;gap:15px;color:var(--cy-ink);width:100%;text-align:left;padding:17px 0;border:0;border-bottom:1px solid var(--cy-line);background:transparent}.cy-profile-links>a:last-child,.cy-profile-links>button:last-child{border-bottom:0;padding-bottom:3px}.cy-profile-links .icon{color:var(--cy-accent);font-size:23px}.cy-profile-links span{flex:1;font-size:15px}.cy-profile-links small{display:block;font-size:11px;color:var(--cy-muted);margin-top:5px}
</style>
