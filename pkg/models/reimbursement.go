package models

// The original expense remains the single spending fact. A receipt points to
// its actual cash income; receivables are derived from these two facts.
type ReimbursementReceipt struct {
	Id            string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid           int64  `xorm:"INDEX NOT NULL" json:"-"`
	ExpenseId     int64  `xorm:"INDEX NOT NULL" json:"expenseId,string"`
	IncomeId      int64  `xorm:"UNIQUE NOT NULL" json:"incomeId,string"`
	RequestId     string `xorm:"VARCHAR(64) UNIQUE(UQE_reimbursement_request) NOT NULL" json:"-"`
	RequestUid    int64  `xorm:"UNIQUE(UQE_reimbursement_request) NOT NULL" json:"-"`
	RequestDigest string `xorm:"VARCHAR(64) NOT NULL" json:"-"`
}
