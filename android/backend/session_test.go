package main

import (
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/mayswind/ezbookkeeping/pkg/core"
	"github.com/mayswind/ezbookkeeping/pkg/datastore"
	"github.com/mayswind/ezbookkeeping/pkg/models"
	"github.com/mayswind/ezbookkeeping/pkg/services"
	"github.com/mayswind/ezbookkeeping/pkg/settings"
	"github.com/mayswind/ezbookkeeping/pkg/uuid"
	"github.com/stretchr/testify/require"
	"xorm.io/xorm"
)

func personalDB(t *testing.T) *xorm.Engine {
	t.Helper()
	previousDB, previousUUID := *datastore.Container, *uuid.Container
	previousConfig := settings.Container.GetCurrentConfig()
	config := &settings.Config{
		DatabaseConfig: &settings.DatabaseConfig{DatabaseType: settings.Sqlite3DbType,
			DatabasePath: filepath.Join(t.TempDir(), "personal.db"), MaxOpenConnection: 1},
		UuidGeneratorType: settings.InternalUuidGeneratorType, UuidServerId: 231,
		TokenExpiredTimeDuration: time.Hour,
	}
	settings.SetCurrentConfig(config)
	require.NoError(t, datastore.InitializeDataStore(config))
	require.NoError(t, uuid.InitializeUuidGenerator(config))
	session := services.Users.UserDB().NewSession(nil)
	engine := session.Engine()
	require.NoError(t, session.Close())
	t.Cleanup(func() {
		require.NoError(t, engine.Close())
		*datastore.Container, *uuid.Container = previousDB, previousUUID
		settings.SetCurrentConfig(previousConfig)
	})
	require.NoError(t, engine.Sync2(&models.User{}, &models.TokenRecord{}, &models.Account{}, &models.Transaction{}, &models.TransactionCategory{}))
	return engine
}

func TestPersonalSessionStartsEmptyAndKeepsOwner(t *testing.T) {
	engine := personalDB(t)
	value, err := personalSession()
	require.NoError(t, err)
	var response models.AuthResponse
	require.NoError(t, json.Unmarshal([]byte(value), &response))
	require.Equal(t, "CNY", response.User.DefaultCurrency)
	require.Equal(t, "zh-Hans", response.User.Language)
	_, claims, _, err := services.Tokens.ParseToken(core.NewNullContext(), response.Token)
	require.NoError(t, err)
	owner, err := personalOwner()
	require.NoError(t, err)
	require.Equal(t, owner.Uid, claims.Uid)
	require.Empty(t, owner.Password)
	_, err = personalSession()
	require.NoError(t, err)
	count, err := engine.Count(&models.User{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
	for _, model := range []any{&models.Account{}, &models.Transaction{}} {
		count, err = engine.Count(model)
		require.NoError(t, err)
		require.Zero(t, count)
	}
}

func TestPersonalUpgradePreservesExistingLedger(t *testing.T) {
	engine := personalDB(t)
	user := &models.User{Uid: 101, Username: "existing", Email: "existing@example.invalid",
		Nickname: "已有账本", Password: "unchanged-hash", DefaultCurrency: "CNY"}
	account := &models.Account{AccountId: 201, Uid: 101, Name: "原账户", Currency: "CNY", Balance: 45678}
	transaction := &models.Transaction{TransactionId: 301, Uid: 101, AccountId: 201, Amount: 1234}
	_, err := engine.Insert(user, account, transaction)
	require.NoError(t, err)
	_, err = personalSession()
	require.NoError(t, err)
	owner, err := personalOwner()
	require.NoError(t, err)
	require.Equal(t, *user, *owner)
	var gotAccount models.Account
	_, err = engine.ID(201).Get(&gotAccount)
	require.NoError(t, err)
	require.Equal(t, *account, gotAccount)
	var gotTransaction models.Transaction
	_, err = engine.ID(301).Get(&gotTransaction)
	require.NoError(t, err)
	require.Equal(t, *transaction, gotTransaction)
	count, err := engine.Count(&models.User{})
	require.NoError(t, err)
	require.Equal(t, int64(1), count)
}

func TestPersonalSessionRefusesAmbiguousOrUnavailableOwner(t *testing.T) {
	for _, state := range []string{"multiple", "disabled", "deleted"} {
		t.Run(state, func(t *testing.T) {
			engine := personalDB(t)
			user := &models.User{Uid: 101, Username: "existing", Email: "existing@example.invalid",
				Disabled: state == "disabled", Deleted: state == "deleted"}
			_, err := engine.Insert(user)
			require.NoError(t, err)
			if state == "multiple" {
				_, err = engine.Insert(&models.User{Uid: 102, Username: "second", Email: "second@example.invalid"})
				require.NoError(t, err)
			}
			_, err = personalSession()
			require.Error(t, err)
			count, err := engine.Count(&models.TokenRecord{})
			require.NoError(t, err)
			require.Zero(t, count)
		})
	}
}
