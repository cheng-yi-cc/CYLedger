package services

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"xorm.io/xorm"
)

type BookService struct{ ServiceUsingDB }

var Books = &BookService{ServiceUsingDB{container: datastore.Container}}
var ErrBookNotFound = errs.NewNormalError(22, 1, 400, "账本不存在或不属于当前用户")
var ErrBookArchived = errs.NewNormalError(22, 2, 400, "账本已归档，请先恢复账本")
var ErrDefaultBookArchived = errs.NewNormalError(22, 3, 400, "请先将其他账本设为默认，再归档此账本")
var ErrBookInvalid = errs.NewNormalError(22, 4, 400, "账本名称或筛选条件无效")

// DefaultBookID is the stable migration identity, not the user's current choice.
func DefaultBookID(uid int64) string { return fmt.Sprintf("default-%d", uid) }

func (s *BookService) ensureDefaultInSession(sess *xorm.Session, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	book := &models.Book{}
	has, err := sess.ID(DefaultBookID(uid)).Where("uid=?", uid).Get(book)
	if err != nil || has {
		return err
	}
	hasDefault, err := sess.Where("uid=? AND is_default=?", uid, true).Exist(&models.Book{})
	if err != nil {
		return err
	}
	_, err = sess.Insert(&models.Book{Id: DefaultBookID(uid), Uid: uid, Name: "日常账本", Icon: "📒", IsDefault: !hasDefault, ShowTransfers: true, ShowInvestments: true})
	return err
}

// ResolveInSession enforces ownership and write eligibility inside the caller's transaction.
func (s *BookService) ResolveInSession(sess *xorm.Session, uid int64, id string, allowArchived bool) (string, error) {
	if uid <= 0 {
		return "", errs.ErrUserIdInvalid
	}
	if len(id) > 64 {
		return "", ErrBookInvalid
	}
	if id == "" {
		if err := s.ensureDefaultInSession(sess, uid); err != nil {
			return "", err
		}
	}
	book := &models.Book{}
	query := sess.Where("uid=?", uid)
	if id == "" {
		query = query.And("is_default=?", true)
	} else {
		query = query.And("id=?", id)
	}
	has, err := query.Get(book)
	if err != nil {
		return "", err
	}
	if !has {
		return "", ErrBookNotFound
	}
	if book.Archived && !allowArchived {
		return "", ErrBookArchived
	}
	return book.Id, nil
}

func (s *BookService) List(c core.Context, uid int64) ([]models.Book, error) {
	books := make([]models.Book, 0)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := s.ensureDefaultInSession(sess, uid); err != nil {
			return err
		}
		return sess.Where("uid=?", uid).OrderBy("display_order asc,id asc").Find(&books)
	})
	return books, err
}

func (s *BookService) Create(c core.Context, uid int64, req models.BookCreateRequest) (*models.Book, error) {
	name := strings.TrimSpace(req.Name)
	if name == "" || utf8.RuneCountInString(name) > 64 || utf8.RuneCountInString(req.Icon) > 32 {
		return nil, ErrBookInvalid
	}
	if req.Icon == "" {
		req.Icon = "📒"
	}
	book := &models.Book{Id: "book-" + investmentID(), Uid: uid, Name: name, Icon: req.Icon, ShowTransfers: true, ShowInvestments: true}
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := s.ensureDefaultInSession(sess, uid); err != nil {
			return err
		}
		count, err := sess.Where("uid=?", uid).Count(&models.Book{})
		if err != nil {
			return err
		}
		book.DisplayOrder = int32(count)
		_, err = sess.Insert(book)
		return err
	})
	return book, err
}

func (s *BookService) Modify(c core.Context, uid int64, req models.BookModifyRequest) (*models.Book, error) {
	book := &models.Book{}
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		has, err := sess.ID(req.Id).Where("uid=?", uid).Get(book)
		if err != nil {
			return err
		}
		if !has {
			return ErrBookNotFound
		}
		if req.Name != nil {
			book.Name = strings.TrimSpace(*req.Name)
			if book.Name == "" || utf8.RuneCountInString(book.Name) > 64 {
				return ErrBookInvalid
			}
		}
		if req.Icon != nil {
			book.Icon = *req.Icon
			if utf8.RuneCountInString(book.Icon) > 32 {
				return ErrBookInvalid
			}
		}
		if req.DisplayOrder != nil {
			book.DisplayOrder = *req.DisplayOrder
		}
		if req.Archived != nil {
			book.Archived = *req.Archived
		}
		if req.ShowTransfers != nil {
			book.ShowTransfers = *req.ShowTransfers
		}
		if req.ShowInvestments != nil {
			book.ShowInvestments = *req.ShowInvestments
		}
		if req.IsDefault != nil && !*req.IsDefault && book.IsDefault {
			return ErrDefaultBookArchived
		}
		if req.IsDefault != nil && *req.IsDefault {
			if book.Archived {
				return ErrBookArchived
			}
			if _, err = sess.Where("uid=? AND is_default=?", uid, true).Cols("is_default").Update(&models.Book{IsDefault: false}); err != nil {
				return err
			}
			book.IsDefault = true
		}
		if book.Archived && book.IsDefault {
			return ErrDefaultBookArchived
		}

		_, err = sess.ID(book.Id).Where("uid=?", uid).AllCols().Update(book)
		return err
	})
	return book, err
}

type bookFilterContext struct {
	core.Context
	ids []string
}
type bookFilterKey struct{}

func (c *bookFilterContext) Value(key any) any {
	if _, ok := key.(bookFilterKey); ok {
		return c.ids
	}
	if c.Context == nil {
		return nil
	}
	return c.Context.Value(key)
}

// FilterContext validates every requested identity; unknown IDs are never silently ignored.
func (s *BookService) FilterContext(c core.Context, uid int64, raw string) (core.Context, error) {
	if raw == "" {
		return c, nil
	}
	if len(raw) > 6500 {
		return nil, ErrBookInvalid
	}
	parts := strings.Split(raw, ",")
	if len(parts) > 100 {
		return nil, ErrBookInvalid
	}
	ids := make([]string, 0, len(parts))
	seen := map[string]bool{}
	sess := s.UserDataDB(uid).NewSession(c)
	defer sess.Close()
	for _, part := range parts {
		id := strings.TrimSpace(part)
		if id == "" || len(id) > 64 {
			return nil, ErrBookInvalid
		}
		if _, err := s.ResolveInSession(sess, uid, id, true); err != nil {
			return nil, err
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	if c == nil {
		c = core.NewNullContext()
	}
	return &bookFilterContext{Context: c, ids: ids}, nil
}

func appendBookFilter(c core.Context, condition string, params []any) (string, []any) {
	if c == nil {
		return condition, params
	}
	ids, _ := c.Value(bookFilterKey{}).([]string)
	if len(ids) == 0 {
		return condition, params
	}
	placeholders := make([]string, len(ids))
	for i, id := range ids {
		placeholders[i] = "?"
		params = append(params, id)
	}
	return condition + " AND book_id IN (" + strings.Join(placeholders, ",") + ")", params
}

// MoveTransactions changes classification only, including both sides of a transfer.
func (s *BookService) MoveTransactions(c core.Context, uid int64, ids []int64, bookID string) error {
	if len(ids) == 0 || len(ids) > 1000 {
		return ErrBookInvalid
	}
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		target, err := s.ResolveInSession(sess, uid, bookID, false)
		if err != nil {
			return err
		}
		allIDs := map[int64]bool{}
		for _, id := range ids {
			var tx models.Transaction
			has, err := sess.ID(id).Where("uid=? AND deleted=?", uid, false).Get(&tx)
			if err != nil {
				return err
			}
			if !has {
				return errs.ErrTransactionNotFound
			}
			allIDs[id] = true
			if tx.RelatedId != 0 {
				allIDs[tx.RelatedId] = true
			}
		}
		paired := make([]int64, 0, len(allIDs))
		for id := range allIDs {
			paired = append(paired, id)
		}
		if err := guardInvestmentTransactions(sess, uid, paired); err != nil {
			return err
		}
		_, err = sess.Where("uid=? AND deleted=?", uid, false).In("transaction_id", paired).Cols("book_id", "updated_unix_time").Update(&models.Transaction{BookId: target, UpdatedUnixTime: time.Now().Unix()})
		return err
	})
}

// MigrateUser is idempotent and preserves all amounts, IDs, and settlement links.
func (s *BookService) MigrateUser(c core.Context, uid int64) error {
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if err := s.ensureDefaultInSession(sess, uid); err != nil {
			return err
		}
		id := DefaultBookID(uid)
		if _, err := sess.Where("uid=? AND (book_id=? OR book_id IS NULL)", uid, "").Cols("book_id").Update(&models.Transaction{BookId: id}); err != nil {
			return err
		}
		if _, err := sess.Where("uid=? AND (book_id=? OR book_id IS NULL)", uid, "").Cols("book_id").Update(&models.TransactionTemplate{BookId: id}); err != nil {
			return err
		}
		var events []models.InvestmentEventRecord
		if err := sess.Where("uid=?", uid).Find(&events); err != nil {
			return err
		}
		for _, event := range events {
			payload, changed, err := backfillBookPayload(event.Payload, id)
			if err != nil {
				return err
			}
			if changed {
				if _, err = sess.ID(event.Id).Where("uid=?", uid).Cols("payload").Update(&models.InvestmentEventRecord{Payload: payload}); err != nil {
					return err
				}
			}
		}
		var revisions []models.InvestmentEventRevision
		if err := sess.Where("uid=?", uid).Find(&revisions); err != nil {
			return err
		}
		for _, revision := range revisions {
			payload, changed, err := backfillBookPayload(revision.Payload, id)
			if err != nil {
				return err
			}
			if changed {
				if _, err = sess.ID(revision.Id).Where("uid=?", uid).Cols("payload").Update(&models.InvestmentEventRevision{Payload: payload}); err != nil {
					return err
				}
			}
		}
		return nil
	})
}

func backfillBookPayload(payload, id string) (string, bool, error) {
	var item map[string]json.RawMessage
	if err := json.Unmarshal([]byte(payload), &item); err != nil {
		return "", false, err
	}
	if item == nil {
		return "", false, ErrBookInvalid
	}
	var previous string
	if value, ok := item["bookId"]; ok {
		if err := json.Unmarshal(value, &previous); err != nil {
			return "", false, err
		}
	}
	if previous != "" {
		return payload, false, nil
	}
	item["bookId"], _ = json.Marshal(id)
	data, err := json.Marshal(item)
	return string(data), true, err
}

func (s *BookService) MigrateAll(c core.Context) error {
	for i := 0; i < s.container.UserStore.Count(); i++ {
		var users []models.User
		sess := s.container.UserStore.Get(i).NewSession(c)
		err := sess.Cols("uid").Find(&users)
		sess.Close()
		if err != nil {
			return err
		}
		for _, user := range users {
			if err := s.MigrateUser(c, user.Uid); err != nil {
				return err
			}
		}
	}
	return nil
}

// WithoutBookFilter keeps shared-account balance calculations independent of reporting scope.
func WithoutBookFilter(c core.Context) core.Context {
	if scoped, ok := c.(*bookFilterContext); ok {
		return scoped.Context
	}
	return c
}
