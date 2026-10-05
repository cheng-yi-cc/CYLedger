package models

import "encoding/json"

// Principal is already present in the original ledger. A plan only schedules
// repayment; an explicitly agreed service fee becomes a separate expense.
type CreditInstallment struct {
	Id             string                 `xorm:"PK VARCHAR(64)" json:"id"`
	Uid            int64                  `xorm:"INDEX NOT NULL" json:"-"`
	AccountId      int64                  `xorm:"INDEX NOT NULL" json:"accountId,string"`
	ExpenseId      int64                  `xorm:"INDEX NOT NULL DEFAULT 0" json:"expenseId,string"`
	StatementMonth string                 `xorm:"VARCHAR(7)" json:"statementMonth"`
	Version        int64                  `json:"version,string"`
	Closed         bool                   `json:"closed"`
	RequestDigest  string                 `xorm:"VARCHAR(64)" json:"-"`
	Data           *CreditInstallmentData `xorm:"BLOB" json:"data"`
}
type CreditInstallmentData struct {
	Principal string                     `json:"principal"`
	TotalFee  string                     `json:"totalFee"`
	Periods   int                        `json:"periods"`
	FirstDate string                     `json:"firstDate"`
	Method    string                     `json:"method"`
	Remainder string                     `json:"remainder"`
	BookId    string                     `json:"bookId"`
	TimeZone  string                     `json:"timeZone"`
	Note      string                     `json:"note"`
	Payments  []CreditInstallmentPayment `json:"payments"`
}
type CreditInstallmentPayment struct {
	Date             string `json:"date"`
	Principal        string `json:"principal"`
	Fee              string `json:"fee"`
	Accrued          bool   `json:"accrued"`
	FeeTransactionId string `json:"feeTransactionId"`
}

func (d *CreditInstallmentData) FromDB(data []byte) error { return json.Unmarshal(data, d) }
func (d *CreditInstallmentData) ToDB() ([]byte, error)    { return json.Marshal(d) }
