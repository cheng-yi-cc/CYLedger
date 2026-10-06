package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"time"
	"xorm.io/xorm"
)

type ImportBatch struct {
	Id             string  `json:"id"`
	Name           string  `json:"name"`
	Count          int     `json:"count,string"`
	CreatedAt      int64   `json:"createdAt"`
	Deleted        bool    `json:"deleted"`
	TransactionIds []int64 `json:"-"`
	Fingerprint    string  `json:"-"`
}
type storedImportBatch struct {
	ImportBatch
	Ids  []int64 `json:"transactionIds"`
	Hash string  `json:"fingerprint"`
}

func importBatchKey(uid int64, id string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("import:%d:%s", uid, id)))
	return hex.EncodeToString(sum[:])
}
func (s *LedgerWorkspaceService) ImportBatches(c core.Context, uid int64) ([]ImportBatch, error) {
	rows := []models.LocalLedgerItem{}
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	if err := sess.Where("uid=? AND kind='import'", uid).Desc("updated_unix_time").Find(&rows); err != nil {
		return nil, err
	}
	result := []ImportBatch{}
	for _, row := range rows {
		var data storedImportBatch
		if err := json.Unmarshal([]byte(row.Payload), &data); err != nil {
			return nil, err
		}
		result = append(result, data.ImportBatch)
	}
	return result, nil
}

// ExistingImportBatch persists idempotency across restarts, including after undo.
func (s *LedgerWorkspaceService) ExistingImportBatch(c core.Context, uid int64, id, hash string) (int, bool, error) {
	if id == "" {
		return 0, false, nil
	}
	if len(id) > 64 {
		return 0, false, ErrLedgerItemInvalid
	}
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	return existingImportBatch(sess, uid, id, hash)
}
func existingImportBatch(sess *xorm.Session, uid int64, id, hash string) (int, bool, error) {
	var row models.LocalLedgerItem
	has, err := sess.ID(importBatchKey(uid, id)).Where("uid=? AND kind='import'", uid).Get(&row)
	if err != nil || !has {
		return 0, false, err
	}
	var data storedImportBatch
	if json.Unmarshal([]byte(row.Payload), &data) != nil || data.Hash != hash || data.Deleted {
		return 0, true, ErrLedgerItemConflict
	}
	return data.Count, true, nil
}
func saveImportBatch(sess *xorm.Session, uid int64, batch *ImportBatch, transactions []*models.Transaction) error {
	ids := make([]int64, 0, len(transactions))
	for _, tx := range transactions {
		ids = append(ids, tx.TransactionId)
	}
	batch.Count = len(ids)
	batch.CreatedAt = time.Now().Unix()
	data, err := json.Marshal(storedImportBatch{ImportBatch: *batch, Ids: ids, Hash: batch.Fingerprint})
	if err != nil {
		return err
	}
	_, err = sess.Insert(&models.LocalLedgerItem{Id: importBatchKey(uid, batch.Id), Uid: uid, Kind: "import", Revision: 1, Payload: string(data), UpdatedUnixTime: batch.CreatedAt})
	return err
}
func (s *LedgerWorkspaceService) UndoImportBatch(c core.Context, uid int64, id string) error {
	if id == "" || len(id) > 64 {
		return ErrLedgerItemInvalid
	}
	defer Investments.lock(uid)()
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var row models.LocalLedgerItem
		has, err := sess.ID(importBatchKey(uid, id)).Where("uid=? AND kind='import'", uid).Get(&row)
		if err != nil {
			return err
		}
		if !has {
			return ErrLedgerItemConflict
		}
		var data storedImportBatch
		if json.Unmarshal([]byte(row.Payload), &data) != nil {
			return ErrLedgerItemInvalid
		}
		if data.Deleted {
			return nil
		}
		for _, id := range data.Ids {
			exists, err := sess.ID(id).Where("uid=? AND deleted=?", uid, false).Exist(&models.Transaction{})
			if err != nil {
				return err
			}
			if exists {
				if err := Transactions.deleteTransactionInSession(c, sess, uid, id); err != nil {
					return err
				}
			}
		}
		data.Deleted = true
		raw, err := json.Marshal(data)
		if err != nil {
			return err
		}
		row.Payload = string(raw)
		row.Revision++
		_, err = sess.ID(row.Id).Cols("payload", "revision").Update(&row)
		return err
	})
}
