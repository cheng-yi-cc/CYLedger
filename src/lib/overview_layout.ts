import { type PartialRecord, entries, keys } from '@/core/base.ts';
import {
    type OverviewWidgetSettingValue,
    type OverviewWidgetSettingItem,
    type OverviewWidgetDefinitionBase,
    type OverviewRecentTransactionsQuery,
    type OverviewLayoutBase,
    type OverviewWidgetLayoutBase,
    type MobileOverviewLayout,
    type MobileOverviewWidgetLayout,
    OverviewWidgetType,
    OverviewWidgetDataRequirement
} from '@/core/overview_layout.ts';

import {
    MOBILE_OVERVIEW_LAYOUT_MAX_WIDGETS,
    MOBILE_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_MOBILE_OVERVIEW_LAYOUT,
    RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT
} from '@/consts/overview_layout.ts';

import { AmountFilterType } from '@/core/numeral.ts';
import { TransactionTagFilter } from '@/models/transaction.ts';

import {
    isDefined,
    isObject,
    isArray,
    isString,
    isNumber,
    isBoolean,
    isInteger,
    isHextualColor,
} from '@/lib/common.ts';

function normalizeOverviewWidgetSettings(definition: OverviewWidgetDefinitionBase, settings: unknown): Record<string, OverviewWidgetSettingValue> {
    const normalized = { ...definition.defaultSettings };
    const source: Record<string, unknown> = isObject(settings) ? settings as Record<string, unknown> : {};

    for (const setting of definition.supportsSettings) {
        const sourceValue = source[setting.settingName];
        const normalizedValue = normalizeOverviewWidgetSetting(setting, sourceValue);

        if (isDefined(normalizedValue) &&
            ((isString(normalizedValue) && normalizedValue.trim() !== '' && normalizedValue.trim() !== normalized[setting.settingName])
                || isNumber(normalizedValue)
                || isBoolean(normalizedValue)
                || isArray(normalizedValue)
            )) {
            normalized[setting.settingName] = normalizedValue;
        }
    }

    return normalized;
}

function normalizeOverviewWidgetSetting(setting: OverviewWidgetSettingItem, value: unknown): OverviewWidgetSettingValue | undefined {
    if (setting.settingType === 'itemCountSelect') {
        return isInteger(value) && setting.itemCountValues.includes(value) ? value : undefined;
    } else if (setting.settingType === 'monthSelect') {
        return isInteger(value) && setting.monthValues.includes(value) ? value : undefined;
    } else if (setting.settingType === 'accountSelect') {
        return isArray(value) ? value.filter(id => isString(id)) : undefined;
    } else if (setting.settingType === 'categorySelect') {
        return isArray(value) ? value.filter(id => isString(id)) : undefined;
    } else if (setting.settingType === 'tagSelect') {
        return isString(value) && TransactionTagFilter.parse(value) ? value : undefined;
    } else if (setting.settingType === 'customSelect') {
        if (!setting.multiple) {
            return (isString(value) || isNumber(value)) && setting.selectValues.some(item => item.value === value) ? value : undefined;
        } else if (setting.multiple) {
            if (!isArray(value)) {
                return undefined;
            }

            const allowedValues: Record<string, boolean> = {};

            for (const item of setting.selectValues) {
                allowedValues[item.value] = true;
            }

            const selectedValues: (string | number)[] = [];
            const selectedValuesMap: Record<string, boolean> = {};

            for (const item of value) {
                if (isDefined(setting.allValue) && item === setting.allValue) {
                    return [setting.allValue];
                }

                if ((isString(item) || isNumber(item)) && allowedValues[item] && !selectedValuesMap[item]) {
                    selectedValues.push(item);
                    selectedValuesMap[item] = true;
                }
            }

            return selectedValues.length >= (setting.minSelections ?? 1) ? selectedValues : undefined;
        } else {
            return undefined;
        }
    } else if (setting.settingType === 'switch') {
        return isBoolean(value) ? value : undefined;
    } else if (setting.settingType === 'color') {
        return isHextualColor(value) ? value.toLowerCase() : undefined;
    } else if (setting.settingType === 'amount') {
        return isString(value) && AmountFilterType.parseTextualFilter(value) ? value : undefined;
    } else if (setting.settingType === 'textbox') {
        return isString(value) ? value : undefined;
    } else {
        return undefined;
    }
}

function getMaximumWidgetMonths(layout: OverviewLayoutBase, type: OverviewWidgetType): number {
    let months: number = 6;

    for (const widget of layout.widgets) {
        if (widget.type === type) {
            const monthsValue = widget.settings['months'];

            if (isInteger(monthsValue)) {
                months = Math.max(months, monthsValue);
            }
        }
    }

    return months;
}

export function cloneWidget<T extends OverviewWidgetLayoutBase>(widget: T): T {
    const settings: Record<string, OverviewWidgetSettingValue> = {};

    for (const [key, value] of entries(widget.settings)) {
        settings[key] = isArray(value) ? [...value] : value;
    }

    return { ...widget, settings };
}

export function getOverviewDataRequirements(layout: OverviewLayoutBase, definitions: PartialRecord<OverviewWidgetType, OverviewWidgetDefinitionBase>): OverviewWidgetDataRequirement[] {
    const requirements: Record<string, boolean> = {};
    const result: OverviewWidgetDataRequirement[] = [];

    for (const widget of layout.widgets) {
        const definition = definitions[widget.type];

        if (!definition) {
            continue;
        }

        for (const requirement of definition.dataRequirements) {
            requirements[requirement] = true;
        }
    }

    if (requirements[OverviewWidgetDataRequirement.TransactionOverviewLast12Months]) {
        requirements[OverviewWidgetDataRequirement.TransactionOverview] = true;
    }

    if (requirements[OverviewWidgetDataRequirement.TransactionOverviewLast2Months]) {
        requirements[OverviewWidgetDataRequirement.TransactionOverview] = true;
    }

    for (const requirement of keys(requirements)) {
        if (requirements[requirement]) {
            result.push(requirement as OverviewWidgetDataRequirement);
        }
    }

    return result;
}

export function getOverviewTransactionOverviewMonths(layout: OverviewLayoutBase): number {
    let months: number = 1;

    for (const widget of layout.widgets) {
        if (widget.type === OverviewWidgetType.IncomeExpenseTrend) {
            const monthsValue = widget.settings['months'];

            if (isInteger(monthsValue)) {
                months = Math.max(months, monthsValue);
            }
        } else if (widget.type === OverviewWidgetType.CurrentMonthExpenseProgress) {
            months = Math.max(months, 2);
        }
    }

    return months;
}

export function getOverviewRecentTransactionsQuery(settings: Record<string, OverviewWidgetSettingValue>): OverviewRecentTransactionsQuery {
    return {
        count: isInteger(settings['itemCount']) ? settings['itemCount'] : RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT,
        accountIds: isArray(settings['accountIds']) ? settings['accountIds'] as string[] : [],
        categoryIds: isArray(settings['categoryIds']) ? settings['categoryIds'] as string[] : [],
        tagFilter: isString(settings['tagFilter']) ? settings['tagFilter'] : '',
        amountFilter: isString(settings['amountFilter']) ? settings['amountFilter'] : '',
        keyword: isString(settings['keyword']) ? settings['keyword'] : ''
    };
}

export function getOverviewRecentTransactionsQueries(layout: OverviewLayoutBase): Record<string, OverviewRecentTransactionsQuery> {
    const queries: Record<string, OverviewRecentTransactionsQuery> = {};

    for (const widget of layout.widgets) {
        if (widget.type === OverviewWidgetType.RecentTransactions) {
            queries[widget.id] = getOverviewRecentTransactionsQuery(widget.settings);
        }
    }

    return queries;
}

export function getOverviewAssetTrendMonths(layout: OverviewLayoutBase): number {
    return getMaximumWidgetMonths(layout, OverviewWidgetType.NetAssetsTrend);
}

export function getOverviewCalendarHeatmapMonths(layout: OverviewLayoutBase): number {
    return getMaximumWidgetMonths(layout, OverviewWidgetType.TransactionCalendarHeatmap);
}

export function getOverviewTransactionCategoryStatisticDateTypes(layout: OverviewLayoutBase): number[] {
    const dateTypes: number[] = [];
    const existingDateTypes: Record<number, boolean> = {};

    for (const widget of layout.widgets) {
        if (widget.type !== OverviewWidgetType.ExpenseCategoryRanking) {
            continue;
        }

        const dateType = widget.settings['dateRange'];

        if (isNumber(dateType) && !existingDateTypes[dateType]) {
            dateTypes.push(dateType);
            existingDateTypes[dateType] = true;
        }
    }

    return dateTypes;
}

export function isDefaultMobileOverviewLayout(layout: MobileOverviewLayout): boolean {
    return serializeMobileOverviewLayout(layout) === serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
}

export function normalizeMobileOverviewLayout(input: unknown): MobileOverviewLayout {
    if (!isObject(input)) {
        throw new Error('input is not an object');
    }

    const source = input as Record<string, unknown>;
    const sourceWidgets = source['widgets'];

    if (!isArray(sourceWidgets)) {
        throw new Error('widgets is not an array');
    }

    const finalWidgets: MobileOverviewWidgetLayout[] = [];
    const existsIds: Record<string, boolean> = {};

    for (const item of sourceWidgets) {
        if (!isObject(item)) {
            continue;
        }

        if (finalWidgets.length >= MOBILE_OVERVIEW_LAYOUT_MAX_WIDGETS) {
            break;
        }

        const sourceWidget = item as Record<string, unknown>;
        const type = sourceWidget['type'] as OverviewWidgetType;
        const definition = MOBILE_OVERVIEW_WIDGET_DEFINITIONS[type];
        const widgetId = sourceWidget['id'];

        if (!definition || !widgetId || !isString(widgetId) || widgetId.length > 100 || existsIds[widgetId]) {
            continue;
        }

        const finalWidget: MobileOverviewWidgetLayout = {
            id: widgetId,
            type: type,
            settings: normalizeOverviewWidgetSettings(definition, sourceWidget['settings'])
        }

        existsIds[widgetId] = true;
        finalWidgets.push(finalWidget);
    }

    return {
        widgets: finalWidgets
    };
}

export function cloneMobileOverviewLayout(original: MobileOverviewLayout): MobileOverviewLayout {
    return {
        widgets: original.widgets.map(cloneWidget)
    };
}

export function parseMobileOverviewLayout(value: string): MobileOverviewLayout {
    if (!value) {
        return cloneMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
    }

    return normalizeMobileOverviewLayout(JSON.parse(value));
}

export function serializeMobileOverviewLayout(layout: MobileOverviewLayout, pretty?: boolean): string {
    const normalized = normalizeMobileOverviewLayout(layout);
    return JSON.stringify(normalized, null, pretty ? 2 : undefined);
}
