import { describe, expect, test } from 'vitest';

import { OverviewWidgetType, OverviewWidgetDataRequirement } from '@/core/overview_layout.ts';
import { DateRange } from '@/core/datetime.ts';

import {
    MOBILE_OVERVIEW_WIDGET_DEFINITIONS,
    DEFAULT_MOBILE_OVERVIEW_LAYOUT
} from '@/consts/overview_layout.ts';

import {
    getOverviewDataRequirements,
    isDefaultMobileOverviewLayout,
    normalizeMobileOverviewLayout,
    parseMobileOverviewLayout,
    serializeMobileOverviewLayout
} from '../overview_layout.ts';

describe('mobile overview layout', () => {
    test('keeps the single-column order and ignores duplicate widgets', () => {
        const layout = normalizeMobileOverviewLayout({
            widgets: [
                { id: 'period', type: OverviewWidgetType.PeriodIncomeExpense, settings: { invalid: true } },
                { id: 'asset-summary', type: OverviewWidgetType.AssetSummary },
                { id: 'accounts', type: OverviewWidgetType.AccountBalanceList },
                { id: 'month', type: OverviewWidgetType.CurrentMonthOverview },
                { id: 'period', type: OverviewWidgetType.CurrentMonthOverview }
            ]
        });

        expect(layout.widgets).toEqual([
            {
                id: 'period',
                type: OverviewWidgetType.PeriodIncomeExpense,
                settings: { dateRanges: [DateRange.Today.type, DateRange.ThisWeek.type, DateRange.ThisMonth.type, DateRange.ThisYear.type] }
            },
            {
                id: 'asset-summary',
                type: OverviewWidgetType.AssetSummary,
                settings: { height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' }
            },
            {
                id: 'accounts',
                type: OverviewWidgetType.AccountBalanceList,
                settings: { accountIds: [], itemCount: 4, showTitle: false, sortBy: 'displayOrder', alwaysShowAmount: false, showAvailableCreditForCreditCard: false }
            },
            {
                id: 'month',
                type: OverviewWidgetType.CurrentMonthOverview,
                settings: { height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' }
            }
        ]);
    });

    test('normalizes mobile widget settings', () => {
        const layout = normalizeMobileOverviewLayout({
            widgets: [
                { id: 'small-month', type: OverviewWidgetType.CurrentMonthOverview, settings: { height: 1, lightBackgroundColor: '112233', darkBackgroundColor: 'abcdef' } },
                { id: 'invalid-month', type: OverviewWidgetType.CurrentMonthOverview, settings: { height: 99, lightBackgroundColor: 'invalid', darkBackgroundColor: '#ffffff' } },
                { id: 'selected-periods', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRanges: [DateRange.ThisYear.type, DateRange.Today.type] } },
                { id: 'empty-periods', type: OverviewWidgetType.PeriodIncomeExpense, settings: { dateRanges: [] } }
            ]
        });

        expect(layout.widgets[0]?.settings).toEqual({ height: 1, lightBackgroundColor: '112233', darkBackgroundColor: 'abcdef' });
        expect(layout.widgets[1]?.settings).toEqual({ height: 3, lightBackgroundColor: 'edddcd', darkBackgroundColor: '7f5e4b' });
        expect(layout.widgets[2]?.settings).toEqual({ dateRanges: [DateRange.ThisYear.type, DateRange.Today.type] });
        expect(layout.widgets[3]?.settings).toEqual({ dateRanges: [DateRange.Today.type, DateRange.ThisWeek.type, DateRange.ThisMonth.type, DateRange.ThisYear.type] });
    });

    test('round trips layout and collects registered data requirements', () => {
        const json = serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT, true);
        const layout = parseMobileOverviewLayout(json);

        expect(serializeMobileOverviewLayout(layout)).toBe(serializeMobileOverviewLayout(DEFAULT_MOBILE_OVERVIEW_LAYOUT));
        expect(getOverviewDataRequirements(layout, MOBILE_OVERVIEW_WIDGET_DEFINITIONS)).toEqual([OverviewWidgetDataRequirement.TransactionOverview]);
    });

    test('empty setting uses a cloned default layout', () => {
        const layout = parseMobileOverviewLayout('');

        expect(isDefaultMobileOverviewLayout(layout)).toBe(true);
        expect(layout).not.toBe(DEFAULT_MOBILE_OVERVIEW_LAYOUT);
        expect(layout.widgets).not.toBe(DEFAULT_MOBILE_OVERVIEW_LAYOUT.widgets);
    });
});
