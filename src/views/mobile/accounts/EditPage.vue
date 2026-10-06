<template>
    <f7-page class="cy-account-edit cy-mobile-surface" @page:afterin="onPageAfterIn">
        <f7-navbar>
            <f7-nav-left><f7-link icon-f7="xmark" aria-label="取消" @click="f7router.back()" /></f7-nav-left>
            <f7-nav-title :title="tt(title)"></f7-nav-title>
            <f7-nav-right :class="{ 'navbar-compact-icons': true, 'disabled': loading }">
                <f7-link icon-f7="ellipsis" v-if="account.type === AccountType.MultiSubAccounts.type" :aria-label="tt('More')" @click="showMoreActionSheet = true"></f7-link>
                <f7-link icon-f7="checkmark_alt" :class="{ 'disabled': inputIsEmpty || submitting }" :aria-label="tt('Save')" @click="save"></f7-link>
            </f7-nav-right>
        </f7-navbar>

        <div v-if="loading" class="cy-empty"><f7-preloader /></div>
        <AssetAccountForm v-if="!loading && account.type === AccountType.SingleAccount.type" :account="account" :edit="!!editAccountId" :initial-balance="initialBalance" :income-name="incomeDraft?.fund.name || boundIncomeName" v-model:disabled-books="disabledBooks" v-model:count-adjustment="countAdjustment" @income="showIncomeSetup=true" />
        <f7-list form strong inset dividers class="margin-vertical" v-if="!loading && account.type === AccountType.MultiSubAccounts.type">
            <f7-list-input
                type="text"
                clear-button
                :label="tt('Account Name')"
                :placeholder="tt('Your account name')"
                v-model:value="account.name"
            ></f7-list-input>

            <f7-list-item class="list-item-with-header-and-title list-item-with-multi-item">
                <template #default>
                    <div class="grid grid-cols-2">
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#" @click="accountContext.showIconSelectionSheet = true">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>{{ tt('Account Icon') }}</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <ItemIcon :icon-type="getAccountIconType(account.iconType)" :icon-id="account.icon" :color="account.color"></ItemIcon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>

                            <icon-selection-sheet :all-system-icon-infos="ALL_ACCOUNT_ICONS"
                                                  :color="account.color"
                                                  v-model:show="accountContext.showIconSelectionSheet"
                                                  v-model:icon-type="account.iconType"
                                                  v-model="account.icon"
                            ></icon-selection-sheet>
                        </div>
                        <div class="list-item-subitem no-chevron">
                            <a class="item-link" href="#" @click="accountContext.showColorSelectionSheet = true">
                                <div class="item-content">
                                    <div class="item-inner">
                                        <div class="item-header">
                                            <span>{{ tt('Account Color') }}</span>
                                        </div>
                                        <div class="item-title">
                                            <div class="list-item-custom-title no-padding">
                                                <ItemIcon icon-type="fixed-f7" icon-id="app_fill" :color="account.color"></ItemIcon>
                                            </div>
                                        </div>
                                    </div>
                                </div>
                            </a>

                            <color-selection-sheet :all-system-color-infos="ALL_ACCOUNT_COLORS"
                                                   v-model:show="accountContext.showColorSelectionSheet"
                                                   v-model="account.color"
                            ></color-selection-sheet>
                        </div>
                    </div>
                </template>
            </f7-list-item>

            <f7-list-item
                class="list-item-with-header-and-title list-item-no-item-after"
                link="#"
                :header="tt('Default Currency')"
                @click="accountContext.showCurrencyPopup = true"
                v-if="account.category === AccountCategory.CreditCard.type"
            >
                <template #title>
                    <div class="no-padding no-margin">
                        <span>{{ getCurrencyName(account.currency) }}&nbsp;</span>
                        <small class="smaller" v-if="account.currency !== ACCOUNT_CURRENCY_NOT_SET_VALUE">{{ account.currency }}</small>
                    </div>
                </template>
                <list-item-selection-popup value-type="item"
                                           key-field="currencyCode" value-field="currencyCode"
                                           title-field="displayName" after-field="currencyCode"
                                           :title="tt('Currency Name')"
                                           :enable-filter="true"
                                           :filter-placeholder="tt('Currency')"
                                           :filter-no-items-text="tt('No results')"
                                           :items="allCurrenciesWithNotSet"
                                           v-model:show="accountContext.showCurrencyPopup"
                                           v-model="account.currency">
                </list-item-selection-popup>
            </f7-list-item>

            <f7-list-item
                link="#" no-chevron
                class="list-item-with-header-and-title"
                :class="{ 'disabled': account.currency === '' || account.currency === ACCOUNT_CURRENCY_NOT_SET_VALUE }"
                :header="tt('Credit Limit')"
                :title="getAccountCreditCardCreditLimitDisplayValue(account.numericCreditCardLimit, account.currency)"
                v-if="account.category === AccountCategory.CreditCard.type"
                @click="setNumberPadAnchor($event); showCreditCardLimitSheet = true"
            >
                <number-pad-sheet :reveal-target="numberPadAnchor" :min-value="0"
                                  :max-value="TRANSACTION_MAX_AMOUNT"
                                  :currency="account.currency"
                                  v-model:show="showCreditCardLimitSheet"
                                  v-model="account.numericCreditCardLimit"
                ></number-pad-sheet>
            </f7-list-item>

            <f7-list-item
                link="#"
                class="list-item-with-header-and-title list-item-no-item-after"
                :header="tt('Statement Date')"
                :title="getAccountCreditCardStatementDate(account.creditCardStatementDate)"
                v-if="account.category === AccountCategory.CreditCard.type"
                @click="accountContext.showCreditCardStatementDatePopup = true"
            >
                <list-item-selection-popup value-type="item"
                                           key-field="type" value-field="type"
                                           title-field="displayName"
                                           :title="tt('Statement Date')"
                                           :enable-filter="true"
                                           :filter-placeholder="tt('Statement Date')"
                                           :filter-no-items-text="tt('No results')"
                                           :items="allAvailableMonthDays"
                                           v-model:show="accountContext.showCreditCardStatementDatePopup"
                                           v-model="account.creditCardStatementDate">
                </list-item-selection-popup>
            </f7-list-item>

            <f7-list-item :title="tt('Visible')" v-if="editAccountId">
                <template #after>
                    <f7-toggle :checked="account.visible" @toggle:change="account.visible = $event"></f7-toggle>
                </template>
            </f7-list-item>

            <f7-list-input
                type="textarea"
                style="height: auto"
                :label="tt('Description')"
                :placeholder="tt('Your account description (optional)')"
                v-textarea-auto-size
                v-model:value="account.comment"
            ></f7-list-input>
        </f7-list>

        <f7-block class="no-padding no-margin" v-if="!loading && account.type === AccountType.MultiSubAccounts.type">
            <f7-list strong inset dividers class="subaccount-edit-list margin-vertical"
                     :key="idx"
                     v-for="(subAccount, idx) in subAccounts">
                <f7-list-item group-title>
                    <small>{{ tt('Sub Account') + ' #' + (idx + 1) }}</small>
                    <f7-button rasied fill class="subaccount-delete-button" color="red" icon-f7="trash" icon-size="16px" :aria-label="tt('Remove')"
                               :tooltip="tt('Remove Sub-account')"
                               @click="removeSubAccount(subAccount, false)">
                    </f7-button>
                </f7-list-item>

                <f7-list-input
                    type="text"
                    clear-button
                    :label="tt('Sub-account Name')"
                    :placeholder="tt('Your sub-account name')"
                    v-model:value="subAccount.name"
                ></f7-list-input>

                <f7-list-item class="list-item-with-header-and-title list-item-with-multi-item">
                    <template #default>
                        <div class="grid grid-cols-2">
                            <div class="list-item-subitem no-chevron">
                                <a class="item-link" href="#" @click="subAccountContexts[idx]!.showIconSelectionSheet = true">
                                    <div class="item-content">
                                        <div class="item-inner">
                                            <div class="item-header">
                                                <span>{{ tt('Sub-account Icon') }}</span>
                                            </div>
                                            <div class="item-title">
                                                <div class="list-item-custom-title no-padding">
                                                    <ItemIcon :icon-type="getAccountIconType(subAccount.iconType)" :icon-id="subAccount.icon" :color="subAccount.color"></ItemIcon>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </a>

                                <icon-selection-sheet :all-system-icon-infos="ALL_ACCOUNT_ICONS"
                                                      :color="subAccount.color"
                                                      v-model:show="subAccountContexts[idx]!.showIconSelectionSheet"
                                                      v-model:icon-type="subAccount.iconType"
                                                      v-model="subAccount.icon"
                                ></icon-selection-sheet>
                            </div>
                            <div class="list-item-subitem no-chevron">
                                <a class="item-link" href="#" @click="subAccountContexts[idx]!.showColorSelectionSheet = true">
                                    <div class="item-content">
                                        <div class="item-inner">
                                            <div class="item-header">
                                                <span>{{ tt('Sub-account Color') }}</span>
                                            </div>
                                            <div class="item-title">
                                                <div class="list-item-custom-title no-padding">
                                                    <ItemIcon icon-type="fixed-f7" icon-id="app_fill" :color="subAccount.color"></ItemIcon>
                                                </div>
                                            </div>
                                        </div>
                                    </div>
                                </a>

                                <color-selection-sheet :all-system-color-infos="ALL_ACCOUNT_COLORS"
                                                       v-model:show="subAccountContexts[idx]!.showColorSelectionSheet"
                                                       v-model="subAccount.color"
                                ></color-selection-sheet>
                            </div>
                        </div>
                    </template>
                </f7-list-item>

                <f7-list-item
                    class="list-item-with-header-and-title list-item-no-item-after"
                    link="#"
                    :class="{ 'disabled': editAccountId && !isNewAccount(subAccount) }"
                    :header="tt('Currency')"
                    :no-chevron="!!editAccountId && !isNewAccount(subAccount)"
                    @click="subAccountContexts[idx]!.showCurrencyPopup = true"
                >
                    <template #title>
                        <div class="no-padding no-margin">
                            <span>{{ getCurrencyName(subAccount.currency) }}&nbsp;</span>
                            <small class="smaller">{{ subAccount.currency }}</small>
                        </div>
                    </template>
                    <list-item-selection-popup value-type="item"
                                               key-field="currencyCode" value-field="currencyCode"
                                               title-field="displayName" after-field="currencyCode"
                                               :title="tt('Currency Name')"
                                               :enable-filter="true"
                                               :filter-placeholder="tt('Currency')"
                                               :filter-no-items-text="tt('No results')"
                                               :items="allCurrencies"
                                               v-model:show="subAccountContexts[idx]!.showCurrencyPopup"
                                               v-model="subAccount.currency">
                    </list-item-selection-popup>
                </f7-list-item>

                <f7-list-item
                    link="#" no-chevron
                    class="list-item-with-header-and-title"
                    :class="{ 'disabled': editAccountId && !isNewAccount(subAccount) }"
                    :header="account.isLiability ? tt('Sub-account Outstanding Balance') : tt('Sub-account Balance')"
                    :title="formatAccountDisplayBalance(subAccount)"
                    @click="setNumberPadAnchor($event); subAccountContexts[idx]!.showBalanceSheet = true"
                >
                    <number-pad-sheet :reveal-target="numberPadAnchor" :min-value="TRANSACTION_MIN_AMOUNT"
                                      :max-value="TRANSACTION_MAX_AMOUNT"
                                      :currency="subAccount.currency"
                                      :flip-negative="account.isLiability"
                                      v-model:show="subAccountContexts[idx]!.showBalanceSheet"
                                      v-model="subAccount.numericBalance"
                    ></number-pad-sheet>
                </f7-list-item>

                <f7-list-item
                    class="account-edit-datetime list-item-with-header-and-title"
                    link="#" no-chevron
                    v-show="subAccount.numericBalance"
                    v-if="!editAccountId || isNewAccount(subAccount)"
                >
                    <template #header>
                        <div class="account-edit-datetime-header" @click="showBalanceDateTimeDialog(subAccountContexts[idx] as AccountContext, 'time')">{{ tt('Sub-account Balance Time') }}</div>
                    </template>
                    <template #title>
                        <div class="account-edit-datetime-title">
                            <div @click="showBalanceDateTimeDialog(subAccountContexts[idx] as AccountContext, 'date')">{{ formatDate(subAccount.balanceTime) }}</div>&nbsp;<div class="account-edit-datetime-time" @click="showBalanceDateTimeDialog(subAccountContexts[idx] as AccountContext, 'time')">{{ formatTime(subAccount.balanceTime) }}</div>
                        </div>
                    </template>
                    <date-time-selection-sheet :init-mode="subAccountContexts[idx]!.balanceDateTimeSheetMode"
                                               :timezone-utc-offset="getDefaultTimezoneOffsetMinutes(subAccount.balanceTime)"
                                               :model-value="subAccount.balanceTime"
                                               v-model:show="subAccountContexts[idx]!.showBalanceDateTimeSheet"
                                               @update:model-value="updateAccountBalanceTime(subAccount, $event)">
                    </date-time-selection-sheet>
                </f7-list-item>

                <f7-list-item
                    class="account-edit-datetime list-item-with-header-and-title"
                    link="#" no-chevron
                    v-if="editAccountId && !isNewAccount(subAccount) && useLastReconciledTime"
                >
                    <template #header>
                        <div class="account-edit-datetime-header" @click="showLastReconciledDateTimeDialog(subAccountContexts[idx] as AccountContext, 'time')">{{ tt('Sub-account Last Reconciled Time') }}</div>
                    </template>
                    <template #title>
                        <div class="account-edit-datetime-title" v-if="subAccount.lastReconciledTime">
                            <div @click="showLastReconciledDateTimeDialog(subAccountContexts[idx] as AccountContext, 'date')">{{ formatDate(subAccount.lastReconciledTime) }}</div>&nbsp;<div class="account-edit-datetime-time" @click="showLastReconciledDateTimeDialog(subAccountContexts[idx] as AccountContext, 'time')">{{ formatTime(subAccount.lastReconciledTime) }}</div>
                        </div>
                        <div class="account-edit-datetime-title" v-else>
                            <div class="account-edit-datetime-time" @click="showLastReconciledDateTimeDialog(subAccountContexts[idx] as AccountContext, 'date')">{{ tt('None') }}</div>
                        </div>
                    </template>
                    <date-time-selection-sheet :init-mode="subAccountContexts[idx]!.lastReconciledDateTimeSheetMode"
                                               :clearable="true"
                                               :timezone-utc-offset="getDefaultTimezoneOffsetMinutes(subAccount.lastReconciledTime)"
                                               :model-value="subAccount.lastReconciledTime ?? getCurrentUnixTime()"
                                               v-model:show="subAccountContexts[idx]!.showLastReconciledTimeSheet"
                                               @update:model-value="updateAccountLastReconciledTime(subAccount, $event)"
                                               @clear:model-value="subAccount.lastReconciledTime = undefined">
                    </date-time-selection-sheet>
                </f7-list-item>

                <f7-list-item :title="tt('Visible')" v-if="editAccountId && !isNewAccount(subAccount)">
                    <template #after>
                        <f7-toggle :checked="subAccount.visible" @toggle:change="subAccount.visible = $event"></f7-toggle>
                    </template>
                </f7-list-item>

                <f7-list-input
                    type="textarea"
                    style="height: auto"
                    :label="tt('Description')"
                    :placeholder="tt('Your sub-account description (optional)')"
                    v-textarea-auto-size
                    v-model:value="subAccount.comment"
                ></f7-list-input>
            </f7-list>
        </f7-block>

        <f7-actions close-by-outside-click close-on-escape :opened="showMoreActionSheet" @actions:closed="showMoreActionSheet = false">
            <f7-actions-group>
                <f7-actions-button @click="addSubAccountAndContext">{{ tt('Add Sub-account') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>

        <f7-actions close-by-outside-click close-on-escape :opened="showDeleteActionSheet" @actions:closed="showDeleteActionSheet = false">
            <f7-actions-group>
                <f7-actions-label>{{ tt('Are you sure you want to remove this sub-account?') }}</f7-actions-label>
                <f7-actions-button color="red" @click="removeSubAccount(subAccountToDelete, true)">{{ tt('Remove') }}</f7-actions-button>
            </f7-actions-group>
            <f7-actions-group>
                <f7-actions-button bold close>{{ tt('Cancel') }}</f7-actions-button>
            </f7-actions-group>
        </f7-actions>
        <f7-popup v-model:opened="showIncomeSetup" class="cy-income-setup" :close-by-backdrop-click="false"><MonetaryIncomePage v-if="showIncomeSetup" :draft-account="account" :draft="incomeDraft" @configured="configureIncome" @cancel="showIncomeSetup=false" /></f7-popup>
        <BankSelectionSheet v-model:opened="showBanks" :credit="account.category===3" @select="selectBank" />
    </f7-page>
</template>

<script setup lang="ts">
import { ref, computed, watch, nextTick } from 'vue';
import AssetAccountForm from '@/components/mobile/AssetAccountForm.vue';
import BankSelectionSheet from '@/components/mobile/BankSelectionSheet.vue';
import { assetTools } from '@/lib/asset-tools.ts';
import { useAssetToolsStore } from '@/stores/assetTools.ts';
import { useTransactionsStore } from '@/stores/transaction.ts';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
import { getTimeZone } from '@/lib/settings.ts';
import { getBrowserTimezoneName } from '@/lib/datetime.ts';
import { findAccountPreset } from '@/lib/account-presets.ts';
import MonetaryIncomePage from '@/views/mobile/accounts/MonetaryIncomePage.vue';
import {monetaryIncome,type MonetaryDraft} from '@/lib/monetary-income.ts';
import {investmentError} from '@/lib/investments.ts';
import type { Router } from 'framework7/types';

import { useI18n } from '@/locales/helpers.ts';
import { useI18nUIComponents, showLoading, hideLoading } from '@/lib/ui/mobile.ts';
import { useAccountEditPageBase } from '@/views/base/accounts/AccountEditPageBase.ts';

import { useAccountsStore } from '@/stores/account.ts';

import { itemAndIndex } from '@/core/base.ts';
import type { LocalizedCurrencyInfo } from '@/core/currency.ts';
import { AccountCategory, AccountType } from '@/core/account.ts';
import { ALL_ACCOUNT_ICONS } from '@/consts/icon.ts';
import { ALL_ACCOUNT_COLORS } from '@/consts/color.ts';
import { ACCOUNT_CURRENCY_NOT_SET_VALUE } from '@/consts/currency.ts';
import { TRANSACTION_MIN_AMOUNT, TRANSACTION_MAX_AMOUNT } from '@/consts/transaction.ts';
import type { Account } from '@/models/account.ts';
import type { ErrorResponse } from '@/core/api.ts';

import { isDefined } from '@/lib/common.ts';
import { parseBigDecimal } from '@/lib/numeral.ts';
import { getAccountIconType } from '@/lib/icon.ts';
import {
    getTimezoneOffsetMinutes,
    getCurrentUnixTime,
    parseDateTimeFromUnixTimeWithTimezoneOffset
} from '@/lib/datetime.ts';
import { generateRandomUUID } from '@/lib/misc.ts';

interface AccountContext {
    showIconSelectionSheet: boolean;
    showColorSelectionSheet: boolean;
    showCurrencyPopup: boolean;
    showCreditCardStatementDatePopup: boolean;
    showBalanceSheet: boolean;
    showBalanceDateTimeSheet: boolean;
    showLastReconciledTimeSheet: boolean;
    balanceDateTimeSheetMode: string;
    lastReconciledDateTimeSheetMode: string;
}

const props = defineProps<{
    f7route: Router.Route;
    f7router: Router.Router;
}>();

const {
    tt,
    te,
    getAllCurrencies,
    getCurrencyName,
    formatDateTimeToLongDate,
    formatDateTimeToLongTime,
    formatAmountToLocalizedNumeralsWithCurrency
} = useI18n();

const { showAlert, showToast, routeBackOnError } = useI18nUIComponents();

const {
    editAccountId,
    clientSessionId,
    loading,
    submitting,
    account,
    subAccounts,
    useLastReconciledTime,
    title,
    inputEmptyProblemMessage,
    inputIsEmpty,
    allAvailableMonthDays,
    getDefaultTimezoneOffsetMinutes,
    getAccountCreditCardStatementDate,
    getAccountCreditCardCreditLimitDisplayValue,
    updateAccountBalanceTime,
    updateAccountLastReconciledTime,
    isNewAccount,
    addSubAccount,
    setAccount
} = useAccountEditPageBase();

const accountsStore = useAccountsStore(), assetPreferences = useAssetToolsStore();
const initialBalance=ref('0'), disabledBooks=ref<string[]>([]), countAdjustment=ref(false),boundIncomeName=ref('');
const showBanks=ref(false);
function selectBank(bank:{name:string;color:string}):void{account.value.name=bank.name+(account.value.category===3?'信用卡':'');account.value.color=bank.color.replace('#','');}
let adjustmentRequestId=generateRandomUUID();
const incomeDraft=ref<MonetaryDraft>(),showIncomeSetup=ref(false),numberPadAnchor=ref<HTMLElement>();
watch(()=>account.value.type===AccountType.SingleAccount.type && account.value.currency==='CNY' && [1,2,4,8].includes(account.value.category),eligible=>{if(!eligible){incomeDraft.value=undefined;showIncomeSetup.value=false;}});
function configureIncome(draft:MonetaryDraft):void{incomeDraft.value=draft;showIncomeSetup.value=false;}
function setNumberPadAnchor(event:MouseEvent):void{if(event.target instanceof Element)numberPadAnchor.value=event.target.closest<HTMLElement>('li') || undefined;}

const DEFAULT_ACCOUNT_CONTEXT: AccountContext = {
    showIconSelectionSheet: false,
    showColorSelectionSheet: false,
    showCurrencyPopup: false,
    showCreditCardStatementDatePopup: false,
    showBalanceSheet: false,
    showBalanceDateTimeSheet: false,
    showLastReconciledTimeSheet: false,
    balanceDateTimeSheetMode: 'time',
    lastReconciledDateTimeSheetMode: 'time'
};

const accountContext = ref<AccountContext>(Object.assign({}, DEFAULT_ACCOUNT_CONTEXT));
const subAccountContexts = ref<AccountContext[]>([]);
const subAccountToDelete = ref<Account | null>(null);
const loadingError = ref<unknown | null>(null);
const showMoreActionSheet = ref<boolean>(false);
const showDeleteActionSheet = ref<boolean>(false);
const showCreditCardLimitSheet = ref<boolean>(false);

const allCurrencies = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies());
const allCurrenciesWithNotSet = computed<LocalizedCurrencyInfo[]>(() => getAllCurrencies(true));

function formatAccountDisplayBalance(selectedAccount: Account): string {
    const balance = parseBigDecimal(selectedAccount.balance);
    return formatAmountToLocalizedNumeralsWithCurrency(account.value.isLiability ? balance.negate() : balance, selectedAccount.currency);
}

function formatDate(unixTime?: number): string {
    if (!isDefined(unixTime)) {
        return '';
    }

    const dateTime = parseDateTimeFromUnixTimeWithTimezoneOffset(unixTime, getTimezoneOffsetMinutes(unixTime));
    return formatDateTimeToLongDate(dateTime);
}

function formatTime(unixTime?: number): string {
    if (!isDefined(unixTime)) {
        return '';
    }

    const dateTime = parseDateTimeFromUnixTimeWithTimezoneOffset(unixTime, getTimezoneOffsetMinutes(unixTime));
    return formatDateTimeToLongTime(dateTime);
}

function init(): void {
    const query = props.f7route.query;
    clientSessionId.value = generateRandomUUID();

    if (query['id']) {
        loading.value = true;

        editAccountId.value = query['id'];

        accountsStore.getAccount({
            accountId: editAccountId.value
        }).then(response => {
            setAccount(response);
            initialBalance.value=response.balance;
            void assetPreferences.load().then(()=>{disabledBooks.value=[...(assetPreferences.preferences.rules[`cash:${response.id}`]?.disabledBooks||[])];});
            void monetaryIncome.list().then(rows=>{boundIncomeName.value=rows.find(r=>r.accountId===response.id&&r.enabled)?.name||'';}).catch(()=>undefined);
            subAccountContexts.value = [];

            for (let i = 0; i < subAccounts.value.length; i++) {
                subAccountContexts.value.push(Object.assign({}, DEFAULT_ACCOUNT_CONTEXT));
            }

            loading.value = false;
        }).catch(error => {
            if (error.processed) {
                loading.value = false;
            } else {
                loadingError.value = error;
                showToast(error.message || error);
            }
        });
    } else {
        const preset = findAccountPreset(query['preset']);
        if (preset?.category) {
            account.value.category = preset.category;
            account.value.assetProfile = {kind:preset.assetKind,group:preset.group,sharedLimitAccount:'',repaymentDay:0,repaymentAfterDays:0};
            account.value.name = preset.name === '自定义' ? '' : preset.name;
            account.value.color = preset.color.replace('#', '');
            void nextTick(() => { account.value.icon = preset.icon; });
        }
        if(query['monetary']==='1'){account.value.name='货币基金';account.value.assetProfile.group='货币基金';void nextTick(()=>{showIncomeSetup.value=true;});}
        loading.value = false;
    }
}

async function save(): Promise<void> {
    if (submitting.value) return;
    const problemMessage=inputEmptyProblemMessage.value;
    if(problemMessage){showAlert(problemMessage);return;}
    const wasNew=!editAccountId.value, desiredBalance=account.value.balance;
    submitting.value=true;showLoading(()=>submitting.value);
    let metadataSaved=false;
    try {
        const savedAccount=await accountsStore.saveAccount({account:account.value,subAccounts:subAccounts.value,isEdit:!wasNew,clientSessionId:clientSessionId.value,allowUnchanged:true});
        metadataSaved=true;
        if(wasNew){editAccountId.value=savedAccount.id;account.value.id=savedAccount.id;initialBalance.value=savedAccount.balance;}
        if(!wasNew && account.value.type===AccountType.SingleAccount.type && desiredBalance!==initialBalance.value){
            await assetTools.adjustBalance({accountId:savedAccount.id,balance:new LedgerDecimal(desiredBalance).div(100).toString(),expectedBalance:new LedgerDecimal(initialBalance.value).div(100).toString(),bookId:'',countInStatistics:countAdjustment.value,timeZone:getTimeZone()||getBrowserTimezoneName(),requestId:adjustmentRequestId});
            initialBalance.value=desiredBalance;adjustmentRequestId=generateRandomUUID();
            useTransactionsStore().updateStoreInvalidState({accountList:true,transactionList:true,overview:true,statistics:true,explorer:true,reconciliationStatement:true});
        }
        await assetPreferences.load();
        const preferences=JSON.parse(JSON.stringify(assetPreferences.preferences));
        const key=`cash:${savedAccount.id}`,old=preferences.rules[key];
        if(JSON.stringify(old?.disabledBooks||[])!==JSON.stringify(disabledBooks.value)){
            preferences.rules[key]={hidden:old?.hidden||false,disabledBooks:[...disabledBooks.value]};
            await assetPreferences.save(preferences);
        }
        if(incomeDraft.value){await monetaryIncome.save({...incomeDraft.value,accountId:savedAccount.id});incomeDraft.value=undefined;}
        showToast(wasNew?'You have added a new account':'You have saved this account');props.f7router.back();
    } catch(cause) {
        const wrapped=cause as {error?:ErrorResponse;message?:string}|null;
        const message=wrapped?.error?.errorMessage ? te({error:wrapped.error}) : wrapped?.message ? te(wrapped.message) : investmentError(cause);
        showAlert(`${metadataSaved?'账户资料已保存，后续设置尚未完成：':''}${message}`);
    }
    finally {submitting.value=false;hideLoading();}
}

function addSubAccountAndContext(): void {
    if (addSubAccount()) {
        subAccountContexts.value.push(Object.assign({}, DEFAULT_ACCOUNT_CONTEXT));
    }
}

function removeSubAccount(currentSubAccount: Account | null, confirm: boolean): void {
    if (!currentSubAccount) {
        showAlert('An error occurred');
        return;
    }

    if (!confirm) {
        subAccountToDelete.value = currentSubAccount;
        showDeleteActionSheet.value = true;
        return;
    }

    showDeleteActionSheet.value = false;
    subAccountToDelete.value = null;

    for (const [subAccount, index] of itemAndIndex(subAccounts.value)) {
        if (subAccount === currentSubAccount) {
            subAccounts.value.splice(index, 1);
            subAccountContexts.value.splice(index, 1);
        }
    }
}

function showBalanceDateTimeDialog(accountContext: AccountContext, sheetMode: string): void {
    accountContext.balanceDateTimeSheetMode = sheetMode;
    accountContext.showBalanceDateTimeSheet = true;
}

function showLastReconciledDateTimeDialog(accountContext: AccountContext, sheetMode: string): void {
    accountContext.lastReconciledDateTimeSheetMode = sheetMode;
    accountContext.showLastReconciledTimeSheet = true;
}

function onPageAfterIn(): void {
    routeBackOnError(props.f7router, loadingError);
    if (!editAccountId.value && !bankPickerShown && ['bank','credit'].includes(props.f7route.query['preset']||'')) {bankPickerShown=true;showBanks.value=true;}
}
let bankPickerShown=false;

watch(() => account.value.type, () => {
    if (subAccounts.value.length < 1) {
        addSubAccountAndContext();
    }
});

init();
</script>

<style>
.cy-account-edit .list-item-with-multi-item .list-item-subitem .item-inner::after{display:none}
.cy-account-edit .list-item-with-multi-item > .item-content > .item-inner::after{background-color:var(--f7-list-item-border-color);left:calc(var(--f7-list-item-padding-horizontal) + var(--f7-safe-area-left));width:calc(100% - var(--f7-list-item-padding-horizontal) - var(--f7-safe-area-left))}

.account-edit-datetime .item-title {
    width: 100%;
}

.account-edit-datetime .item-title > .item-header > .account-edit-datetime-header {
    display: block;
    width: 100%;
}

.account-edit-datetime .item-title > .account-edit-datetime-title {
    display: flex;
    width: 100%;
}

.account-edit-datetime .item-title > .account-edit-datetime-title > .account-edit-datetime-time {
    flex-grow: 1;
    overflow: hidden;
    text-overflow: ellipsis;
}

.subaccount-delete-button {
    margin-inline-start: auto;
}
</style>
