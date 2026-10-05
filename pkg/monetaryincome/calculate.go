// Package monetaryincome calculates local rule-based money-market income.
// It does not claim to reproduce a distributor's actual confirmed shares.
package monetaryincome

import (
	"errors"
	"time"

	"github.com/shopspring/decimal"
)

var ErrCalendar = errors.New("该日期的交易日历尚未维护，已暂停收益计算")

// SSE annual closure notices, independent calendar facts, not third-party code:
// https://www.sse.com.cn/disclosure/announcement/general/c/c_20231226_5733939.shtml
// https://www.sse.com.cn/disclosure/announcement/general/c/c_20241223_10767108.shtml
// https://www.sse.com.cn/disclosure/announcement/general/c/c_20251222_10802507.shtml
var closures = map[int][][2]string{
	2024: {{"01-01", "01-01"}, {"02-09", "02-17"}, {"04-04", "04-06"}, {"05-01", "05-05"}, {"06-10", "06-10"}, {"09-15", "09-17"}, {"10-01", "10-07"}},
	2025: {{"01-01", "01-01"}, {"01-28", "02-04"}, {"04-04", "04-06"}, {"05-01", "05-05"}, {"05-31", "06-02"}, {"10-01", "10-08"}},
	2026: {{"01-01", "01-03"}, {"02-15", "02-23"}, {"04-04", "04-06"}, {"05-01", "05-05"}, {"06-19", "06-21"}, {"09-25", "09-27"}, {"10-01", "10-07"}},
}

func previousOrSameTradingDay(day time.Time) (time.Time, error) {
	for n := 0; n < 40; n++ {
		ranges, ok := closures[day.Year()]
		if !ok {
			return time.Time{}, ErrCalendar
		}
		closed := day.Weekday() == time.Saturday || day.Weekday() == time.Sunday
		md := day.Format("01-02")
		for _, r := range ranges {
			if md >= r[0] && md <= r[1] {
				closed = true
				break
			}
		}
		if !closed {
			return day, nil
		}
		day = day.AddDate(0, 0, -1)
	}
	return time.Time{}, ErrCalendar
}

// Cutoffs matches the cash-account rule: inflows before 15:00 of the trading
// day preceding D's latest trading day; outflows through D's calendar day.
// Exactly 15:00 is after the cutoff. Market days always use Shanghai time.
func Cutoffs(date string) (inflow, outflow time.Time, err error) {
	zone, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		return inflow, outflow, err
	}
	d, err := time.ParseInLocation("2006-01-02", date, zone)
	if err != nil || len(date) != 10 {
		return inflow, outflow, errors.New("收益日期无效")
	}
	trading, err := previousOrSameTradingDay(d)
	if err != nil {
		return inflow, outflow, err
	}
	previous, err := previousOrSameTradingDay(trading.AddDate(0, 0, -1))
	if err != nil {
		return inflow, outflow, err
	}
	return previous.Add(15 * time.Hour), d.AddDate(0, 0, 1), nil
}

type Flow struct {
	At      int64
	Minor   int64 // signed balance change, in cents
	Opening bool  // first balance fact is already-held principal, not a purchase
}

func Principal(date string, flows []Flow) (decimal.Decimal, error) {
	in, out, err := Cutoffs(date)
	if err != nil {
		return decimal.Zero, err
	}
	total := decimal.Zero
	for _, f := range flows {
		cutoff := in.Unix()
		if f.Minor < 0 || f.Opening {
			cutoff = out.Unix()
		}
		if f.At < cutoff {
			total = total.Add(decimal.New(f.Minor, -2))
		}
	}
	if total.IsNegative() {
		return decimal.Zero, nil
	}
	return total, nil
}

func Income(principal, perTenThousand decimal.Decimal) decimal.Decimal {
	return principal.Mul(perTenThousand).Div(decimal.NewFromInt(10000)).Round(2)
}
