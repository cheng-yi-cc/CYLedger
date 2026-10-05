package monetaryincome

import (
	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func stamp(value string) int64 {
	z, _ := time.LoadLocation("Asia/Shanghai")
	t, _ := time.ParseInLocation("2006-01-02 15:04:05", value, z)
	return t.Unix()
}

func TestMonetaryCutoffWeekendHolidayAndUnknownYear(t *testing.T) {
	for _, v := range [][2]string{{"2026-09-30", "2026-09-29 15:00:00"}, {"2026-10-01", "2026-09-29 15:00:00"}, {"2026-10-07", "2026-09-29 15:00:00"}, {"2026-10-08", "2026-09-30 15:00:00"}, {"2026-09-20", "2026-09-17 15:00:00"}, {"2025-01-01", "2024-12-30 15:00:00"}} {
		in, _, err := Cutoffs(v[0])
		require.NoError(t, err)
		require.Equal(t, stamp(v[1]), in.Unix(), v[0])
	}
	_, _, err := Cutoffs("2027-01-04")
	require.ErrorIs(t, err, ErrCalendar)
}

func TestMonetaryPrincipalCutoffTransfersRefundsAndRounding(t *testing.T) {
	flows := []Flow{
		{At: stamp("2026-09-28 18:00:00"), Minor: 1000000, Opening: true},
		{At: stamp("2026-09-29 14:59:59"), Minor: 500000},
		{At: stamp("2026-09-29 15:00:00"), Minor: 900000},
		{At: stamp("2026-09-30 22:00:00"), Minor: -200000},
		{At: stamp("2026-10-01 00:00:00"), Minor: -300000},
	}
	p, err := Principal("2026-09-30", flows)
	require.NoError(t, err)
	require.Equal(t, "13000", p.String())
	require.Equal(t, "1.3", Income(p, decimal.RequireFromString("1")).String())
	require.Equal(t, "0.01", Income(decimal.NewFromInt(100), decimal.RequireFromString("0.5")).String())
	p, err = Principal("2026-09-30", []Flow{{At: stamp("2026-09-01 00:00:00"), Minor: -1}})
	require.NoError(t, err)
	require.True(t, p.IsZero())
}
