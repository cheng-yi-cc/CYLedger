package main

import (
	"encoding/json"
	"fmt"
	"sync"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
)

var sessionMutex sync.Mutex

// personalOwner preserves the existing UID. Never silently pick a user from a
// multi-user database or replace a disabled/deleted personal ledger.
func personalOwner() (*models.User, error) {
	ctx := core.NewNullContext()
	session := services.Users.UserDB().NewSession(ctx)
	var users []*models.User
	err := session.Limit(2).Find(&users)
	session.Close()
	if err != nil {
		return nil, err
	}
	if len(users) == 1 && !users[0].Deleted && !users[0].Disabled {
		return users[0], nil
	}
	if len(users) != 0 {
		return nil, fmt.Errorf("personal ledger owner is ambiguous or unavailable")
	}
	user := &models.User{
		Username: "local", Email: "local@cyledger.invalid", Nickname: "我的账本",
		Language: "zh-Hans", DefaultCurrency: "CNY", FirstDayOfWeek: core.WeekDay(1),
		FiscalYearStart:      core.FISCAL_YEAR_START_DEFAULT,
		TransactionEditScope: models.TRANSACTION_EDIT_SCOPE_ALL,
		FeatureRestriction:   services.Users.CurrentConfig().DefaultFeatureRestrictions,
	}
	if err := services.Users.CreateUser(ctx, user, true); err != nil {
		return nil, err
	}
	return user, nil
}

func personalSession() (string, error) {
	sessionMutex.Lock()
	defer sessionMutex.Unlock()
	user, err := personalOwner()
	if err != nil {
		return "", err
	}
	if err := services.TransactionCategories.EnsurePersonalCategories(core.NewNullContext(), user.Uid); err != nil {
		return "", err
	}
	token, err := services.Tokens.CreateNativeLocalToken(core.NewNullContext(), user)
	if err != nil {
		return "", err
	}
	data, err := json.Marshal(models.AuthResponse{
		Token: token, User: user.ToUserBasicInfo(services.Users.CurrentConfig().AvatarProvider, ""),
	})
	return string(data), err
}
