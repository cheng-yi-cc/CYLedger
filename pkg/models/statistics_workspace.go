package models

// StatisticsBudget is a spending limit, never an account balance or transaction.
type StatisticsBudget struct {
	Id         string `xorm:"PK VARCHAR(64)" json:"id"`
	Uid        int64  `xorm:"INDEX NOT NULL" json:"-"`
	BookId     string `xorm:"VARCHAR(64) INDEX" json:"bookId"`
	CategoryId int64  `json:"categoryId,string"`
	Name       string `xorm:"VARCHAR(64)" json:"name"`
	Amount     string `xorm:"VARCHAR(32)" json:"amount"`
	StartDate  string `xorm:"VARCHAR(10)" json:"startDate"`
	EndDate    string `xorm:"VARCHAR(10)" json:"endDate"`
	Kind       string `xorm:"VARCHAR(16)" json:"kind"`
	Repeat     bool   `json:"repeat"`
	Revision   int64  `json:"revision,string"`
}

type StatisticsNote struct {
	Id       string `xorm:"PK VARCHAR(64)" json:"id"`
	Uid      int64  `xorm:"INDEX NOT NULL" json:"-"`
	BookId   string `xorm:"VARCHAR(64)" json:"bookId"`
	Period   string `xorm:"VARCHAR(7)" json:"period"`
	Content  string `xorm:"TEXT" json:"content"`
	Revision int64  `json:"revision,string"`
}

type StatisticsPreference struct {
	Uid      int64 `xorm:"PK"`
	Revision int64
	Payload  string `xorm:"TEXT"`
}

type StatisticsModule struct {
	Id      string `json:"id"`
	Visible bool   `json:"visible"`
}

type StatisticsPreferences struct {
	Revision        string                        `json:"revision"`
	Modules         map[string][]StatisticsModule `json:"modules"`
	CarrySurplus    bool                          `json:"carrySurplus"`
	CarryDeficit    bool                          `json:"carryDeficit"`
	DailyBudgetMode string                        `json:"dailyBudgetMode"`
	BudgetProgress  string                        `json:"budgetProgress"`
}

type StatisticsDeleteRequest struct {
	Id       string `json:"id" binding:"required,max=64"`
	Revision int64  `json:"revision,string" binding:"min=1"`
}

// Only identifiers are needed to classify linked fees and principal movements.
type StatisticsAuxiliary struct {
	DebtActions map[string]string `json:"debtActions"`
	FeeIds      []string          `json:"feeIds"`
}
