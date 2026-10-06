import '@/styles/mobile/investment.scss';
import type { Router } from 'framework7/types';

import { isUserLogined, isUserUnlocked } from '@/lib/userstate.ts';
import { isNativePersonalMode } from '@/lib/native.ts';

import HomePage from '@/views/mobile/HomePage.vue';
import InvestmentPage from '@/views/mobile/InvestmentPage.vue';
import AssetToolsPage from '@/views/mobile/AssetToolsPage.vue';
import DebtPage from '@/views/mobile/DebtPage.vue';
import ReimbursementPage from '@/views/mobile/ReimbursementPage.vue';
import InvestmentManagePage from '@/views/mobile/InvestmentManagePage.vue';
import InvestmentSetupPage from '@/views/mobile/InvestmentSetupPage.vue';
import InvestmentPlansPage from '@/views/mobile/InvestmentPlansPage.vue';
import InvestmentStatisticsPage from '@/views/mobile/InvestmentStatisticsPage.vue';
import InvestmentPositionPage from '@/views/mobile/InvestmentPositionPage.vue';
import InvestmentRecordPage from '@/views/mobile/InvestmentRecordPage.vue';
import CryptoAccountPage from '@/views/mobile/CryptoAccountPage.vue';
import CryptoAccountDetailPage from '@/views/mobile/CryptoAccountDetailPage.vue';
import WalletEntryPage from '@/views/mobile/WalletEntryPage.vue';
import CryptoConvertPage from '@/views/mobile/CryptoConvertPage.vue';
import InvestmentDetailPage from '@/views/mobile/InvestmentDetailPage.vue';
import LedgerMonthPage from '@/views/mobile/LedgerMonthPage.vue';
import CalendarSettingsPage from '@/views/mobile/CalendarSettingsPage.vue';
import CalendarDuePage from '@/views/mobile/CalendarDuePage.vue';
import LedgerDetailsPage from '@/views/mobile/LedgerDetailsPage.vue';
import BooksPage from '@/views/mobile/BooksPage.vue';
import StatisticsOverviewPage from '@/views/mobile/StatisticsOverviewPage.vue';
import StatisticsBudgetPage from '@/views/mobile/StatisticsBudgetPage.vue';
import LoginPage from '@/views/mobile/LoginPage.vue';
import SignUpPage from '@/views/mobile/SignupPage.vue';
import UnlockPage from '@/views/mobile/UnlockPage.vue';

import TransactionListPage from '@/views/mobile/transactions/ListPage.vue';
import TransactionEditPage from '@/views/mobile/transactions/EditPage.vue';
import TransactionAmountFilterPage from '@/views/mobile/transactions/AmountFilterPage.vue';

import AccountListPage from '@/views/mobile/accounts/ListPage.vue';
import AccountEditPage from '@/views/mobile/accounts/EditPage.vue';
import AccountDetailPage from '@/views/mobile/accounts/DetailPage.vue';
import CreditPage from '@/views/mobile/accounts/CreditPage.vue';
import InstallmentPage from '@/views/mobile/accounts/InstallmentPage.vue';
import AccountActivityPage from '@/views/mobile/accounts/ActivityPage.vue';
import MonetaryIncomePage from '@/views/mobile/accounts/MonetaryIncomePage.vue';
import AccountReconciliationStatementPage from '@/views/mobile/accounts/ReconciliationStatementPage.vue';
import AccountMoveAllTransactionsPage from '@/views/mobile/accounts/MoveAllTransactionsPage.vue';

import StatisticsTransactionPage from '@/views/mobile/statistics/TransactionPage.vue';
import StatisticsSettingsPage from '@/views/mobile/statistics/SettingsPage.vue';

import TextSizeSettingsPage from '@/views/mobile/settings/TextSizeSettingsPage.vue';
import PreferencesSettingsPage from '@/views/mobile/settings/PreferencesSettingsPage.vue';
import OverviewLayoutEditorPage from '@/views/mobile/overview/LayoutEditorPage.vue';
import ChartColorSchemeSettingsPage from '@/views/mobile/settings/ChartColorSchemeSettingsPage.vue';
import AccountCategoryDisplayOrderSettingsPage from '@/views/mobile/settings/AccountCategoryDisplayOrderSettingsPage.vue';
import ApplicationCloudSyncSettingsPage from '@/views/mobile/settings/ApplicationCloudSyncSettingsPage.vue';
import BrowserCacheSettingPage from '@/views/mobile/settings/BrowserCacheSettingPage.vue';
import AccountFilterSettingsPage from '@/views/mobile/settings/AccountFilterSettingsPage.vue';
import CategoryFilterSettingsPage from '@/views/mobile/settings/CategoryFilterSettingsPage.vue';
import TransactionTagFilterSettingsPage from '@/views/mobile/settings/TransactionTagFilterSettingsPage.vue';

import SettingsPage from '@/views/mobile/SettingsPage.vue';
import MoreSettingsPage from '@/views/mobile/settings/MoreSettingsPage.vue';
import AppearancePage from '@/views/mobile/settings/AppearancePage.vue';
import CardsPage from '@/views/mobile/settings/CardsPage.vue';
import WishesPage from '@/views/mobile/WishesPage.vue';
import KeywordsPage from '@/views/mobile/KeywordsPage.vue';
import ApplicationLockPage from '@/views/mobile/ApplicationLockPage.vue';
import ExchangeRatesListPage from '@/views/mobile/exchangerates/ListPage.vue';
import ExchangeRatesUpdatePage from '@/views/mobile/exchangerates/UpdatePage.vue';
import AboutPage from '@/views/mobile/AboutPage.vue';

import UserProfilePage from '@/views/mobile/users/UserProfilePage.vue';
import BackupPage from '@/views/mobile/users/BackupPage.vue';
import ImportPage from '@/views/mobile/users/ImportPage.vue';
import DataManagementPage from '@/views/mobile/users/DataManagementPage.vue';
import TwoFactorAuthPage from '@/views/mobile/users/TwoFactorAuthPage.vue';
import SessionListPage from '@/views/mobile/users/SessionListPage.vue';

import CategoryAllPage from '@/views/mobile/categories/AllPage.vue';
import CategoryListPage from '@/views/mobile/categories/ListPage.vue';
import CategoryEditPage from '@/views/mobile/categories/EditPage.vue';
import CategoryPresetPage from '@/views/mobile/categories/PresetPage.vue';

import TagListPage from '@/views/mobile/tags/ListPage.vue';
import TagGroupListPage from '@/views/mobile/tags/GroupListPage.vue';

import TemplateListPage from '@/views/mobile/templates/ListPage.vue';

function asyncResolve(component: unknown): (ctx: Router.RouteCallbackCtx) => void {
    return function({ resolve }: { resolve: ({ component }: { component: unknown }) => void }): void {
        return resolve({
            component: component
        });
    } as unknown as (ctx: Router.RouteCallbackCtx) => void;
}

function checkLogin({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (!isUserLogined()) {
        reject();
        router.navigate('/login', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (!isUserUnlocked()) {
        reject();
        router.navigate('/unlock', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

function checkLocked({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (!isUserLogined()) {
        reject();
        router.navigate('/login', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (isUserUnlocked()) {
        reject();
        router.navigate('/', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

function checkNotLogin({ router, resolve, reject }: { router: Router.Router, resolve: () => void, reject: () => void }): void {
    if (isNativePersonalMode()) {
        reject();
        window.location.replace('/personal');
        return;
    }
    if (isUserLogined() && !isUserUnlocked()) {
        reject();
        router.navigate('/unlock', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    if (isUserLogined()) {
        reject();
        router.navigate('/', {
            clearPreviousHistory: true,
            browserHistory: false
        });
        return;
    }

    resolve();
}

const routes: Router.RouteParameters[] = [
    { path: '/settings/more', async: asyncResolve(MoreSettingsPage), beforeEnter: [checkLogin] },
    { path: '/settings/appearance', async: asyncResolve(AppearancePage), beforeEnter: [checkLogin] },
    { path: '/user/backup', async: asyncResolve(BackupPage), beforeEnter: [checkLogin] },
    { path: '/user/import', async: asyncResolve(ImportPage), beforeEnter: [checkLogin] },
    { path: '/settings/cards', async: asyncResolve(CardsPage), beforeEnter: [checkLogin] },
    { path: '/wishes', async: asyncResolve(WishesPage), beforeEnter: [checkLogin] },
    { path: '/keywords', async: asyncResolve(KeywordsPage), beforeEnter: [checkLogin] },
    { path: '/assets/debts', async: asyncResolve(DebtPage), beforeEnter: [checkLogin] },
    { path: '/account/debt', async: asyncResolve(DebtPage), beforeEnter: [checkLogin] },
    { path: '/assets/reimbursements', async: asyncResolve(ReimbursementPage), beforeEnter: [checkLogin] },
    { path: '/account/detail', async: asyncResolve(AccountDetailPage), beforeEnter: [checkLogin] },
    { path: '/account/credit', async: asyncResolve(CreditPage), beforeEnter: [checkLogin] },
    { path: '/account/installments', async: asyncResolve(InstallmentPage), beforeEnter: [checkLogin] },
    { path: '/account/activity', async: asyncResolve(AccountActivityPage), beforeEnter: [checkLogin] },
    { path: '/investments/add', async: asyncResolve(InvestmentSetupPage), beforeEnter: [checkLogin] },
    { path: '/investments/plans', async: asyncResolve(InvestmentPlansPage), beforeEnter: [checkLogin] },
    { path: '/investments/statistics', async: asyncResolve(InvestmentStatisticsPage), beforeEnter: [checkLogin] },
    { path: '/investments/manage', async: asyncResolve(InvestmentManagePage), beforeEnter: [checkLogin] },
    { path: '/investments/position', async: asyncResolve(InvestmentPositionPage), beforeEnter: [checkLogin] },
    { path: '/crypto/add', async: asyncResolve(CryptoAccountPage), beforeEnter: [checkLogin] },
    { path: '/crypto/entry', async: asyncResolve(WalletEntryPage), beforeEnter: [checkLogin] },
    { path: '/crypto/account', async: asyncResolve(CryptoAccountDetailPage), beforeEnter: [checkLogin] },
    { path: '/crypto/convert', async: asyncResolve(CryptoConvertPage), beforeEnter: [checkLogin] },
    { path: '/investments/record', async: asyncResolve(InvestmentRecordPage), beforeEnter: [checkLogin] },
    { path: '/ledger/details', async: asyncResolve(LedgerDetailsPage), beforeEnter: [checkLogin] },
    { path: '/statistics/budgets', async: asyncResolve(StatisticsBudgetPage), beforeEnter: [checkLogin] },
    { path: '/books', async: asyncResolve(BooksPage), beforeEnter: [checkLogin] },
    { path: '/calendar/settings', async: asyncResolve(CalendarSettingsPage), beforeEnter: [checkLogin] },
    { path: '/calendar/due', async: asyncResolve(CalendarDuePage), beforeEnter: [checkLogin] },
    { path: '/assets/tool', async: asyncResolve(AssetToolsPage), beforeEnter: [checkLogin] },
    {
        path: '/',
        async: asyncResolve(LedgerMonthPage),
        beforeEnter: [checkLogin],
        options: {
            animate: false,
            clearPreviousHistory: true,
        }
    },
    {
        path: '/calendar',
        async: asyncResolve(LedgerMonthPage),
        beforeEnter: [checkLogin],
        options: { animate: false, clearPreviousHistory: true }
    },
    {
        path: '/statistics',
        async: asyncResolve(StatisticsOverviewPage),
        beforeEnter: [checkLogin],
        options: { animate: false, clearPreviousHistory: true }
    },
    {
        path: '/investments/ledger',
        async: asyncResolve(InvestmentDetailPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/investments',
        async: asyncResolve(InvestmentPage),
        beforeEnter: [checkLogin],
        options: { animate: false, clearPreviousHistory: true }
    },
    {
        path: '/overview',
        async: asyncResolve(HomePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/login',
        async: asyncResolve(LoginPage),
        beforeEnter: [checkNotLogin],
        options: {
            animate: false,
        }
    },
    {
        path: '/signup',
        async: asyncResolve(SignUpPage),
        beforeEnter: [checkNotLogin],
        options: {
            animate: false,
        }
    },
    {
        path: '/unlock',
        async: asyncResolve(UnlockPage),
        beforeEnter: [checkLocked],
        options: {
            animate: false,
        }
    },
    {
        path: '/transaction/list',
        async: asyncResolve(TransactionListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/filter/amount',
        async: asyncResolve(TransactionAmountFilterPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/add',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/edit',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/transaction/detail',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/list',
        async: asyncResolve(AccountListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/add',
        async: asyncResolve(AccountEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/edit',
        async: asyncResolve(AccountEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/income',
        async: asyncResolve(MonetaryIncomePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/reconciliation_statements',
        async: asyncResolve(AccountReconciliationStatementPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/account/move_all_transactions',
        async: asyncResolve(AccountMoveAllTransactionsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/statistic/transaction',
        async: asyncResolve(StatisticsTransactionPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/statistic/settings',
        async: asyncResolve(StatisticsSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/textsize',
        async: asyncResolve(TextSizeSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/account',
        async: asyncResolve(AccountFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/category',
        async: asyncResolve(CategoryFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/filter/tag',
        async: asyncResolve(TransactionTagFilterSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/preferences',
        async: asyncResolve(PreferencesSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/overview_layout',
        async: asyncResolve(OverviewLayoutEditorPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/chart_color_scheme',
        async: asyncResolve(ChartColorSchemeSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/account_category_display_order',
        async: asyncResolve(AccountCategoryDisplayOrderSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/sync',
        async: asyncResolve(ApplicationCloudSyncSettingsPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings/browser_caches',
        async: asyncResolve(BrowserCacheSettingPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/settings',
        async: asyncResolve(SettingsPage),
        beforeEnter: [checkLogin],
        options: { animate: false, clearPreviousHistory: true }
    },
    {
        path: '/app_lock',
        async: asyncResolve(ApplicationLockPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/exchange_rates',
        async: asyncResolve(ExchangeRatesListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/exchange_rates/update',
        async: asyncResolve(ExchangeRatesUpdatePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/about',
        async: asyncResolve(AboutPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/profile',
        async: asyncResolve(UserProfilePage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/data/management',
        async: asyncResolve(DataManagementPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/2fa',
        async: asyncResolve(TwoFactorAuthPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/user/sessions',
        async: asyncResolve(SessionListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/all',
        async: asyncResolve(CategoryAllPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/list',
        async: asyncResolve(CategoryListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/add',
        async: asyncResolve(CategoryEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/edit',
        async: asyncResolve(CategoryEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/category/preset',
        async: asyncResolve(CategoryPresetPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/tag/list',
        async: asyncResolve(TagListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/tag/group/list',
        async: asyncResolve(TagGroupListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/list',
        async: asyncResolve(TemplateListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/schedule/list',
        async: asyncResolve(TemplateListPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/add',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '/template/edit',
        async: asyncResolve(TransactionEditPage),
        beforeEnter: [checkLogin]
    },
    {
        path: '(.*)',
        redirect: '/'
    }
];

export default isNativePersonalMode() ? routes.filter(route => route.path !== '/signup') : routes;
