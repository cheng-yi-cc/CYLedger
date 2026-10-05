package models

// AccountAssetProfile describes an account without creating a second balance.
// Empty fields retain the legacy category's presentation and behaviour.
type AccountAssetProfile struct {
	Kind               string `json:"kind" binding:"max=20"`
	Group              string `json:"group" binding:"max=40"`
	ShortName          string `json:"shortName" binding:"max=24"`
	CardNumber         string `json:"cardNumber" binding:"max=64"`
	NightIcon          string `json:"nightIcon" binding:"max=24"`
	NightIconType      int    `json:"nightIconType" binding:"min=0,max=1"`
	NightColor         string `json:"nightColor" binding:"omitempty,len=6,validHexRGBColor"`
	ExcludeFromTotal   bool   `json:"excludeFromTotal"`
	RepaymentDay       int    `json:"repaymentDay" binding:"min=0,max=31"`
	RepaymentAfterDays int    `json:"repaymentAfterDays" binding:"min=0,max=60"`
	StatementNextCycle bool   `json:"statementNextCycle"`
	SharedLimitAccount string `json:"sharedLimitAccount" binding:"max=24"`
	AnnualFee          string `json:"annualFee" binding:"max=20"`
	AnnualFeeDate      string `json:"annualFeeDate" binding:"max=10"`
	AnnualWaiverAmount string `json:"annualWaiverAmount" binding:"max=20"`
	AnnualWaiverCount  int    `json:"annualWaiverCount" binding:"min=0,max=10000"`
	DebtDueDate        string `json:"debtDueDate" binding:"max=10"`
}

func (a *Account) AssetProfile() *AccountAssetProfile {
	if a.Extend == nil {
		return nil
	}
	return a.Extend.AssetProfile
}

func (a *Account) IsReimbursement() bool {
	return a.AssetProfile() != nil && a.AssetProfile().Kind == "reimbursement"
}

func (a *Account) ExcludedFromTotal() bool {
	return a.AssetProfile() != nil && a.AssetProfile().ExcludeFromTotal
}
