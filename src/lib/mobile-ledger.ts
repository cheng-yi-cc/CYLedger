import { computed, onScopeDispose, ref, watch } from 'vue';
import { LedgerDecimal } from '@/lib/ledger-display.ts';
export { LedgerDecimal, ledgerMoney, ledgerTotals, ledgerSignedAmount } from '@/lib/ledger-display.ts';
import moment from 'moment-timezone';
import services from '@/lib/services.ts';
import { investmentError } from '@/lib/investments.ts';
import { useAccountsStore } from '@/stores/account.ts';
import { useTransactionCategoriesStore } from '@/stores/transactionCategory.ts';
import { useTransactionTagsStore } from '@/stores/transactionTag.ts';
import { useBooksStore } from '@/stores/books.ts';
import { useLedgerScopeStore } from '@/stores/ledgerScope.ts';
import { useUserStore } from '@/stores/user.ts';
import type { TransactionInfoResponse } from '@/models/transaction.ts';
import type { TransactionPictureInfoBasicResponse } from '@/models/transaction_picture_info.ts';

export interface LedgerEntry {
    id: string; type: number; day: string; time: number; title: string; account: string;
    currency: string; amount: string; cny: string | null; hidden: boolean;
    categoryId: string; primaryCategoryId: string; primaryCategory: string; comment: string; tags: string[];
    bookId: string; accountId: string; destinationAccountId: string; tagIds: string[]; investment: boolean;
    investmentEventId?: string; excludeFromStatistics?: boolean; reimbursementAccountId?: string; reimbursementReceiptId?: string;
    discountAmount?: string; transferFeeParentId?: string;
    bookName?: string; icon?: string; iconType?: number; color?: string; transferDirection?: 'in'|'out';
    accountIcon?: string; accountIconType?: number; pictures?: TransactionPictureInfoBasicResponse[];
    location?: string; reimbursementClosedAt?: number; destinationAccount?: string; sourceAccountName?:string; destinationCurrency?:string; destinationAmount?:string; transferFeeAmount?:string; debtDueDate?:string;
}
export function useMobileLedger(options: { applyFilters?: () => boolean; accountId?: () => string; bookIds?: () => string[] } = {}) {
    const accounts = useAccountsStore();
    const categories = useTransactionCategoriesStore();
    const tags = useTransactionTagsStore();
    const books = useBooksStore(), scope = useLedgerScopeStore();
    const allEntries = ref<LedgerEntry[]>([]);
    const entries = computed(() => options.applyFilters?.() === false ? allEntries.value : allEntries.value.filter(item =>
        (!scope.accountId || item.accountId === scope.accountId || item.destinationAccountId === scope.accountId) &&
        (!scope.categoryId || item.categoryId === scope.categoryId || item.primaryCategoryId === scope.categoryId) &&
        (!scope.tagId || item.tagIds.includes(scope.tagId)) && (!scope.type || item.type === scope.type)));
    const displayEntries = computed(() => entries.value.filter(item => {
        const book = books.allBooks.find(book => book.id === item.bookId);
        return item.investment ? book?.showInvestments !== false : item.type !== 4 || book?.showTransfers !== false;
    }));
    const loading = ref(true);
    const error = ref('');
    const hasMore = ref(false);
    let nextTime = 0;
    let requestNumber = 0;
    const users = useUserStore();
    watch(() => users.currentUserBasicInfo?.username, () => {
        requestNumber++;
        allEntries.value = [];
        error.value = '';
        loading.value = true;
    }, { flush: 'sync' });
    onScopeDispose(() => { requestNumber++; });

    function normalize(item: TransactionInfoResponse): LedgerEntry {
        const destination = accounts.allAccountsMap[item.destinationAccountId] || item.destinationAccount;
        const account = accounts.allAccountsMap[item.sourceAccountId] || item.sourceAccount || (item.investmentEventId ? destination : undefined);
        const category = categories.allTransactionCategoriesMap[item.categoryId] || item.category;
        const parent = category?.parentId ? categories.allTransactionCategoriesMap[category.parentId] : undefined;
        const incoming = !!options.accountId && item.type === 4 && item.destinationAccountId === options.accountId();
        const currency = (incoming ? destination : account)?.currency || '';
        const amount = new LedgerDecimal((incoming ? item.destinationAmount : item.sourceAmount).toString()).div(100).toString();
        // Legacy daily transactions have no confirmed historical FX. Never substitute today's FX.
        const converted = currency === 'CNY' ? amount : null;
        return {
            id: item.id, type: item.type, excludeFromStatistics: !!item.excludeFromStatistics || (!!item.reimbursementAccountId && item.reimbursementAccountId !== '0'), reimbursementAccountId: item.reimbursementAccountId, reimbursementReceiptId: item.reimbursementReceiptId, day: moment.unix(item.time).tz(scope.timeZone).format('YYYY-MM-DD'), time: item.time,
            discountAmount: item.discountAmount || '0', transferFeeParentId:item.transferFeeParentId,
            title: item.investmentEventId ? '投资结算' : category?.name || ({ 1: '余额调整', 2: '收入', 3: '支出', 4: '账户转账' }[item.type] || '账户转账'),
            primaryCategory: parent?.name || category?.name || '未分类', primaryCategoryId: parent?.id || item.categoryId, categoryId: item.categoryId,
            bookId: item.bookId || '', accountId: item.sourceAccountId, destinationAccountId: item.destinationAccountId,
            tagIds: item.tagIds || [], investment: !!item.investmentEventId, investmentEventId: item.investmentEventId,
            account: `${account?.assetProfile?.shortName || account?.name || '账户'}${item.type === 4 && destination && !item.investmentEventId ? ` → ${destination.assetProfile?.shortName||destination.name}` : ''}`,
            currency, amount, cny: converted, hidden: item.hideAmount, comment: item.comment,
            tags: (item.tagIds || []).map(id => tags.allTransactionTagsMap[id]?.name).filter((name): name is string => !!name),
            bookName: books.allBooks.find(b=>b.id===item.bookId)?.name || '日常账本', icon: category?.icon, iconType: category?.iconType, color: category?.color,
            accountIcon: account?.icon, accountIconType: account?.iconType, pictures: item.pictures,
            location: item.locationName || (item.geoLocation ? `${item.geoLocation.latitude}, ${item.geoLocation.longitude}` : ''),
            reimbursementClosedAt: item.reimbursementClosedAt, destinationAccount: destination?.name, sourceAccountName:account?.name, destinationCurrency:destination?.currency, destinationAmount:item.type===4?new LedgerDecimal(item.destinationAmount).div(100).toString():undefined, transferFeeAmount:item.transferFeeAmount, debtDueDate:item.debtDueDate,
            transferDirection: options.accountId && item.type===4 ? incoming ? 'in' : 'out' : undefined
        };
    }
    async function load(startTime: number, endTime: number): Promise<void> {
        const version = ++requestNumber;
        const isCurrentScope = options.bookIds ? () => true : books.captureReportScope();
        loading.value = true;
        error.value = '';
        allEntries.value = [];
        try {
            const [, , , , response] = await Promise.all([
                accounts.loadAllAccounts({ force: true }).catch(keepUpToDate), categories.loadAllCategories({ force: false }).catch(keepUpToDate),
                tags.loadAllTags({ force: false }).catch(keepUpToDate), books.loadBooks(), services.getAllTransactions({ startTime: Math.max(0,startTime), endTime, withPictures: true, bookIds: options.bookIds?.() || books.selectedBookIds })
            ]);
            if (version !== requestNumber || !isCurrentScope()) return;
            if (!response.data.success) throw new Error('账单加载失败，请重试。');
            const items = response.data.result;
            if (version !== requestNumber || !isCurrentScope()) return;
            allEntries.value = items.map(normalize).sort((a, b) => b.time - a.time || b.id.localeCompare(a.id));
        } catch (cause) {
            if (version === requestNumber && isCurrentScope()) { allEntries.value = []; error.value = investmentError(cause); }
        } finally { if (version === requestNumber) loading.value = false; }
    }
    async function loadAccount(more=false):Promise<void> {
        if(more && (loading.value || !hasMore.value))return;
        const version=++requestNumber, accountId=options.accountId?.();
        if(!accountId)return;
        loading.value=true; error.value='';
        if(!more){allEntries.value=[];nextTime=0;hasMore.value=false;}
        try {
            if(!more)await Promise.all([accounts.loadAllAccounts({force:true}).catch(keepUpToDate),categories.loadAllCategories({force:false}).catch(keepUpToDate),tags.loadAllTags({force:false}).catch(keepUpToDate),books.loadBooks()]);
            const response=await services.getTransactions({bookIds:[],accountIds:accountId,maxTime:nextTime,minTime:0,count:50,page:1,withCount:false,type:0,categoryIds:'',tagFilter:'',amountFilter:'',keyword:'',matchMode:0});
            if(version!==requestNumber)return;
            if(!response.data.success)throw Error('账户流水加载失败，请重试。');
            const result=response.data.result, seen=new Set(allEntries.value.map(item=>item.id));
            allEntries.value.push(...result.items.filter(item=>!seen.has(item.id)).map(normalize));
            nextTime=result.nextTimeSequenceId||0;hasMore.value=nextTime>0;
        }catch(cause){if(version===requestNumber)error.value=investmentError(cause);}
        finally{if(version===requestNumber)loading.value=false;}
    }
    return { entries, displayEntries, loading, error, load, loadAccount, hasMore, normalize };
}
// Legacy stores report an unchanged force refresh by rejecting with this marker.
export function keepUpToDate(error: unknown): void {
    if (typeof error === 'object' && error !== null && 'isUpToDate' in error && error.isUpToDate === true) return;
    throw error;
}
