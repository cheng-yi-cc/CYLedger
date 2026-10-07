import { ref, computed } from 'vue';

import { useSettingsStore } from '@/stores/setting.ts';

import { keysIfValueEquals } from '@/core/base.ts';
import type { ApplicationCloudSetting } from '@/core/setting.ts';

export interface CategorizedApplicationCloudSettingItems {
    readonly categoryName: string;
    readonly categorySubName?: string;
    readonly items: ApplicationCloudSettingItem[];
}

export interface ApplicationCloudSettingItem {
    readonly settingKey: string;
    readonly relatedSettingKeys?: string[];
    readonly settingName: string;
    readonly mobile: boolean;
}

export const ALL_APPLICATION_CLOUD_SETTINGS: CategorizedApplicationCloudSettingItems[] = [
    {
        categoryName: 'General Settings',
        items: [
            { settingKey: 'showAccountBalance', settingName: 'Show Account Balance', mobile: true },
            { settingKey: 'accountCategoryOrders', settingName: 'Account Category Order', mobile: true },
            { settingKey: 'chartColors', settingName: 'Chart Color Scheme', mobile: true },
            { settingKey: 'autoUpdateExchangeRatesData', settingName: 'Auto-update Exchange Rates Data', mobile: true }
        ]
    },
    {
        categoryName: 'Overview Page',
        items: [
            { settingKey: 'mobileOverviewPageLayout', settingName: 'Mobile Home Page Layout', mobile: true },
            { settingKey: 'showAmountInHomePage', settingName: 'Show Amount', mobile: true },
            { settingKey: 'timezoneUsedForStatisticsInHomePage', settingName: 'Timezone Used for Statistics', mobile: true },
            { settingKey: 'overviewAccountFilterInHomePage', settingName: 'Accounts Included in Overview Statistics', mobile: true },
            { settingKey: 'overviewTransactionCategoryFilterInHomePage', settingName: 'Transaction Categories Included in Overview Statistics', mobile: true }
        ]
    },
    {
        categoryName: 'Transaction List Page',
        items: [
            { settingKey: 'showTotalAmountInTransactionListPage', settingName: 'Show Monthly Total Amount', mobile: true },
            { settingKey: 'showTagInTransactionListPage', settingName: 'Show Transaction Tags', mobile: true },
            { settingKey: 'defaultKeywordMatchModeInTransactionListPage', settingName: 'Default Keyword Search Matching Mode', mobile: true }
        ]
    },
    {
        categoryName: 'Transaction Edit Page',
        items: [
            { settingKey: 'quickSaveButtonStyleInMobileTransactionListPage', settingName: 'Quick Save Button Style', mobile: true },
            { settingKey: 'quickAddButtonActionInMobileTransactionEditPage', settingName: 'Quick Add Button Action', mobile: true },
            { settingKey: 'autoSaveTransactionDraft', settingName: 'Automatically Save Draft', mobile: true },
            { settingKey: 'autoGetCurrentGeoLocation', settingName: 'Automatically Add Geolocation', mobile: true },
            { settingKey: 'alwaysShowTransactionPicturesInMobileTransactionEditPage', settingName: 'Always Show Transaction Pictures', mobile: true },
            { settingKey: 'transactionPictureQuality', settingName: 'Transaction Picture Upload Quality', mobile: true }
        ]
    },
    {
        categoryName: 'AI Clipboard Text Recognition',
        items: [
            { settingKey: 'alwaysRequireConfirmationOfClipboardContentBeforeSubmission', settingName: 'Always Require Confirmation of Clipboard Content Before Submission', mobile: true }
        ]
    },
    {
        categoryName: 'AI Image Recognition',
        items: [
            { settingKey: 'autoUploadTransactionPictureForAIRecognition', settingName: 'Auto Upload AI Recognition Image as Transaction Picture', mobile: true }
        ]
    },
    {
        categoryName: 'Account List Page',
        items: [
            { settingKey: 'totalAmountExcludeAccountIds', settingName: 'Accounts Included in Total', mobile: true },
            { settingKey: 'defaultCreditCardAmountDisplayTypeInMobile', settingName: 'Default Credit Card Amount', mobile: true },
            { settingKey: 'reconciliationStatementPageDefaultDateRangeTypeInMobile', settingName: 'Default Date Range for Reconciliation Statement Page', mobile: true }
        ]
    },
    {
        categoryName: 'Exchange Rates Data Page',
        items: [
            { settingKey: 'currencySortByInExchangeRatesPage', settingName: 'Sort by', mobile: true }
        ]
    },
    {
        categoryName: 'Browser Cache Management',
        items: [
            { settingKey: 'mapCacheExpiration', settingName: 'Cache Expiration for Map Data', mobile: true },
            { settingKey: 'exchangeRatesDataCacheExpiration', settingName: 'Cache Expiration for Exchange Rates Data', mobile: true }
        ]
    },
    {
        categoryName: 'Statistics Settings',
        categorySubName: 'Common Settings',
        items: [
            { settingKey: 'statistics.defaultChartDataType', settingName: 'Default Chart Data Type', mobile: true },
            { settingKey: 'statistics.defaultTimezoneType', settingName: 'Timezone Used for Date Range', mobile: true },
            { settingKey: 'statistics.defaultAccountFilter', settingName: 'Default Account Filter', mobile: true },
            { settingKey: 'statistics.defaultTransactionCategoryFilter', settingName: 'Default Transaction Category Filter', mobile: true },
            { settingKey: 'statistics.defaultKeywordMatchMode', settingName: 'Default Keyword Search Matching Mode', mobile: true },
            { settingKey: 'statistics.defaultSortingType', settingName: 'Default Sort Order', mobile: true }
        ]
    },
    {
        categoryName: 'Statistics Settings',
        categorySubName: 'Categorical Analysis Settings',
        items: [
            { settingKey: 'statistics.defaultCategoricalChartType', settingName: 'Default Chart Type', mobile: true },
            { settingKey: 'statistics.defaultCategoricalChartDataRangeType', settingName: 'Default Date Range', mobile: true }
        ]
    },
    {
        categoryName: 'Statistics Settings',
        categorySubName: 'Trend Analysis Settings',
        items: [
            { settingKey: 'statistics.defaultTrendChartDataRangeType', settingName: 'Default Date Range', mobile: true }
        ]
    },
    {
        categoryName: 'Statistics Settings',
        categorySubName: 'Asset Trends Settings',
        items: [
            { settingKey: 'statistics.defaultAssetTrendsChartDataRangeType', settingName: 'Default Date Range', mobile: true }
        ]
    }
];

export function useAppCloudSyncBase() {
    const settingsStore = useSettingsStore();

    const loading = ref<boolean>(false);
    const enabling = ref<boolean>(false);
    const disabling = ref<boolean>(false);
    const enabledApplicationCloudSettings = ref<Record<string, boolean>>(Object.assign({}, settingsStore.syncedAppSettings));

    const isEnableCloudSync = computed<boolean>(() => settingsStore.enableApplicationCloudSync);

    const hasEnabledApplicationCloudSettings = computed<boolean>(() => {
        for (const _ of keysIfValueEquals(enabledApplicationCloudSettings.value, true)) {
            return true;
        }

        return false;
    });

    const enabledApplicationCloudSettingKeys = computed<string[]>(() => {
        const keys: string[] = [];

        for (const key of keysIfValueEquals(enabledApplicationCloudSettings.value, true)) {
            keys.push(key);
        }

        return keys;
    });

    function isAllSettingsSelected(categorizedItems: CategorizedApplicationCloudSettingItems): boolean {
        for (const item of categorizedItems.items) {
            if (!enabledApplicationCloudSettings.value[item.settingKey]) {
                return false;
            }
        }

        return true;
    }

    function hasSettingSelectedButNotAllChecked(categorizedItems: CategorizedApplicationCloudSettingItems): boolean {
        let checkedCount = 0;

        for (const item of categorizedItems.items) {
            if (!enabledApplicationCloudSettings.value[item.settingKey]) {
                checkedCount++;
            }
        }

        return checkedCount > 0 && checkedCount < categorizedItems.items.length;
    }

    function updateSettingsSelected(categorizedItems: CategorizedApplicationCloudSettingItems, value: boolean): void {
        for (const item of categorizedItems.items) {
            enabledApplicationCloudSettings.value[item.settingKey] = value;

            if (item.relatedSettingKeys) {
                for (const relatedKey of item.relatedSettingKeys) {
                    enabledApplicationCloudSettings.value[relatedKey] = value;
                }
            }
        }
    }

    function updateSettingSelected(settingItem: ApplicationCloudSettingItem, value: boolean): void {
        enabledApplicationCloudSettings.value[settingItem.settingKey] = value;

        if (settingItem.relatedSettingKeys) {
            for (const relatedKey of settingItem.relatedSettingKeys) {
                enabledApplicationCloudSettings.value[relatedKey] = value;
            }
        }
    }

    function selectAllSettings(): void {
        for (const categorizedItems of ALL_APPLICATION_CLOUD_SETTINGS) {
            for (const item of categorizedItems.items) {
                enabledApplicationCloudSettings.value[item.settingKey] = true;

                if (item.relatedSettingKeys) {
                    for (const relatedKey of item.relatedSettingKeys) {
                        enabledApplicationCloudSettings.value[relatedKey] = true;
                    }
                }
            }
        }
    }

    function selectNoneSettings(): void {
        for (const categorizedItems of ALL_APPLICATION_CLOUD_SETTINGS) {
            for (const item of categorizedItems.items) {
                enabledApplicationCloudSettings.value[item.settingKey] = false;

                if (item.relatedSettingKeys) {
                    for (const relatedKey of item.relatedSettingKeys) {
                        enabledApplicationCloudSettings.value[relatedKey] = false;
                    }
                }
            }
        }
    }

    function selectInvertSettings(): void {
        for (const categorizedItems of ALL_APPLICATION_CLOUD_SETTINGS) {
            for (const item of categorizedItems.items) {
                const newValue = !enabledApplicationCloudSettings.value[item.settingKey];
                enabledApplicationCloudSettings.value[item.settingKey] = newValue;

                if (item.relatedSettingKeys) {
                    for (const relatedKey of item.relatedSettingKeys) {
                        enabledApplicationCloudSettings.value[relatedKey] = newValue;
                    }
                }
            }
        }
    }

    function setUserApplicationCloudSettings(settings: ApplicationCloudSetting[] | false) {
        if (settings && settings.length > 0) {
            settingsStore.setApplicationSettingsFromCloudSettings(settings);

            for (const setting of settings) {
                if (setting && setting.settingKey) {
                    enabledApplicationCloudSettings.value[setting.settingKey] = true;
                }
            }
        } else {
            settingsStore.setApplicationSettingsFromCloudSettings(undefined);
            enabledApplicationCloudSettings.value = {};
        }
    }

    return {
        // constants
        ALL_APPLICATION_CLOUD_SETTINGS,
        // states
        loading,
        enabling,
        disabling,
        enabledApplicationCloudSettings,
        // computed states
        isEnableCloudSync,
        hasEnabledApplicationCloudSettings,
        enabledApplicationCloudSettingKeys,
        // functions
        isAllSettingsSelected,
        hasSettingSelectedButNotAllChecked,
        updateSettingsSelected,
        updateSettingSelected,
        selectAllSettings,
        selectNoneSettings,
        selectInvertSettings,
        setUserApplicationCloudSettings
    };
}
