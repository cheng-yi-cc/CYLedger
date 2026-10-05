export interface LedgerBook {
    id: string;
    name: string;
    icon: string;
    displayOrder: number;
    isDefault: boolean;
    archived: boolean;
    showTransfers: boolean;
    showInvestments: boolean;
}
