package services

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/shopspring/decimal"
	"xorm.io/xorm"
)

type StatisticsWorkspaceService struct{ ServiceUsingDB }

var StatisticsWorkspace = &StatisticsWorkspaceService{ServiceUsingDB{container: datastore.Container}}
var ErrStatisticsInvalid = errs.NewNormalError(24, 1, 400, "统计设置、金额或日期无效")
var ErrStatisticsConflict = errs.NewNormalError(24, 2, 409, "内容已更新，请刷新后重试")
var ErrTagHierarchy = errs.NewNormalError(24, 3, 400, "标签最多两级；父标签必须属于本人，且不能将有子标签的标签移到第二级")
var ErrTagHasChildren = errs.NewNormalError(24, 4, 400, "请先移动或删除子标签")

var statisticsMoneyPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,12})(\.[0-9]{1,2})?$`)
var statisticsPeriodPattern = regexp.MustCompile(`^[0-9]{4}(-[0-9]{2})?$`)

func statisticsKey(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:])
}

func normalizeStatisticsMoney(value string) (string, error) {
	if len(value) > 16 || !statisticsMoneyPattern.MatchString(value) {
		return "", ErrStatisticsInvalid
	}
	d, err := decimal.NewFromString(value)
	if err != nil {
		return "", ErrStatisticsInvalid
	}
	return d.StringFixed(2), nil
}

func validateTransactionDiscount(transaction *models.Transaction) error {
	if transaction.DiscountAmount == "" {
		transaction.DiscountAmount = "0"
	}
	value, err := normalizeStatisticsMoney(transaction.DiscountAmount)
	if err != nil {
		return err
	}
	if value != "0.00" && (transaction.Amount <= 0 || (transaction.Type != models.TRANSACTION_DB_TYPE_INCOME && transaction.Type != models.TRANSACTION_DB_TYPE_EXPENSE) || transaction.InvestmentEventId != "" || transaction.ReimbursementReceiptId != "") {
		return ErrStatisticsInvalid
	}
	transaction.DiscountAmount = value
	return nil
}

func validateTagHierarchy(sess *xorm.Session, tag *models.TransactionTag) error {
	if tag.ParentTagId < 0 || (tag.ParentTagId != 0 && tag.ParentTagId == tag.TagId) {
		return ErrTagHierarchy
	}
	if tag.ParentTagId == 0 {
		return nil
	}
	var parent models.TransactionTag
	has, err := sess.Where("uid=? AND tag_id=? AND deleted=?", tag.Uid, tag.ParentTagId, false).Get(&parent)
	if err != nil {
		return err
	}
	if !has || parent.ParentTagId != 0 {
		return ErrTagHierarchy
	}
	has, err = sess.Where("uid=? AND parent_tag_id=? AND deleted=?", tag.Uid, tag.TagId, false).Exist(&models.TransactionTag{})
	if err != nil {
		return err
	}
	if has {
		return ErrTagHierarchy
	}
	return nil
}

var statisticsModuleIds = map[string][]string{
	"daily":  {"cash", "assets", "budget", "tags"},
	"month":  {"cash", "wealth", "flow", "categories", "ranking", "report", "accounts", "tagShare", "tags", "note"},
	"year":   {"cash", "heatmap", "wealth", "flow", "categories", "ranking", "report", "accounts", "tagShare", "tags", "note"},
	"custom": {"cash", "wealth", "flow", "categories", "ranking", "report", "accounts", "tagShare", "tags"},
}

func defaultStatisticsPreferences() *models.StatisticsPreferences {
	p := &models.StatisticsPreferences{Revision: "0", Modules: map[string][]models.StatisticsModule{}, DailyBudgetMode: "remaining", BudgetProgress: "remaining"}
	for period, ids := range statisticsModuleIds {
		for _, id := range ids {
			p.Modules[period] = append(p.Modules[period], models.StatisticsModule{Id: id, Visible: true})
		}
	}
	return p
}

func (s *StatisticsWorkspaceService) Preferences(c core.Context, uid int64) (*models.StatisticsPreferences, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	out := defaultStatisticsPreferences()
	var row models.StatisticsPreference
	has, err := s.UserDataDB(uid).NewSession(c).ID(uid).Get(&row)
	if err != nil {
		return nil, err
	}
	if has {
		if err = json.Unmarshal([]byte(row.Payload), out); err != nil {
			return nil, err
		}
		out.Revision = strconv.FormatInt(row.Revision, 10)
	}
	return out, nil
}

func (s *StatisticsWorkspaceService) SavePreferences(c core.Context, uid int64, input models.StatisticsPreferences) (*models.StatisticsPreferences, error) {
	if uid <= 0 || len(input.Modules) != len(statisticsModuleIds) || (input.DailyBudgetMode != "remaining" && input.DailyBudgetMode != "fixed") || (input.BudgetProgress != "remaining" && input.BudgetProgress != "spent") {
		return nil, ErrStatisticsInvalid
	}
	for period, allowed := range statisticsModuleIds {
		items := input.Modules[period]
		if len(items) != len(allowed) {
			return nil, ErrStatisticsInvalid
		}
		seen := map[string]bool{}
		for _, item := range items {
			seen[item.Id] = true
		}
		for _, id := range allowed {
			if !seen[id] {
				return nil, ErrStatisticsInvalid
			}
		}
	}
	revision, err := strconv.ParseInt(input.Revision, 10, 64)
	if err != nil || revision < 0 || revision > 1000000000 {
		return nil, ErrStatisticsInvalid
	}
	defer Investments.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		var current models.StatisticsPreference
		has, err := sess.ID(uid).Get(&current)
		if err != nil {
			return err
		}
		if current.Revision != revision {
			return ErrStatisticsConflict
		}
		input.Revision = strconv.FormatInt(revision+1, 10)
		payload, err := json.Marshal(input)
		if err != nil {
			return err
		}
		row := &models.StatisticsPreference{Uid: uid, Revision: revision + 1, Payload: string(payload)}
		if !has {
			_, err = sess.Insert(row)
		} else {
			var affected int64
			affected, err = sess.ID(uid).Where("revision=?", revision).Cols("revision", "payload").Update(row)
			if err == nil && affected != 1 {
				return ErrStatisticsConflict
			}
		}
		return err
	})
	return &input, err
}

func (s *StatisticsWorkspaceService) Budgets(c core.Context, uid int64) ([]*models.StatisticsBudget, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	items := make([]*models.StatisticsBudget, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("start_date, id").Find(&items)
	return items, err
}

func (s *StatisticsWorkspaceService) SaveBudget(c core.Context, uid int64, input models.StatisticsBudget) (*models.StatisticsBudget, error) {
	amount, err := normalizeStatisticsMoney(input.Amount)
	start, startErr := time.Parse("2006-01-02", input.StartDate)
	end, endErr := time.Parse("2006-01-02", input.EndDate)
	input.Name = strings.TrimSpace(input.Name)
	if uid <= 0 || err != nil || amount == "0.00" || startErr != nil || endErr != nil || start.Year() < 1900 || end.Year() > 9999 || end.Before(start) || len(input.Id) > 64 || len(input.BookId) > 64 || input.CategoryId < 0 || utf8.RuneCountInString(input.Name) > 64 || input.Revision < 0 {
		return nil, ErrStatisticsInvalid
	}
	if input.Kind == "monthly" {
		if start.Day() != 1 || !end.Equal(start.AddDate(0, 1, -1)) {
			return nil, ErrStatisticsInvalid
		}
	} else if input.Kind != "custom" || input.Repeat {
		return nil, ErrStatisticsInvalid
	}
	input.Amount, input.Uid = amount, uid
	defer Investments.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		bookId, err := Books.ResolveInSession(sess, uid, input.BookId, false)
		if err != nil {
			return err
		}
		input.BookId = bookId
		if input.CategoryId != 0 {
			var category models.TransactionCategory
			has, err := sess.Where("uid=? AND category_id=? AND deleted=? AND type=?", uid, input.CategoryId, false, models.CATEGORY_TYPE_EXPENSE).Get(&category)
			if err != nil {
				return err
			}
			if !has {
				return ErrStatisticsInvalid
			}
		}
		key := statisticsKey(strconv.FormatInt(uid, 10), input.BookId, input.Kind, input.StartDate, input.EndDate, strconv.FormatInt(input.CategoryId, 10))
		if input.Id != "" && input.Id != key {
			return ErrStatisticsInvalid
		}
		input.Id = key
		var current models.StatisticsBudget
		has, err := sess.ID(input.Id).Where("uid=?", uid).Get(&current)
		if err != nil {
			return err
		}
		if current.Revision != input.Revision {
			return ErrStatisticsConflict
		}
		input.Revision++
		if !has {
			_, err = sess.Insert(&input)
		} else {
			var affected int64
			affected, err = sess.ID(input.Id).Where("uid=? AND revision=?", uid, current.Revision).AllCols().Update(&input)
			if err == nil && affected != 1 {
				return ErrStatisticsConflict
			}
		}
		return err
	})
	return &input, err
}

func (s *StatisticsWorkspaceService) DeleteBudget(c core.Context, uid int64, req models.StatisticsDeleteRequest) error {
	if uid <= 0 || len(req.Id) != 64 || req.Revision < 1 {
		return ErrStatisticsInvalid
	}
	affected, err := s.UserDataDB(uid).NewSession(c).ID(req.Id).Where("uid=? AND revision=?", uid, req.Revision).Delete(&models.StatisticsBudget{})
	if err != nil {
		return err
	}
	if affected != 1 {
		return ErrStatisticsConflict
	}
	return nil
}

func (s *StatisticsWorkspaceService) Notes(c core.Context, uid int64) ([]*models.StatisticsNote, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	items := make([]*models.StatisticsNote, 0)
	err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).OrderBy("period DESC").Find(&items)
	return items, err
}

func (s *StatisticsWorkspaceService) SaveNote(c core.Context, uid int64, input models.StatisticsNote) (*models.StatisticsNote, error) {
	format := "2006"
	if len(input.Period) == 7 {
		format = "2006-01"
	}
	date, err := time.Parse(format, input.Period)
	if uid <= 0 || err != nil || !statisticsPeriodPattern.MatchString(input.Period) || date.Year() < 1900 || utf8.RuneCountInString(input.Content) > 10000 || len(input.Id) > 64 || len(input.BookId) > 64 || input.Revision < 0 {
		return nil, ErrStatisticsInvalid
	}
	defer Investments.lock(uid)()
	err = s.UserDataDB(uid).DoTransaction(c, func(sess *xorm.Session) error {
		if input.BookId != "" {
			if _, err := Books.ResolveInSession(sess, uid, input.BookId, true); err != nil {
				return err
			}
		}
		key := statisticsKey(fmt.Sprint(uid), input.BookId, input.Period)
		if input.Id != "" && input.Id != key {
			return ErrStatisticsInvalid
		}
		input.Id, input.Uid = key, uid
		var current models.StatisticsNote
		has, err := sess.ID(key).Where("uid=?", uid).Get(&current)
		if err != nil {
			return err
		}
		if current.Revision != input.Revision {
			return ErrStatisticsConflict
		}
		input.Revision++
		if !has {
			_, err = sess.Insert(&input)
		} else {
			var affected int64
			affected, err = sess.ID(key).Where("uid=? AND revision=?", uid, current.Revision).Cols("content", "revision").Update(&input)
			if err == nil && affected != 1 {
				return ErrStatisticsConflict
			}
		}
		return err
	})
	return &input, err
}

func (s *StatisticsWorkspaceService) Auxiliary(c core.Context, uid int64) (*models.StatisticsAuxiliary, error) {
	if uid <= 0 {
		return nil, errs.ErrUserIdInvalid
	}
	out := &models.StatisticsAuxiliary{DebtActions: map[string]string{}, FeeIds: []string{}}
	var movements []models.DebtMovement
	if err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&movements); err != nil {
		return nil, err
	}
	for _, row := range movements {
		out.DebtActions[strconv.FormatInt(row.PrincipalTransactionId, 10)] = row.Action
		if row.InterestTransactionId > 0 {
			out.FeeIds = append(out.FeeIds, strconv.FormatInt(row.InterestTransactionId, 10))
		}
	}
	var plans []models.CreditInstallment
	if err := s.UserDataDB(uid).NewSession(c).Where("uid=?", uid).Find(&plans); err != nil {
		return nil, err
	}
	for _, plan := range plans {
		if plan.Data == nil {
			continue
		}
		for _, payment := range plan.Data.Payments {
			if payment.FeeTransactionId != "" {
				out.FeeIds = append(out.FeeIds, payment.FeeTransactionId)
			}
		}
	}
	return out, nil
}
