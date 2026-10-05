package models

// AssetAdjustment keeps a retry from creating a second balance correction.
type AssetAdjustment struct {
	Id            string `xorm:"PK VARCHAR(80)" json:"id"`
	Uid           int64  `xorm:"INDEX NOT NULL" json:"-"`
	Digest        string `xorm:"VARCHAR(64) NOT NULL" json:"-"`
	TransactionId int64  `json:"transactionId,string"`
}

func (a *AssetAdjustment) TableName() string { return "asset_adjustment" }
