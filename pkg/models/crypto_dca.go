package models

// CryptoDCAPlan is explicit consent to record reference-price purchases locally.
// NextDate is a calendar date in TimeZone, never a rolling 24-hour interval.
type CryptoDCAPlan struct {
	Id                  string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid                 int64  `xorm:"INDEX NOT NULL" json:"-"`
	AccountId           string `xorm:"VARCHAR(64) INDEX NOT NULL" json:"accountId"`
	InstrumentId        string `xorm:"VARCHAR(64) NOT NULL" json:"instrumentId"`
	PaymentInstrumentId string `xorm:"VARCHAR(64) NOT NULL" json:"paymentInstrumentId"`
	Amount              string `xorm:"VARCHAR(128) NOT NULL" json:"amount"`
	DailyTime           string `xorm:"VARCHAR(5) NOT NULL" json:"dailyTime"`
	StartDate           string `xorm:"VARCHAR(10) NOT NULL" json:"startDate"`
	NextDate            string `xorm:"VARCHAR(10) NOT NULL" json:"nextDate"`
	TimeZone            string `xorm:"VARCHAR(64) NOT NULL" json:"timeZone"`
	BookId              string `xorm:"VARCHAR(64) NOT NULL" json:"bookId"`
	Enabled             bool   `json:"enabled"`
	Revision            int64  `json:"revision"`
	Status              string `xorm:"VARCHAR(200)" json:"status"`
	LastAttempt         int64  `json:"lastAttempt"`
}

// A successful day survives revisions/voids of its event. Both are committed
// together, so retries and restored backups cannot buy the same day twice.
type CryptoDCADay struct {
	Id                  string `xorm:"VARCHAR(100) PK" json:"id"`
	Uid                 int64  `xorm:"INDEX NOT NULL" json:"-"`
	PlanId              string `xorm:"VARCHAR(64) INDEX NOT NULL" json:"planId"`
	Date                string `xorm:"VARCHAR(10) NOT NULL" json:"date"`
	ScheduledAt         int64  `xorm:"INDEX NOT NULL" json:"scheduledAt"`
	PlanRevision        int64  `json:"-"`
	AccountId           string `xorm:"VARCHAR(64) NOT NULL" json:"accountId"`
	InstrumentId        string `xorm:"VARCHAR(64) NOT NULL" json:"instrumentId"`
	PaymentInstrumentId string `xorm:"VARCHAR(64) NOT NULL" json:"paymentInstrumentId"`
	Amount              string `xorm:"VARCHAR(128) NOT NULL" json:"amount"`
	BookId              string `xorm:"VARCHAR(64) NOT NULL" json:"-"`
	Status              string `xorm:"VARCHAR(16) NOT NULL" json:"status"`
	Message             string `xorm:"VARCHAR(200)" json:"message"`
	LastAttempt         int64  `json:"lastAttempt"`
	EventId             string `xorm:"VARCHAR(64)" json:"eventId"`
}

type CryptoDCASaveRequest struct {
	RequestKey          string `json:"requestKey" binding:"max=64"`
	Id                  string `json:"id" binding:"max=64"`
	Revision            int64  `json:"revision"`
	AccountId           string `json:"accountId" binding:"required,max=64"`
	InstrumentId        string `json:"instrumentId" binding:"required,max=64"`
	PaymentInstrumentId string `json:"paymentInstrumentId" binding:"required,max=64"`
	Amount              string `json:"amount" binding:"required,max=128"`
	DailyTime           string `json:"dailyTime" binding:"required,len=5"`
	StartDate           string `json:"startDate" binding:"required,len=10"`
	TimeZone            string `json:"timeZone" binding:"required,max=64"`
	BookId              string `json:"bookId" binding:"max=64"`
}
