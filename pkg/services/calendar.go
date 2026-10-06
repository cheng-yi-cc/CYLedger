package services

import (
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

type CalendarService struct{ ServiceUsingDB }

var Calendar = &CalendarService{ServiceUsingDB{container: datastore.Container}}
var ErrCalendarInvalid = errs.NewNormalError(23, 1, 400, "到期事项无效，请检查日期、金额和账户类型")
var ErrCalendarNotFound = errs.NewNormalError(23, 2, 404, "到期事项不存在")
var calendarAmountPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)

func (s *CalendarService) Pending(c core.Context, uid int64) ([]models.CalendarEvent, error) {
	items := make([]models.CalendarEvent, 0)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var all []models.CalendarEvent
		if err := sess.Where("uid=? AND completed=?", uid, false).OrderBy("date asc,id asc").Find(&all); err != nil {
			return err
		}
		for _, item := range all {
			a := new(models.Account)
			has, err := sess.Where("uid=? AND account_id=? AND deleted=? AND system_role=?", uid, item.AccountId, false, "").Get(a)
			if err != nil {
				return err
			}
			if has {
				item.AccountName = a.Name
				items = append(items, item)
			}
		}
		return nil
	})
	return items, err
}

func validCalendarDate(value string) bool {
	if len(value) != 10 {
		return false
	}
	d, err := time.Parse("2006-01-02", value)
	return err == nil && d.Year() >= 1900 && d.Year() <= 9999
}

func (s *CalendarService) List(c core.Context, uid int64, month string) ([]models.CalendarEvent, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if len(month) != 7 || !validCalendarDate(month+"-01") {
		return nil, ErrCalendarInvalid
	}

	items := make([]models.CalendarEvent, 0)
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		return sess.Where("uid=? AND date>=? AND date<=?", uid, month+"-01", month+"-31").OrderBy("date asc,id asc").Find(&items)
	})
	return items, err
}

func (s *CalendarService) Save(c core.Context, uid int64, req models.CalendarSaveRequest) (*models.CalendarEvent, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	if len(req.Id) > 64 || len(req.AccountId) > 32 || len(req.Amount) > 32 || !calendarAmountPattern.MatchString(req.Amount) || !validCalendarDate(req.Date) || utf8.RuneCountInString(req.Note) > 200 || (req.Kind != "repayment" && req.Kind != "deposit") {
		return nil, ErrCalendarInvalid
	}
	amount, err := decimal.NewFromString(req.Amount)
	if err != nil || !amount.IsPositive() || amount.GreaterThan(decimal.New(999999999999999, -2)) {
		return nil, ErrCalendarInvalid
	}
	accountId, err := strconv.ParseInt(req.AccountId, 10, 64)
	if err != nil || accountId <= 0 {
		return nil, ErrCalendarInvalid
	}
	item := &models.CalendarEvent{}
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if req.Id != "" {
			has, err := sess.ID(req.Id).Where("uid=?", uid).Get(item)
			if err != nil {
				return err
			}
			if !has {
				return ErrCalendarNotFound
			}
			if item.TransactionId > 0 {
				return investmentError("请在原借款账单中修改约定还款日期")
			}
		}
		bookId, err := Books.ResolveInSession(sess, uid, req.BookId, false)
		if err != nil {
			return err
		}
		account := &models.Account{}
		has, err := sess.ID(accountId).Where("uid=? AND deleted=? AND system_role=?", uid, false, "").Get(account)
		if err != nil {
			return err
		}
		if !has || account.Type != models.ACCOUNT_TYPE_SINGLE_ACCOUNT {
			return ErrCalendarInvalid
		}
		if req.Kind == "repayment" && account.Category != models.ACCOUNT_CATEGORY_CREDIT_CARD && account.Category != models.ACCOUNT_CATEGORY_DEBT {
			return ErrCalendarInvalid
		}
		if req.Kind == "deposit" && account.Category != models.ACCOUNT_CATEGORY_CERTIFICATE_OF_DEPOSIT {
			return ErrCalendarInvalid
		}
		item.Uid = uid
		item.BookId = bookId
		item.AccountId = req.AccountId
		item.AccountName = account.Name
		item.Currency = account.Currency
		item.Kind = req.Kind
		item.Date = req.Date
		item.Amount = amount.StringFixed(2)
		item.Note = strings.TrimSpace(req.Note)
		if req.Id == "" {
			item.Id = "due-" + investmentID()
			_, err = sess.Insert(item)
		} else {
			_, err = sess.ID(item.Id).Where("uid=?", uid).AllCols().Update(item)
		}
		return err
	})
	return item, err
}

func (s *CalendarService) SetCompleted(c core.Context, uid int64, id string, completed bool) (bool, error) {
	return s.change(c, uid, id, func(sess *xorm.Session) (int64, error) {
		return sess.ID(id).Where("uid=?", uid).Cols("completed").Update(&models.CalendarEvent{Completed: completed})
	})
}
func (s *CalendarService) Delete(c core.Context, uid int64, id string) (bool, error) {
	return s.change(c, uid, id, func(sess *xorm.Session) (int64, error) {
		var item models.CalendarEvent
		if _, err := sess.ID(id).Where("uid=?", uid).Get(&item); err != nil {
			return 0, err
		}
		if item.TransactionId > 0 {
			return 0, investmentError("请在原借款账单中清除约定还款日期")
		}
		return sess.ID(id).Where("uid=?", uid).Delete(&models.CalendarEvent{})
	})
}

func (s *CalendarService) DeleteAll(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	return s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		_, err := sess.Where("uid=?", uid).Delete(&models.CalendarEvent{})
		return err
	})
}
func (s *CalendarService) change(c core.Context, uid int64, id string, action func(*xorm.Session) (int64, error)) (bool, error) {
	if uid <= 0 {
		return false, errs.ErrUserIdInvalid
	}
	if id == "" || len(id) > 64 {
		return false, ErrCalendarInvalid
	}
	err := s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		has, err := sess.ID(id).Where("uid=?", uid).Exist(&models.CalendarEvent{})
		if err != nil {
			return err
		}
		if !has {
			return ErrCalendarNotFound
		}
		_, err = action(sess)
		return err
	})
	return err == nil, err
}
