package models

import "encoding/json"

// LocalLedgerItem stores local planning/rules, never a second account balance.
type LocalLedgerItem struct {
	Id              string `xorm:"PK VARCHAR(64)"`
	Uid             int64  `xorm:"INDEX NOT NULL"`
	Kind            string `xorm:"VARCHAR(16) INDEX NOT NULL"`
	Revision        int64
	Payload         string `xorm:"TEXT"`
	UpdatedUnixTime int64
}
type LedgerWorkspaceItem struct {
	Id       string          `json:"id" binding:"max=64"`
	Kind     string          `json:"kind" binding:"required,max=16"`
	Revision int64           `json:"revision,string" binding:"min=0,max=1000000000"`
	Data     json.RawMessage `json:"data"`
}
type LedgerWishLog struct {
	Id     string `json:"id"`
	Date   string `json:"date"`
	Amount string `json:"amount"`
	Note   string `json:"note"`
}
type LedgerWish struct {
	Name               string          `json:"name"`
	Icon               string          `json:"icon"`
	Target             string          `json:"target"`
	Initial            string          `json:"initial"`
	StartDate          string          `json:"startDate"`
	EndDate            string          `json:"endDate"`
	Mode               string          `json:"mode"`
	Cycle              string          `json:"cycle"`
	Amount             string          `json:"amount"`
	Ratio              string          `json:"ratio"`
	BookIds            []string        `json:"bookIds"`
	AccountIds         []string        `json:"accountIds"`
	IncomeCategoryIds  []string        `json:"incomeCategoryIds"`
	ExpenseCategoryIds []string        `json:"expenseCategoryIds"`
	Note               string          `json:"note"`
	Archived           bool            `json:"archived"`
	Logs               []LedgerWishLog `json:"logs"`
}
type LedgerKeywordRule struct {
	Keywords   []string `json:"keywords"`
	CategoryId string   `json:"categoryId"`
	AccountId  string   `json:"accountId"`
	TagIds     []string `json:"tagIds"`
	Enabled    bool     `json:"enabled"`
}
