package models

// Book groups transaction facts. Accounts and their balances remain user-wide.
type Book struct {
	Id              string `xorm:"VARCHAR(64) PK" json:"id"`
	Uid             int64  `xorm:"INDEX NOT NULL" json:"-"`
	Name            string `xorm:"VARCHAR(64) NOT NULL" json:"name"`
	Icon            string `xorm:"VARCHAR(32) NOT NULL" json:"icon"`
	DisplayOrder    int32  `json:"displayOrder"`
	Archived        bool   `json:"archived"`
	IsDefault       bool   `json:"isDefault"`
	ShowTransfers   bool   `json:"showTransfers"`
	ShowInvestments bool   `json:"showInvestments"`
}

type BookCreateRequest struct {
	Name string `json:"name" binding:"required,notBlank,max=64"`
	Icon string `json:"icon" binding:"max=32"`
}

type BookModifyRequest struct {
	Id              string  `json:"id" binding:"required,max=64"`
	Name            *string `json:"name" binding:"omitempty,notBlank,max=64"`
	Icon            *string `json:"icon" binding:"omitempty,max=32"`
	DisplayOrder    *int32  `json:"displayOrder"`
	Archived        *bool   `json:"archived"`
	IsDefault       *bool   `json:"isDefault"`
	ShowTransfers   *bool   `json:"showTransfers"`
	ShowInvestments *bool   `json:"showInvestments"`
}

type BookMoveRequest struct {
	TransactionIds []string `json:"transactionIds" binding:"required,min=1,max=1000"`
	BookId         string   `json:"bookId" binding:"required,max=64"`
}
