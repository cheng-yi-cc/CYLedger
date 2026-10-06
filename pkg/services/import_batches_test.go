package services

import (
	"encoding/json"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/utils"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestImportBatchDedupAndAtomicUndo(t *testing.T) {
	f, transfer := feeFixture(t)
	batch := &ImportBatch{Id: "fictional-import", Name: "fixture.csv", Fingerprint: "fixture-hash"}
	require.NoError(t, Transactions.BatchCreateTransactions(nil, f.uid, []*models.Transaction{transfer}, nil, nil, batch))
	require.Equal(t, int64(1989877), f.balance())
	count, exists, err := LedgerWorkspace.ExistingImportBatch(nil, f.uid, batch.Id, batch.Fingerprint)
	require.NoError(t, err)
	require.True(t, exists)
	require.Equal(t, 1, count)
	duplicate := *transfer
	duplicate.TransactionId = 0
	require.NoError(t, Transactions.BatchCreateTransactions(nil, f.uid, []*models.Transaction{&duplicate}, nil, nil, batch))
	require.Equal(t, int64(1989877), f.balance())
	_, _, err = LedgerWorkspace.ExistingImportBatch(nil, f.uid, batch.Id, "different")
	require.Error(t, err)
	require.Error(t, LedgerWorkspace.UndoImportBatch(nil, f.uid+1, batch.Id))
	require.NoError(t, LedgerWorkspace.UndoImportBatch(nil, f.uid, batch.Id))
	require.Equal(t, int64(2000000), f.balance())
	require.NoError(t, LedgerWorkspace.UndoImportBatch(nil, f.uid, batch.Id))
	require.Equal(t, int64(2000000), f.balance())
	_, _, err = LedgerWorkspace.ExistingImportBatch(nil, f.uid, batch.Id, batch.Fingerprint)
	require.Error(t, err)
}
func TestImportFailureDoesNotPersistBatch(t *testing.T) {
	f, transfer := feeFixture(t)
	bad := &models.Transaction{Uid: f.uid, AccountId: f.bankID, Type: models.TRANSACTION_DB_TYPE_EXPENSE, Amount: 123, CategoryId: 99999, TransactionTime: utils.GetMinTransactionTimeFromUnixTime(f.at + 2)}
	batch := &ImportBatch{Id: "failure", Fingerprint: "hash"}
	require.Error(t, Transactions.BatchCreateTransactions(nil, f.uid, []*models.Transaction{transfer, bad}, nil, nil, batch))
	require.Equal(t, int64(2000000), f.balance())
	_, exists, err := LedgerWorkspace.ExistingImportBatch(nil, f.uid, batch.Id, batch.Fingerprint)
	require.NoError(t, err)
	require.False(t, exists)
}
func TestLedgerWishOwnershipRevisionAndNoBalanceMutation(t *testing.T) {
	f, _ := feeFixture(t)
	wish := models.LedgerWish{Name: "旅行", Icon: "star", Target: "10000.00", Initial: "50", Amount: "10", Ratio: "100", StartDate: "2026-01-01", Mode: "manual", Cycle: "month", Logs: []models.LedgerWishLog{{Id: "log", Date: "2026-01-02", Amount: "25", Note: "计划储蓄"}}}
	raw, err := json.Marshal(wish)
	require.NoError(t, err)
	item, err := LedgerWorkspace.Save(nil, f.uid, models.LedgerWorkspaceItem{Kind: "wish", Data: raw})
	require.NoError(t, err)
	require.Equal(t, int64(2000000), f.balance())
	_, err = LedgerWorkspace.Save(nil, f.uid+1, *item)
	require.Error(t, err)
	original := *item
	item, err = LedgerWorkspace.Save(nil, f.uid, *item)
	require.NoError(t, err)
	_, err = LedgerWorkspace.Save(nil, f.uid, original)
	require.Error(t, err)
	require.Error(t, LedgerWorkspace.Delete(nil, f.uid+1, *item))
	require.NoError(t, LedgerWorkspace.Delete(nil, f.uid, *item))
	require.Equal(t, int64(2000000), f.balance())
	wish.Target = "1e1000000"
	raw, _ = json.Marshal(wish)
	_, err = LedgerWorkspace.Save(nil, f.uid, models.LedgerWorkspaceItem{Kind: "wish", Data: raw})
	require.Error(t, err)
}
