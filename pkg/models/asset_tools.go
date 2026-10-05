package models

// AssetPresentation affects visibility and account choices, never ledger balances.
type AssetPresentation struct {
	Uid      int64  `xorm:"PK" json:"-"`
	Revision int64  `json:"-"`
	Payload  string `xorm:"TEXT NOT NULL" json:"-"`
}

type AssetAccountRule struct {
	Hidden        bool     `json:"hidden"`
	DisabledBooks []string `json:"disabledBooks"`
}
type AssetReminderSettings struct {
	Enabled     bool `json:"enabled"`
	Credit      bool `json:"credit"`
	Debt        bool `json:"debt"`
	Deposit     bool `json:"deposit"`
	AdvanceDays int  `json:"advanceDays"`
	MinuteOfDay int  `json:"minuteOfDay"`
}
type AssetPreferences struct {
	Revision  string                      `json:"revision"`
	Rules     map[string]AssetAccountRule `json:"rules"`
	Reminders AssetReminderSettings       `json:"reminders"`
}

// A deposit reserves an existing part of an account balance. Only its earned
// interest creates an income fact, with a permanent settlement identity.
type FixedDeposit struct {
	ReceivedInterest string `xorm:"-" json:"receivedInterest"`
	ClosedDate       string `xorm:"VARCHAR(10) NOT NULL DEFAULT ''" json:"closedDate"`
	Id               string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid              int64  `xorm:"INDEX NOT NULL" json:"-"`
	AccountId        string `xorm:"VARCHAR(32) NOT NULL" json:"accountId"`
	BookId           string `xorm:"VARCHAR(64) NOT NULL" json:"bookId"`
	CategoryId       string `xorm:"VARCHAR(32) NOT NULL" json:"categoryId"`
	Currency         string `xorm:"VARCHAR(3) NOT NULL" json:"currency"`
	Principal        string `xorm:"VARCHAR(32) NOT NULL" json:"principal"`
	AnnualRate       string `xorm:"VARCHAR(32) NOT NULL" json:"annualRate"`
	Term             int    `json:"term"`
	Unit             string `xorm:"VARCHAR(8) NOT NULL" json:"unit"`
	StartDate        string `xorm:"VARCHAR(10) NOT NULL" json:"startDate"`
	MaturityDate     string `xorm:"VARCHAR(10) NOT NULL" json:"maturityDate"`
	ExpectedInterest string `xorm:"VARCHAR(32) NOT NULL" json:"expectedInterest"`
	TimeZone         string `xorm:"VARCHAR(64) NOT NULL" json:"timeZone"`
	Note             string `xorm:"VARCHAR(200) NOT NULL" json:"note"`
	Settled          bool   `json:"settled"`
	TransactionId    string `xorm:"VARCHAR(32)" json:"transactionId"`
	Closed           bool   `json:"closed"`
}
