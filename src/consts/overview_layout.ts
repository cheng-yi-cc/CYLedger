import type { PartialRecord } from '@/core/base.ts';
import { DateRange } from '@/core/datetime.ts';
import { TransactionType } from '@/core/transaction.ts';
import {
    type OverviewWidgetSwitchSettingItem,
    type OverviewWidgetColorSettingItem,
    type OverviewWidgetTextboxSettingItem,
    type MobileOverviewLayout,
    type MobileOverviewWidgetDefinition,
    OverviewWidgetType,
    OverviewWidgetDataRequirement
} from '@/core/overview_layout.ts';

import {
    DEFAULT_MOBILE_OVERVIEW_WIDGET_DARK_BACKGROUND_COLOR,
    DEFAULT_MOBILE_OVERVIEW_WIDGET_LIGHT_BACKGROUND_COLOR
} from '@/consts/color.ts';

export const MOBILE_OVERVIEW_LAYOUT_MAX_WIDGETS: number = 100;
export const RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT: number = 3;

const WIDGET_TITLE_SETTING: OverviewWidgetTextboxSettingItem = {
    settingType: 'textbox',
    settingName: 'title',
    displayName: 'Widget Title',
    placeholder: 'Widget Title'
};

const WIDGET_SHOW_TITLE_SETTING: OverviewWidgetSwitchSettingItem = {
    settingType: 'switch',
    settingName: 'showTitle',
    displayName: 'Show Title'
};

const WIDGET_BACKGROUND_COLOR_SETTINGS: OverviewWidgetColorSettingItem[] = [
    {
        settingType: 'color',
        settingName: 'lightBackgroundColor',
        displayName: 'Light Mode Background Color'
    },
    {
        settingType: 'color',
        settingName: 'darkBackgroundColor',
        displayName: 'Dark Mode Background Color'
    }
];

export const MOBILE_OVERVIEW_WIDGET_DEFINITIONS: PartialRecord<OverviewWidgetType, MobileOverviewWidgetDefinition> = {
    [OverviewWidgetType.AssetSummary]: {
        type: OverviewWidgetType.AssetSummary,
        name: 'Asset Summary',
        supportsSettings: [
            {
                settingType: 'customSelect',
                settingName: 'height',
                displayName: 'Widget Height',
                selectValues: [
                    { name: 'Small', value: 1 },
                    { name: 'Medium', value: 2 },
                    { name: 'Large', value: 3 }
                ]
            },
            ...WIDGET_BACKGROUND_COLOR_SETTINGS
        ],
        defaultSettings: {
            height: 3,
            lightBackgroundColor: DEFAULT_MOBILE_OVERVIEW_WIDGET_LIGHT_BACKGROUND_COLOR,
            darkBackgroundColor: DEFAULT_MOBILE_OVERVIEW_WIDGET_DARK_BACKGROUND_COLOR
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.Accounts
        ]
    },
    [OverviewWidgetType.AccountBalanceList]: {
        type: OverviewWidgetType.AccountBalanceList,
        name: 'Account Balance List',
        supportsSettings: [
            WIDGET_TITLE_SETTING,
            WIDGET_SHOW_TITLE_SETTING,
            {
                settingType: 'accountSelect',
                settingName: 'accountIds',
                displayName: 'Account',
                disableHiddenAccounts: true
            },
            {
                settingType: 'itemCountSelect',
                settingName: 'itemCount',
                displayName: 'Item Count',
                itemCountValues: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
            },
            {
                settingType: 'customSelect',
                settingName: 'sortBy',
                displayName: 'Sort By',
                selectValues: [
                    {
                        name: 'Display Order',
                        value: 'displayOrder'
                    },
                    {
                        name: 'Balance',
                        value: 'balance'
                    }
                ]
            },
            {
                settingType: 'switch',
                settingName: 'alwaysShowAmount',
                displayName: 'Always Show Amount'
            },
            {
                settingType: 'switch',
                settingName: 'showAvailableCreditForCreditCard',
                displayName: 'Show Available Credit for Credit Cards'
            }
        ],
        defaultSettings: {
            showTitle: false,
            accountIds: [],
            itemCount: 4,
            sortBy: 'displayOrder',
            alwaysShowAmount: false,
            showAvailableCreditForCreditCard: false
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.Accounts
        ]
    },
    [OverviewWidgetType.CurrentMonthOverview]: {
        type: OverviewWidgetType.CurrentMonthOverview,
        name: 'This Month\'s Income and Expense Overview',
        supportsSettings: [
            {
                settingType: 'customSelect',
                settingName: 'height',
                displayName: 'Widget Height',
                selectValues: [
                    { name: 'Small', value: 1 },
                    { name: 'Medium', value: 2 },
                    { name: 'Large', value: 3 }
                ]
            },
            ...WIDGET_BACKGROUND_COLOR_SETTINGS
        ],
        defaultSettings: {
            height: 3,
            lightBackgroundColor: DEFAULT_MOBILE_OVERVIEW_WIDGET_LIGHT_BACKGROUND_COLOR,
            darkBackgroundColor: DEFAULT_MOBILE_OVERVIEW_WIDGET_DARK_BACKGROUND_COLOR
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.TransactionOverview
        ]
    },
    [OverviewWidgetType.CurrentMonthExpenseProgress]: {
        type: OverviewWidgetType.CurrentMonthExpenseProgress,
        name: 'This Month\'s Expense Progress',
        supportsSettings: [
            WIDGET_TITLE_SETTING,
            WIDGET_SHOW_TITLE_SETTING
        ],
        defaultSettings: {
            showTitle: false
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.TransactionOverviewLast2Months
        ]
    },
    [OverviewWidgetType.PeriodIncomeExpense]: {
        type: OverviewWidgetType.PeriodIncomeExpense,
        name: 'Period Income and Expense',
        supportsSettings: [
            {
                settingType: 'customSelect',
                settingName: 'dateRanges',
                displayName: 'Date Range',
                selectValues: [
                    DateRange.Today,
                    DateRange.ThisWeek,
                    DateRange.ThisMonth,
                    DateRange.ThisYear
                ].map(dateRange => ({
                    name: dateRange.name,
                    value: dateRange.type
                })),
                multiple: true
            }
        ],
        defaultSettings: {
            dateRanges: [
                DateRange.Today.type,
                DateRange.ThisWeek.type,
                DateRange.ThisMonth.type,
                DateRange.ThisYear.type
            ]
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.TransactionOverview
        ]
    },
    [OverviewWidgetType.PeriodNetIncomeAndSavingsRate]: {
        type: OverviewWidgetType.PeriodNetIncomeAndSavingsRate,
        name: 'Period Net Income and Savings Rate',
        supportsSettings: [
            WIDGET_TITLE_SETTING,
            WIDGET_SHOW_TITLE_SETTING,
            {
                settingType: 'customSelect',
                settingName: 'dateRange',
                displayName: 'Date Range',
                selectValues: [
                    DateRange.Today,
                    DateRange.ThisWeek,
                    DateRange.ThisMonth,
                    DateRange.ThisYear
                ].map(dateRange => ({ name: dateRange.name, value: dateRange.type }))
            }
        ],
        defaultSettings: {
            showTitle: false,
            dateRange: DateRange.ThisMonth.type
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.TransactionOverview
        ]
    },
    [OverviewWidgetType.ExpenseCategoryRanking]: {
        type: OverviewWidgetType.ExpenseCategoryRanking,
        name: 'Expense Category Ranking',
        supportsSettings: [
            WIDGET_TITLE_SETTING,
            WIDGET_SHOW_TITLE_SETTING,
            {
                settingType: 'customSelect',
                settingName: 'dateRange',
                displayName: 'Date Range',
                selectValues: [
                    DateRange.ThisMonth,
                    DateRange.ThisYear
                ].map(dateRange => ({
                    name: dateRange.name,
                    value: dateRange.type
                }))
            },
            {
                settingType: 'customSelect',
                settingName: 'categoryLevel',
                displayName: 'Category Level',
                selectValues: [
                    {
                        name: 'Primary Category',
                        value: 'primary'
                    },
                    {
                        name: 'Secondary Category',
                        value: 'secondary'
                    }
                ]
            },
            {
                settingType: 'itemCountSelect',
                settingName: 'itemCount',
                displayName: 'Item Count',
                itemCountValues: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
            }
        ],
        defaultSettings: {
            showTitle: false,
            dateRange: DateRange.ThisMonth.type,
            categoryLevel: 'primary',
            itemCount: 4
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.Accounts,
            OverviewWidgetDataRequirement.TransactionCategories,
            OverviewWidgetDataRequirement.TransactionCategoryStatistics
        ]
    },
    [OverviewWidgetType.RecentTransactions]: {
        type: OverviewWidgetType.RecentTransactions,
        name: 'Recent Transactions',
        supportsSettings: [
            WIDGET_TITLE_SETTING,
            WIDGET_SHOW_TITLE_SETTING,
            {
                settingType: 'itemCountSelect',
                settingName: 'itemCount',
                displayName: 'Item Count',
                itemCountValues: [1, 2, 3, 4, 5, 6, 7, 8, 9, 10]
            },
            {
                settingType: 'accountSelect',
                settingName: 'accountIds',
                displayName: 'Account'
            },
            {
                settingType: 'categorySelect',
                settingName: 'categoryIds',
                displayName: 'Category'
            },
            {
                settingType: 'tagSelect',
                settingName: 'tagFilter',
                displayName: 'Tags'
            },
            {
                settingType: 'amount',
                settingName: 'amountFilter',
                displayName: 'Amount'
            },
            {
                settingType: 'textbox',
                settingName: 'keyword',
                displayName: 'Description',
                placeholder: 'Filter Description'
            }
        ],
        defaultSettings: {
            showTitle: false,
            itemCount: RECENT_TRANSACTIONS_WIDGET_DEFAULT_ITEM_COUNT,
            accountIds: [],
            categoryIds: [],
            tagFilter: '',
            amountFilter: '',
            keyword: ''
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.RecentTransactions
        ]
    },
    [OverviewWidgetType.TransactionCalendar]: {
        type: OverviewWidgetType.TransactionCalendar,
        name: 'Transaction Calendar',
        supportsSettings: [
            {
                settingType: 'customSelect',
                settingName: 'transactionTypes',
                displayName: 'Transaction Type',
                selectValues: [
                    {
                        name: 'Income',
                        value: TransactionType.Income
                    },
                    {
                        name: 'Expense',
                        value: TransactionType.Expense
                    }
                ],
                multiple: true,
                minSelections: 1
            },
            {
                settingType: 'switch',
                settingName: 'showAlternateDate',
                displayName: 'Show Alternate Date'
            },
            {
                settingType: 'switch',
                settingName: 'showAmount',
                displayName: 'Show Amount'
            }
        ],
        defaultSettings: {
            transactionTypes: [
                TransactionType.Income,
                TransactionType.Expense
            ],
            showAlternateDate: true,
            showAmount: true
        },
        dataRequirements: [
            OverviewWidgetDataRequirement.CurrentMonthTransactions
        ]
    }
};

export const DEFAULT_MOBILE_OVERVIEW_LAYOUT: MobileOverviewLayout = {
    widgets: [
        {
            id: 'default-current-month-overview',
            type: OverviewWidgetType.CurrentMonthOverview,
            settings: {
                height: 3
            }
        },
        {
            id: 'default-period-income-expense',
            type: OverviewWidgetType.PeriodIncomeExpense,
            settings: {
                dateRanges: [
                    DateRange.Today.type,
                    DateRange.ThisWeek.type,
                    DateRange.ThisMonth.type,
                    DateRange.ThisYear.type
                ]
            }
        }
    ]
};
