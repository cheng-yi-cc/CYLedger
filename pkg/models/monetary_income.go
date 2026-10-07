package models

// A binding is explicit consent to create local income transactions.
type MonetaryIncomeBinding struct {
	Id                 string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid                int64  `xorm:"UNIQUE(UQE_monetary_account) NOT NULL" json:"-"`
	AccountId          int64  `xorm:"UNIQUE(UQE_monetary_account) NOT NULL" json:"accountId,string"`
	Code               string `xorm:"VARCHAR(6) NOT NULL" json:"code"`
	Name               string `xorm:"VARCHAR(120) NOT NULL" json:"name"`
	StartDate          string `xorm:"VARCHAR(10) NOT NULL" json:"startDate"`
	NextDate           string `xorm:"VARCHAR(10) NOT NULL" json:"nextDate"`
	BookId             string `xorm:"VARCHAR(64) NOT NULL" json:"bookId"`
	TimeZone           string `xorm:"VARCHAR(64) NOT NULL" json:"timeZone"`
	CategoryId         int64  `json:"categoryId,string"`
	LastTransactionId  int64  `json:"lastTransactionId,string"`
	Enabled            bool   `json:"enabled"`
	Status             string `xorm:"VARCHAR(200)" json:"status"`
	LastAttempt        int64  `json:"lastAttempt"`
	TotalIncome        string `xorm:"-" json:"totalIncome"`
	LastPerTenThousand string `xorm:"-" json:"lastPerTenThousand"`
}

// Rows survive edits/deletion of the ordinary transaction, preventing recreation.
type MonetaryIncomeDay struct {
	Id             string `xorm:"VARCHAR(100) PK" json:"id"`
	Uid            int64  `xorm:"INDEX NOT NULL" json:"-"`
	AccountId      int64  `xorm:"NOT NULL" json:"accountId,string"`
	Date           string `xorm:"VARCHAR(10) NOT NULL" json:"date"`
	Code           string `xorm:"VARCHAR(6) NOT NULL" json:"code"`
	Principal      string `xorm:"VARCHAR(40) NOT NULL" json:"principal"`
	PerTenThousand string `xorm:"VARCHAR(32) NOT NULL" json:"perTenThousand"`
	Amount         string `xorm:"VARCHAR(40) NOT NULL" json:"amount"`
	TransactionId  int64  `json:"transactionId,string"`
}

type MonetaryIncomeSaveRequest struct {
	AccountId  string `json:"accountId" binding:"required,max=24"`
	Code       string `json:"code" binding:"required,len=6"`
	StartDate  string `json:"startDate" binding:"required,len=10"`
	BookId     string `json:"bookId" binding:"max=64"`
	TimeZone   string `json:"timeZone" binding:"required,max=64"`
	CategoryId string `json:"categoryId" binding:"max=24"`
	Enabled    bool   `json:"enabled"`
}
