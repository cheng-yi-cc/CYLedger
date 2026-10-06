package services

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
	"xorm.io/xorm"
)

type LedgerWorkspaceService struct{ ServiceUsingDB }

var LedgerWorkspace = &LedgerWorkspaceService{ServiceUsingDB{container: datastore.Container}}
var ErrLedgerItemInvalid = errs.NewNormalError(25, 1, 400, "愿望或关键词设置无效，请检查金额、日期和所选账户")
var ErrLedgerItemConflict = errs.NewNormalError(25, 2, 409, "内容已变更或不存在，请刷新后重试")
var ledgerIconPattern = regexp.MustCompile(`^[a-z0-9_]{1,64}$`)

func (s *LedgerWorkspaceService) Items(c core.Context, uid int64, kind string) ([]models.LedgerWorkspaceItem, error) {
	if uid <= 0 || (kind != "wish" && kind != "keyword") {
		return nil, ErrLedgerItemInvalid
	}
	rows := make([]models.LocalLedgerItem, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=? AND kind=?", uid, kind).OrderBy("updated_unix_time DESC, id").Find(&rows)
	items := make([]models.LedgerWorkspaceItem, 0, len(rows))
	for _, row := range rows {
		items = append(items, models.LedgerWorkspaceItem{Id: row.Id, Kind: row.Kind, Revision: row.Revision, Data: json.RawMessage(row.Payload)})
	}
	return items, err
}
func ledgerReferenceIds(sess *xorm.Session, uid int64, kind string, ids []string) error {
	if len(ids) > 100 {
		return ErrLedgerItemInvalid
	}
	for _, id := range ids {
		if len(id) == 0 || len(id) > 64 {
			return ErrLedgerItemInvalid
		}
		if kind == "book" {
			if _, err := Books.ResolveInSession(sess, uid, id, true); err != nil {
				return err
			}
			continue
		}
		num, err := strconv.ParseInt(id, 10, 64)
		if err != nil || num <= 0 {
			return ErrLedgerItemInvalid
		}
		var exists bool
		switch kind {
		case "account":
			exists, err = sess.Where("uid=? AND account_id=? AND deleted=? AND system_role=''", uid, num, false).Exist(&models.Account{})
		case "category":
			exists, err = sess.Where("uid=? AND category_id=? AND deleted=?", uid, num, false).Exist(&models.TransactionCategory{})
		case "tag":
			exists, err = sess.Where("uid=? AND tag_id=? AND deleted=?", uid, num, false).Exist(&models.TransactionTag{})
		default:
			return ErrLedgerItemInvalid
		}
		if err != nil {
			return err
		}
		if !exists {
			return ErrLedgerItemInvalid
		}
	}
	return nil
}
func validateLedgerWish(sess *xorm.Session, uid int64, data json.RawMessage) ([]byte, error) {
	var wish models.LedgerWish
	if json.Unmarshal(data, &wish) != nil || strings.TrimSpace(wish.Name) == "" || utf8.RuneCountInString(wish.Name) > 64 || utf8.RuneCountInString(wish.Note) > 1000 || !ledgerIconPattern.MatchString(wish.Icon) || !validCalendarDate(wish.StartDate) || (wish.EndDate != "" && (!validCalendarDate(wish.EndDate) || wish.EndDate < wish.StartDate)) || len(wish.Logs) > 300 {
		return nil, ErrLedgerItemInvalid
	}
	for _, field := range []*string{&wish.Target, &wish.Initial, &wish.Amount, &wish.Ratio} {
		v, err := normalizeStatisticsMoney(*field)
		if err != nil {
			return nil, ErrLedgerItemInvalid
		}
		*field = v
	}
	target, _ := decimal.NewFromString(wish.Target)
	ratio, _ := decimal.NewFromString(wish.Ratio)
	if !target.IsPositive() || ratio.GreaterThan(decimal.NewFromInt(100)) {
		return nil, ErrLedgerItemInvalid
	}
	switch wish.Mode {
	case "manual", "schedule", "income", "balance", "asset":
	default:
		return nil, ErrLedgerItemInvalid
	}
	switch wish.Cycle {
	case "day", "week", "month", "year":
	default:
		return nil, ErrLedgerItemInvalid
	}
	for kind, ids := range map[string][]string{"book": wish.BookIds, "account": wish.AccountIds, "category": append(append([]string{}, wish.IncomeCategoryIds...), wish.ExpenseCategoryIds...)} {
		if err := ledgerReferenceIds(sess, uid, kind, ids); err != nil {
			return nil, err
		}
	}
	seen := map[string]bool{}
	saved, _ := decimal.NewFromString(wish.Initial)
	for i := range wish.Logs {
		log := &wish.Logs[i]
		if log.Id == "" || len(log.Id) > 64 || seen[log.Id] || !validCalendarDate(log.Date) || utf8.RuneCountInString(log.Note) > 255 {
			return nil, ErrLedgerItemInvalid
		}
		seen[log.Id] = true
		negative := strings.HasPrefix(log.Amount, "-")
		value, err := normalizeStatisticsMoney(strings.TrimPrefix(log.Amount, "-"))
		if err != nil {
			return nil, ErrLedgerItemInvalid
		}
		amount, _ := decimal.NewFromString(value)
		if negative {
			amount = amount.Neg()
		}
		if amount.IsZero() {
			return nil, ErrLedgerItemInvalid
		}
		log.Amount = amount.StringFixed(2)
		saved = saved.Add(amount)
	}
	if wish.Mode == "manual" && saved.IsNegative() {
		return nil, ErrLedgerItemInvalid
	}
	wish.Name = strings.TrimSpace(wish.Name)
	return json.Marshal(wish)
}
func validateLedgerKeyword(sess *xorm.Session, uid int64, data json.RawMessage) ([]byte, error) {
	var rule models.LedgerKeywordRule
	if json.Unmarshal(data, &rule) != nil || len(rule.Keywords) == 0 || len(rule.Keywords) > 100 {
		return nil, ErrLedgerItemInvalid
	}
	for i, word := range rule.Keywords {
		word = strings.TrimSpace(word)
		if word == "" || utf8.RuneCountInString(word) > 64 {
			return nil, ErrLedgerItemInvalid
		}
		rule.Keywords[i] = word
	}
	if err := ledgerReferenceIds(sess, uid, "category", []string{rule.CategoryId}); err != nil {
		return nil, err
	}
	if rule.AccountId != "" {
		if err := ledgerReferenceIds(sess, uid, "account", []string{rule.AccountId}); err != nil {
			return nil, err
		}
	}
	if len(rule.TagIds) > 10 {
		return nil, ErrLedgerItemInvalid
	}
	if err := ledgerReferenceIds(sess, uid, "tag", rule.TagIds); err != nil {
		return nil, err
	}
	return json.Marshal(rule)
}
func (s *LedgerWorkspaceService) Save(c core.Context, uid int64, input models.LedgerWorkspaceItem) (*models.LedgerWorkspaceItem, error) {
	if uid <= 0 || len(input.Id) > 64 || input.Revision < 0 || input.Revision > 1000000000 || len(input.Data) > 60*1024 || (input.Kind != "wish" && input.Kind != "keyword") {
		return nil, ErrLedgerItemInvalid
	}
	defer Investments.lock(uid)()
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var payload []byte
		var err error
		if input.Kind == "wish" {
			payload, err = validateLedgerWish(sess, uid, input.Data)
		} else {
			payload, err = validateLedgerKeyword(sess, uid, input.Data)
		}
		if err != nil {
			return err
		}
		row := models.LocalLedgerItem{Uid: uid, Kind: input.Kind, UpdatedUnixTime: time.Now().Unix(), Payload: string(payload)}
		if input.Id == "" {
			if input.Revision != 0 {
				return ErrLedgerItemConflict
			}
			count, err := sess.Where("uid=? AND kind=?", uid, input.Kind).Count(&models.LocalLedgerItem{})
			if err != nil {
				return err
			}
			if count >= 500 {
				return ErrLedgerItemInvalid
			}
			id := make([]byte, 16)
			if _, err = rand.Read(id); err != nil {
				return err
			}
			row.Id = hex.EncodeToString(id)
			row.Revision = 1
			_, err = sess.Insert(&row)
			if err != nil {
				return err
			}
		} else {
			var current models.LocalLedgerItem
			has, err := sess.ID(input.Id).Where("uid=? AND kind=?", uid, input.Kind).Get(&current)
			if err != nil {
				return err
			}
			if !has || current.Revision != input.Revision {
				return ErrLedgerItemConflict
			}
			row.Id = input.Id
			row.Revision = input.Revision + 1
			n, err := sess.ID(row.Id).Where("uid=? AND revision=?", uid, input.Revision).Cols("payload", "revision", "updated_unix_time").Update(&row)
			if err != nil {
				return err
			}
			if n != 1 {
				return ErrLedgerItemConflict
			}
		}
		input.Id, input.Revision, input.Data = row.Id, row.Revision, json.RawMessage(payload)
		return nil
	})
	return &input, err
}
func (s *LedgerWorkspaceService) Delete(c core.Context, uid int64, input models.LedgerWorkspaceItem) error {
	if uid <= 0 || input.Id == "" || len(input.Id) > 64 || input.Revision < 1 || (input.Kind != "wish" && input.Kind != "keyword") {
		return ErrLedgerItemInvalid
	}
	n, err := s.UserDataDB(uid).NewSession(c).ID(input.Id).Where("uid=? AND kind=? AND revision=?", uid, input.Kind, input.Revision).Delete(&models.LocalLedgerItem{})
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrLedgerItemConflict
	}
	return nil
}
