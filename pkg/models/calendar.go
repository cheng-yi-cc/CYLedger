package models

// CalendarEvent is a manually maintained due item, never an accounting fact.
type CalendarEvent struct {
	Id          string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid         int64  `xorm:"INDEX NOT NULL" json:"-"`
	BookId      string `xorm:"VARCHAR(64) INDEX NOT NULL" json:"bookId"`
	AccountId   string `xorm:"VARCHAR(32) NOT NULL" json:"accountId"`
	AccountName string `xorm:"VARCHAR(64) NOT NULL" json:"accountName"`
	Currency    string `xorm:"VARCHAR(3) NOT NULL" json:"currency"`
	Kind        string `xorm:"VARCHAR(16) NOT NULL" json:"kind"`
	Date        string `xorm:"VARCHAR(10) INDEX NOT NULL" json:"date"`
	Amount      string `xorm:"VARCHAR(32) NOT NULL" json:"amount"`
	Note        string `xorm:"VARCHAR(200) NOT NULL" json:"note"`
	Completed   bool   `json:"completed"`
}

type CalendarSaveRequest struct {
	Id        string `json:"id" binding:"max=64"`
	BookId    string `json:"bookId" binding:"required,max=64"`
	AccountId string `json:"accountId" binding:"required,max=32"`
	Kind      string `json:"kind" binding:"required,max=16"`
	Date      string `json:"date" binding:"required,len=10"`
	Amount    string `json:"amount" binding:"required,max=32"`
	Note      string `json:"note" binding:"max=200"`
}
