export interface AccountPreset {
    id: string;
    name: string;
    icon: string;
    color: string;
    category?: number;
    kind?: string;
    assetKind?: 'prepaid'|'secondhand'|'insurance'|'reimbursement';
    group?: string;
}

export const accountPresetGroups: { name: string; items: AccountPreset[] }[] = [
    { name: '资金账户', items: [
        { id: 'cash', name: '现金', category: 1, icon: '10', color: '#dfa82d' },
        { id: 'alipay', name: '支付宝', category: 4, icon: '8300', color: '#269add' },
        { id: 'wechat', name: '微信钱包', category: 4, icon: '8302', color: '#39a95b' },
        { id: 'bank', name: '银行卡', category: 2, icon: '100', color: '#d4a43b' },
        { id: 'qq', name: 'QQ钱包', category: 4, icon: '8301', color: '#5485b8' },
        { id: 'jd', name: '京东金融', category: 4, icon: '500', color: '#da5b58' },
        { id: 'savings', name: '储蓄账户', category: 8, icon: '30', color: '#58998a' },
        { id: 'cash-custom', name: '自定义', category: 2, icon: '1', color: '#6ca699' }
    ] },
    { name: '信贷账户', items: [
        { id: 'credit', name: '信用卡', category: 3, icon: '100', color: '#8b8ca5' },
        { id: 'huabei', name: '花呗', category: 3, icon: '8300', color: '#358edf' },
        { id: 'baitiao', name: '白条', category: 3, icon: '110', color: '#db6170' },
        { id: 'credit-custom', name: '自定义', category: 3, icon: '100', color: '#6ca699' }
    ] },
    { name: '预付账户', items: [
        {id:'member',name:'会员卡',category:4,assetKind:'prepaid',group:'预付账户',icon:'100',color:'#ca9954'},
        {id:'bus',name:'公交卡',category:4,assetKind:'prepaid',group:'预付账户',icon:'100',color:'#47a590'},
        {id:'meal',name:'饭卡',category:4,assetKind:'prepaid',group:'预付账户',icon:'100',color:'#cf8072'},
        {id:'deposit-card',name:'存款卡',category:4,assetKind:'prepaid',group:'预付账户',icon:'100',color:'#778cc3'},
        {id:'prepaid-custom',name:'自定义',category:4,assetKind:'prepaid',group:'预付账户',icon:'1',color:'#6ca699'}
    ] },
    { name: '二手资产', items: [
        {id:'house',name:'房产',category:7,assetKind:'secondhand',group:'二手资产',icon:'800',color:'#cb9868'},
        {id:'vehicle',name:'车辆',category:7,assetKind:'secondhand',group:'二手资产',icon:'800',color:'#648dbe'},
        {id:'property-custom',name:'自定义',category:7,assetKind:'secondhand',group:'二手资产',icon:'1',color:'#6ca699'}
    ] },
    { name: '投资理财', items: [
        {id:'insurance',name:'保险',category:7,assetKind:'insurance',group:'投资理财',icon:'800',color:'#a28bba'},
        { id: 'broker', name: '证券账户', kind: 'BROKER', icon: '801', color: '#648dbe' },
        { id: 'crypto', name: '加密货币交易所', kind: 'EXCHANGE', icon: '1500', color: '#bf82ca' },
        { id: 'wallet', name: '加密钱包', kind: 'WALLET', icon: '1', color: '#d49b48' },
        { id: 'deposit', name: '定期存款', category: 9, icon: '110', color: '#58998a' },
        { id: 'investment-custom', name: '自定义', kind: 'OTHER', icon: '800', color: '#6ca699' }
    ] },
    { name: '债务', items: [
        { id: 'payable', name: '应付款/借入', category: 5, icon: '600', color: '#d59d42' },
        {id:'reimbursement',name:'报销账户',category:6,assetKind:'reimbursement',group:'报销',icon:'700',color:'#6ca699'},
        { id: 'receivable', name: '应收款/借出', category: 6, icon: '700', color: '#6ca699' }
    ] }
];

export function findAccountPreset(id?: string): AccountPreset | undefined {
    return accountPresetGroups.flatMap(group => group.items).find(item => item.id === id);
}
