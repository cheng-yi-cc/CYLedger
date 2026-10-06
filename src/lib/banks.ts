export interface BankBrand { id: string; name: string; color: string; aliases: string[]; icon: string }

const bankData = [
    ['icbc','工商银行','#c84444','工行'], ['abchina','农业银行','#36927a','农行'],
    ['boc','中国银行','#bb4548','中行'], ['ccb','建设银行','#406eaa','建行'],
    ['bankcomm','交通银行','#465f9e','交行'], ['psbc','邮储银行','#419865','邮政储蓄银行,邮政储蓄,邮储'],
    ['cmbchina','招商银行','#bf434c','招行'], ['spdb','浦发银行','#537cb8','上海浦东发展银行,浦发'],
    ['cib','兴业银行','#44619b',''], ['cmbc','民生银行','#4b8c82',''],
    ['citicbank','中信银行','#cf5665',''], ['cebbank','光大银行','#b186b7',''],
    ['pingan','平安银行','#da8745',''], ['hxb','华夏银行','#c25c64',''],
    ['cgbchina','广发银行','#c44c50','广东发展银行'], ['bankofbeijing','北京银行','#c65464',''],
    ['bosc','上海银行','#d0a361',''], ['jsbchina','江苏银行','#739bae',''],
    ['nbcb','宁波银行','#d79056','']
] as const;

export const BANK_BRANDS: BankBrand[] = bankData.map(([id,name,color,aliases],index) => ({id,name,color,aliases:aliases.split(',').filter(Boolean),icon:String(200+index)}));
export function bankLogo(icon: string | number): string | undefined {
    const bank = BANK_BRANDS.find(bank => bank.icon === String(icon));
    return bank ? `img/banks/${bank.id}.svg` : undefined;
}
export function matchingBank(name: string): BankBrand | undefined {
    const normalized = name.replace(/\s+/g,'');
    const matches = BANK_BRANDS.filter(bank => [bank.name,...bank.aliases].some(alias => normalized.includes(alias)));
    return matches.length === 1 ? matches[0] : undefined;
}
export function automaticBankIcon(name: string, category: number, iconType: number): string | undefined {
    return [2,3].includes(category) && iconType === 0 ? matchingBank(name)?.icon : undefined;
}
