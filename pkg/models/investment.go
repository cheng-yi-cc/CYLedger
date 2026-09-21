package models

// Investment tables keep decimal values in JSON/text, never SQLite floating point.
type PortfolioAccount struct {
	Id   string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid  int64  `xorm:"INDEX NOT NULL" json:"-"`
	Name string `xorm:"VARCHAR(64) NOT NULL" json:"name"`
	Kind string `xorm:"VARCHAR(32) NOT NULL" json:"kind"`
}

type InvestmentInstrument struct {
	Id        string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid       int64  `xorm:"INDEX NOT NULL" json:"-"`
	Type      string `xorm:"VARCHAR(16) NOT NULL" json:"type"`
	Symbol    string `xorm:"VARCHAR(24) NOT NULL" json:"symbol"`
	Name      string `xorm:"VARCHAR(64) NOT NULL" json:"name"`
	Precision int    `json:"precision"`
}

type InvestmentSettings struct {
	Uid          int64  `xorm:"PK" json:"-"`
	BaseCurrency string `xorm:"VARCHAR(3) NOT NULL" json:"baseCurrency"`
	TimeZone     string `xorm:"VARCHAR(64) NOT NULL" json:"timeZone"`
	Revision     int64  `json:"-"`
}

type InvestmentEventRecord struct {
	Id         string `xorm:"VARCHAR(64) PK"`
	Uid        int64  `xorm:"INDEX NOT NULL"`
	OccurredAt int64  `xorm:"INDEX NOT NULL"`
	Version    int    `xorm:"NOT NULL"`
	Voided     bool   `xorm:"NOT NULL"`
	Payload    string `xorm:"TEXT NOT NULL"`
}

type InvestmentEventRevision struct {
	Id         string `xorm:"VARCHAR(64) PK"`
	Uid        int64  `xorm:"INDEX NOT NULL"`
	EventId    string `xorm:"VARCHAR(64) INDEX NOT NULL"`
	Version    int
	RecordedAt int64
	Payload    string `xorm:"TEXT NOT NULL"`
}

type InvestmentTransactionLink struct {
	TransactionId int64  `xorm:"PK"`
	Uid           int64  `xorm:"INDEX NOT NULL"`
	EventId       string `xorm:"VARCHAR(64) INDEX NOT NULL"`
}

type InvestmentIdempotency struct {
	Id         string `xorm:"VARCHAR(64) PK"`
	Uid        int64  `xorm:"UNIQUE(investment_request) NOT NULL"`
	RequestKey string `xorm:"VARCHAR(128) UNIQUE(investment_request) NOT NULL"`
	Digest     string `xorm:"VARCHAR(64) NOT NULL"`
	Response   string `xorm:"TEXT NOT NULL"`
}

type InvestmentQuote struct {
	Id           string `xorm:"VARCHAR(128) PK"`
	Uid          int64  `xorm:"INDEX NOT NULL"`
	InstrumentId string `xorm:"VARCHAR(64) NOT NULL"`
	Payload      string `xorm:"TEXT NOT NULL"`
}

type WealthSnapshot struct {
	Id           string  `xorm:"VARCHAR(64) PK" json:"id"`
	Uid          int64   `xorm:"INDEX NOT NULL" json:"-"`
	RecordedAt   int64   `xorm:"INDEX NOT NULL" json:"recordedAt"`
	NetAssets    *string `xorm:"TEXT" json:"netAssets"`
	ValuedAssets string  `xorm:"TEXT" json:"valuedAssets"`
	Complete     bool    `json:"complete"`
	Invalidated  bool    `json:"invalidated"`
	Payload      string  `xorm:"TEXT NOT NULL" json:"-"`
}
