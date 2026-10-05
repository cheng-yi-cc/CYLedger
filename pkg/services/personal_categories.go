package services

import (
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/errs"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"xorm.io/xorm"
)

type personalCategoryPreset struct {
	name     string
	typeID   models.TransactionCategoryType
	icon     int64
	children []string
}

// Every primary category has a selectable leaf, preserving the existing
// transaction/category model and its primary/secondary statistics.
var personalCategoryPresets = []personalCategoryPreset{
	{"创业", models.CATEGORY_TYPE_EXPENSE, 2000, nil},
	{"恋爱", models.CATEGORY_TYPE_EXPENSE, 500, nil},
	{"其他", models.CATEGORY_TYPE_EXPENSE, 1000, nil},
	{"购物消费", models.CATEGORY_TYPE_EXPENSE, 100, []string{"服饰鞋包", "数码产品", "日用百货"}},
	{"食品餐饮", models.CATEGORY_TYPE_EXPENSE, 1, []string{"早餐", "午餐", "晚餐", "零食饮料"}},
	{"出行交通", models.CATEGORY_TYPE_EXPENSE, 300, []string{"公共交通", "打车", "自驾", "火车机票"}},
	{"休闲娱乐", models.CATEGORY_TYPE_EXPENSE, 550, []string{"影音游戏", "运动健身", "旅行"}},
	{"居家生活", models.CATEGORY_TYPE_EXPENSE, 200, []string{"房租", "水费", "电费", "燃气", "通讯网络"}},
	{"文化教育", models.CATEGORY_TYPE_EXPENSE, 600, []string{"书籍", "课程培训", "学习用品"}},
	{"送礼人情", models.CATEGORY_TYPE_EXPENSE, 700, []string{"礼物", "红包礼金", "聚会请客"}},
	{"健康医疗", models.CATEGORY_TYPE_EXPENSE, 800, []string{"看病", "药品", "体检"}},
	{"工资薪酬", models.CATEGORY_TYPE_INCOME, 2000, []string{"工资", "奖金"}},
	{"兼职副业", models.CATEGORY_TYPE_INCOME, 2010, nil},
	{"创业收入", models.CATEGORY_TYPE_INCOME, 2000, nil},
	{"投资收益", models.CATEGORY_TYPE_INCOME, 2100, nil},
	{"礼金红包", models.CATEGORY_TYPE_INCOME, 700, nil},
	{"其他收入", models.CATEGORY_TYPE_INCOME, 3010, nil},
	{"资金往来", models.CATEGORY_TYPE_TRANSFER, 4000, []string{"借入", "借出", "还款", "收回"}},
}

// EnsurePersonalCategories initializes a genuinely empty personal category list.
// Deleted rows count too, so intentionally removed categories never reappear.
func (s *TransactionCategoryService) EnsurePersonalCategories(c core.Context, uid int64) error {
	if uid <= 0 {
		return errs.ErrUserIdInvalid
	}
	return s.UserDataDB(uid).DoTransaction(c, func(session *xorm.Session) error {
		count, err := session.Where("uid=?", uid).Count(&models.TransactionCategory{})
		if err != nil || count > 0 {
			return err
		}
		now := time.Now().Unix()
		orders := map[models.TransactionCategoryType]int32{}
		for _, preset := range personalCategoryPresets {
			orders[preset.typeID]++
			parent := &models.TransactionCategory{CategoryId: s.GenerateUuid(uuid.UUID_TYPE_CATEGORY), Uid: uid,
				Name: preset.name, Type: preset.typeID, Icon: preset.icon, Color: "12786f", BookIds: []string{},
				DisplayOrder: orders[preset.typeID], CreatedUnixTime: now, UpdatedUnixTime: now}
			if parent.CategoryId <= 0 {
				return errs.ErrSystemIsBusy
			}
			if _, err = session.Insert(parent); err != nil {
				return err
			}
			for index, name := range append([]string{preset.name}, preset.children...) {
				child := &models.TransactionCategory{CategoryId: s.GenerateUuid(uuid.UUID_TYPE_CATEGORY), Uid: uid,
					ParentCategoryId: parent.CategoryId, Name: name, Type: preset.typeID, Icon: preset.icon,
					Color: parent.Color, BookIds: []string{}, DisplayOrder: int32(index + 1), CreatedUnixTime: now, UpdatedUnixTime: now}
				if child.CategoryId <= 0 {
					return errs.ErrSystemIsBusy
				}
				if _, err = session.Insert(child); err != nil {
					return err
				}
			}
		}
		return nil
	})
}
