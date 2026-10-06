package services

import (
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

func feeFixture(t *testing.T) (*investmentDBFixture, *models.Transaction) {
	f := newInvestmentDBFixture(t)
	_, err := f.engine.Insert(&models.Account{AccountId: 1003, Uid: f.uid, Name: "转入", Type: models.ACCOUNT_TYPE_SINGLE_ACCOUNT, Category: models.ACCOUNT_CATEGORY_CASH, Currency: "CNY"},
		&models.TransactionCategory{CategoryId: 501, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, Name: "转账"}, &models.TransactionCategory{CategoryId: 502, Uid: f.uid, Type: models.CATEGORY_TYPE_TRANSFER, ParentCategoryId: 501, Name: "账户互转"},
		&models.TransactionCategory{CategoryId: 601, Uid: f.uid, Type: models.CATEGORY_TYPE_EXPENSE, Name: "费用"}, &models.TransactionCategory{CategoryId: 602, Uid: f.uid, Type: models.CATEGORY_TYPE_EXPENSE, ParentCategoryId: 601, Name: "手续费"},
		&models.TransactionCategory{CategoryId: 701, Uid: f.uid, Type: models.CATEGORY_TYPE_INCOME, Name: "收入"}, &models.TransactionCategory{CategoryId: 702, Uid: f.uid, Type: models.CATEGORY_TYPE_INCOME, ParentCategoryId: 701, Name: "优惠"})
	require.NoError(t, err)
	return f, &models.Transaction{Uid: f.uid, AccountId: f.bankID, RelatedAccountId: 1003, Type: models.TRANSACTION_DB_TYPE_TRANSFER_OUT, CategoryId: 502, Amount: 10000, RelatedAccountAmount: 10000, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 1), TransferFeeAmount: "1.23", TransferFeeCategoryId: 602}
}
func TestTransferFeeAtomicCreateModifyMoveDelete(t *testing.T) {
	f, tx := feeFixture(t)
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	require.Equal(t, int64(1989877), f.balance())
	var fee models.Transaction
	has, err := f.engine.Where("uid=? AND transfer_fee_parent_id=? AND deleted=?", f.uid, tx.TransactionId, false).Get(&fee)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, int64(123), fee.Amount)
	require.Error(t, Transactions.DeleteTransaction(nil, f.uid, fee.TransactionId))
	fee.Amount = 200
	require.Error(t, Transactions.ModifyTransaction(nil, &fee, false, 0, nil, nil, nil, nil))
	book, err := Books.Create(nil, f.uid, models.BookCreateRequest{Name: "旅途"})
	require.NoError(t, err)
	require.NoError(t, Books.MoveTransactions(nil, f.uid, []int64{tx.TransactionId}, book.Id))
	tx.BookId = book.Id
	tx.TransferFeeAmount = "-2.34"
	tx.TransferFeeCategoryId = 702
	tx.Amount = 20000
	tx.RelatedAccountAmount = 20000
	require.NoError(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, int64(1980234), f.balance())
	fee = models.Transaction{}
	has, err = f.engine.Where("uid=? AND transfer_fee_parent_id=? AND deleted=?", f.uid, tx.TransactionId, false).Get(&fee)
	require.NoError(t, err)
	require.True(t, has)
	require.Equal(t, book.Id, fee.BookId)
	require.Equal(t, models.TRANSACTION_DB_TYPE_INCOME, fee.Type)
	tx.TransferFeeAmount = "99"
	tx.TransferFeeCategoryId = 99999
	require.Error(t, Transactions.ModifyTransaction(nil, tx, false, 0, nil, nil, nil, nil))
	require.Equal(t, int64(1980234), f.balance(), "invalid fee must roll back the parent and old fee")
	require.NoError(t, Transactions.DeleteTransaction(nil, f.uid, tx.TransactionId))
	require.Equal(t, int64(2000000), f.balance())
	count, err := f.engine.Where("uid=? AND transfer_fee_parent_id=? AND deleted=?", f.uid, tx.TransactionId, false).Count(&models.Transaction{})
	require.NoError(t, err)
	require.Zero(t, count)
}
func TestTransferFeeInvalidCreateAndCascadeRestoreOtherAccount(t *testing.T) {
	f, tx := feeFixture(t)
	for _, value := range []string{"1e10", "NaN", "0.001", "99999999999999", "--1"} {
		tx.TransferFeeAmount = value
		require.Error(t, Transactions.CreateTransaction(nil, tx, nil, nil))
		require.Equal(t, int64(2000000), f.balance())
	}
	tx.TransferFeeAmount = "1.23"
	require.NoError(t, Transactions.CreateTransaction(nil, tx, nil, nil))
	input := AccountDeletionInput{ID: fmt.Sprint(1003), Kind: "cash", DeleteRelated: true}
	preview, err := f.s.DeleteAssetAccount(nil, f.uid, input, true)
	require.NoError(t, err)
	input.Token = preview.Token
	_, err = f.s.DeleteAssetAccount(nil, f.uid, input, false)
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance(), "deleting transfer destination must also reverse source fee")
}
