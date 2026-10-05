export interface CryptoPlatform { id: string; name: string; kind: 'EXCHANGE' | 'WALLET' }
export const cryptoPlatforms: CryptoPlatform[] = [
 { id:'binance', name:'币安 Binance', kind:'EXCHANGE' },
 { id:'okx', name:'欧易 OKX', kind:'EXCHANGE' },
 { id:'coinbase', name:'Coinbase', kind:'EXCHANGE' },
 { id:'kraken', name:'Kraken', kind:'EXCHANGE' },
 { id:'bybit', name:'Bybit', kind:'EXCHANGE' },
 { id:'bitget', name:'Bitget', kind:'EXCHANGE' },
 { id:'metamask', name:'MetaMask', kind:'WALLET' },
 { id:'trust', name:'Trust Wallet', kind:'WALLET' },
 { id:'phantom', name:'Phantom', kind:'WALLET' },
 { id:'rabby', name:'Rabby', kind:'WALLET' },
 { id:'ledger', name:'Ledger', kind:'WALLET' },
 { id:'trezor', name:'Trezor', kind:'WALLET' }
];
export const isCryptoAccount = (kind?: string): boolean => kind === 'EXCHANGE' || kind === 'WALLET';
export function platformIcon(id?: string): string | undefined { return cryptoPlatforms.some(p=>p.id===id) ? `img/crypto-platforms/${id}.svg` : undefined; }
export function platformName(id?: string): string { return cryptoPlatforms.find(p=>p.id===id)?.name || ''; }
