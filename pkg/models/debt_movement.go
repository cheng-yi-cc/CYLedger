package models

// DebtMovement links the optional interest entry to its principal movement.
// Amounts and balances are always read from the ordinary transaction ledger.
type DebtMovement struct {
	Id                     string `xorm:"PK VARCHAR(80)" json:"id"`
	Uid                    int64  `xorm:"INDEX NOT NULL" json:"-"`
	DebtAccountId          int64  `xorm:"INDEX NOT NULL" json:"debtAccountId,string"`
	Action                 string `xorm:"VARCHAR(16)" json:"action"`
	PrincipalTransactionId int64  `json:"principalTransactionId,string"`
	InterestTransactionId  int64  `json:"interestTransactionId,string"`
	Digest                 string `xorm:"VARCHAR(64)" json:"-"`
}
