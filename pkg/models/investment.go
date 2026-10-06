package models

// Investment tables keep decimal values in JSON/text, never SQLite floating point.
type PortfolioAccount struct {
	Id                 string   `xorm:"VARCHAR(64) PK" json:"id"`
	Uid                int64    `xorm:"INDEX NOT NULL" json:"-"`
	Name               string   `xorm:"VARCHAR(64) NOT NULL" json:"name"`
	Kind               string   `xorm:"VARCHAR(32) NOT NULL" json:"kind"`
	Platform           string   `xorm:"VARCHAR(32)" json:"platform,omitempty"`
	Instruments        []string `xorm:"TEXT" json:"instruments,omitempty"`
	Currency           string   `xorm:"VARCHAR(3) NOT NULL DEFAULT 'CNY'" json:"currency"`
	PaymentInstruments []string `xorm:"TEXT" json:"paymentInstruments"`
}

type InvestmentInstrument struct {
	Id         string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid        int64  `xorm:"INDEX NOT NULL" json:"-"`
	Type       string `xorm:"VARCHAR(16) NOT NULL" json:"type"`
	Symbol     string `xorm:"VARCHAR(24) NOT NULL" json:"symbol"`
	Name       string `xorm:"VARCHAR(64) NOT NULL" json:"name"`
	Precision  int    `json:"precision"`
	Market     string `xorm:"VARCHAR(16)" json:"market,omitempty"`
	Provider   string `xorm:"VARCHAR(24)" json:"provider,omitempty"`
	ProviderID string `xorm:"VARCHAR(80)" json:"providerId,omitempty"`
	Currency   string `xorm:"VARCHAR(3)" json:"currency,omitempty"`
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

// 持仓偏好不保存第二份余额；成本和数量始终从投资事实回放。
type InvestmentHoldingProfile struct {
	Id               string   `xorm:"VARCHAR(64) PK" json:"id"`
	Uid              int64    `xorm:"UNIQUE(holding_profile) NOT NULL" json:"-"`
	AccountId        string   `xorm:"VARCHAR(64) UNIQUE(holding_profile) NOT NULL" json:"accountId"`
	InstrumentId     string   `xorm:"VARCHAR(64) UNIQUE(holding_profile) NOT NULL" json:"instrumentId"`
	Name             string   `xorm:"VARCHAR(64)" json:"name"`
	Group            string   `xorm:"VARCHAR(64)" json:"group"`
	Note             string   `xorm:"TEXT" json:"note"`
	ProfitOffset     string   `xorm:"TEXT" json:"profitOffset"`
	Hidden           bool     `json:"hidden"`
	ExcludeFromTotal bool     `json:"excludeFromTotal"`
	ExcludeProfit    bool     `json:"excludeProfit"`
	BookIds          []string `xorm:"TEXT" json:"bookIds"`
	Version          int      `json:"version"`
}

// 定投和待确认记录是指令。只有入账事务成功才产生投资事实。
type InvestmentPlan struct {
	Id            string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid           int64  `xorm:"INDEX NOT NULL" json:"-"`
	AccountId     string `xorm:"VARCHAR(64)" json:"accountId"`
	InstrumentId  string `xorm:"VARCHAR(64)" json:"instrumentId"`
	CashAccountId string `xorm:"VARCHAR(32)" json:"cashAccountId"`
	BookId        string `xorm:"VARCHAR(64)" json:"bookId"`
	Amount        string `xorm:"TEXT" json:"amount"`
	FeePercent    string `xorm:"TEXT" json:"feePercent"`
	Cycle         string `xorm:"VARCHAR(16)" json:"cycle"`
	StartDate     string `xorm:"VARCHAR(10)" json:"startDate"`
	NextDate      string `xorm:"VARCHAR(10)" json:"nextDate"`
	EndDate       string `xorm:"VARCHAR(10)" json:"endDate"`
	Time          string `xorm:"VARCHAR(5)" json:"time"`
	TimeZone      string `xorm:"VARCHAR(64)" json:"timeZone"`
	Note          string `xorm:"TEXT" json:"note"`
	Paused        bool   `json:"paused"`
	Deleted       bool   `json:"deleted"`
	Version       int    `json:"version"`
}

type InvestmentOrder struct {
	Id            string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid           int64  `xorm:"UNIQUE(investment_order_key) INDEX NOT NULL" json:"-"`
	RequestKey    string `xorm:"VARCHAR(128) UNIQUE(investment_order_key) NOT NULL" json:"-"`
	PlanId        string `xorm:"VARCHAR(64) INDEX" json:"planId"`
	AccountId     string `xorm:"VARCHAR(64)" json:"accountId"`
	InstrumentId  string `xorm:"VARCHAR(64)" json:"instrumentId"`
	CashAccountId string `xorm:"VARCHAR(32)" json:"cashAccountId"`
	BookId        string `xorm:"VARCHAR(64)" json:"bookId"`
	Type          string `xorm:"VARCHAR(16)" json:"type"`
	Amount        string `xorm:"TEXT" json:"amount"`
	Quantity      string `xorm:"TEXT" json:"quantity"`
	Fee           string `xorm:"TEXT" json:"fee"`
	FeePercent    string `xorm:"TEXT" json:"feePercent"`
	TradeDate     string `xorm:"VARCHAR(10)" json:"tradeDate"`
	ConfirmDate   string `xorm:"VARCHAR(10)" json:"confirmDate"`
	Time          string `xorm:"VARCHAR(5)" json:"time"`
	TimeZone      string `xorm:"VARCHAR(64)" json:"timeZone"`
	Note          string `xorm:"TEXT" json:"note"`
	Status        string `xorm:"VARCHAR(16)" json:"status"`
	EventId       string `xorm:"VARCHAR(64)" json:"eventId"`
	Price         string `xorm:"TEXT" json:"price"`
	PriceDate     string `xorm:"VARCHAR(10)" json:"priceDate"`
	Error         string `xorm:"TEXT" json:"error"`
	LastAttempt   int64  `json:"-"`
	Version       int    `json:"version"`
}
