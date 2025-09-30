// Package exchange provides some API client for exchange rates.
package exchange

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/adobaai/pkg/netz/httpz"
	"go.uber.org/multierr"
)

var ErrNotFound = errors.New("not found")

type Exchanger interface {
	// GetExchangeRate returns the exchange rate at the given time.
	// If the time is zero time, the latest exchange rate is returned.
	GetExchangeRate(
		ctx context.Context, from, to string, time time.Time,
	) (float64, time.Time, error)
}

// Frankfurter is a free API that provides exchange rates for over 200 currencies.
// Only support daily exchange rates.
// The input time will be formatted in UTC timezone.
type Frankfurter struct {
	hc *http.Client
}

type FrankfurterResp struct {
	Base  string             `json:"base"`
	Date  string             `json:"date"` // 2025-06-13
	Rates map[string]float64 `json:"rates"`
}

func NewFrankfurter(hc *http.Client) *Frankfurter {
	return &Frankfurter{
		hc: hc,
	}
}

func (f *Frankfurter) GetExchangeRate(ctx context.Context, from, to string, t time.Time,
) (n float64, dt time.Time, err error) {
	ts := t.UTC().Format(time.DateOnly)
	if t.IsZero() {
		ts = "latest"
	}

	from, to = strings.ToUpper(from), strings.ToUpper(to)
	url := fmt.Sprintf("https://api.frankfurter.dev/v1/%s?base=%s&symbols=%s", ts, from, to)
	hres, err := httpz.RawJSON(ctx, http.DefaultClient, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	if hres.StatusCode == http.StatusNotFound {
		defer multierr.AppendFunc(&err, hres.Body.Close)
		err = ErrNotFound
		return
	}

	resp, err := httpz.RespJSON[FrankfurterResp](hres)
	if err != nil {
		return
	}

	if rate, ok := resp.Rates[to]; ok {
		n = rate
		dt, err = time.Parse(time.DateOnly, resp.Date)
		return
	}

	err = ErrNotFound
	return
}

// Fawazahmed0 is a free API that provides exchange rates for over 200 currencies.
// Only support daily exchange rates.
// The input time will be formatted in UTC timezone.
type Fawazahmed0 struct {
	hc *http.Client
}

func NewFawazahmed0(hc *http.Client) *Fawazahmed0 {
	return &Fawazahmed0{
		hc: hc,
	}
}

type Fawazahmed0Resp map[string]any

func (f *Fawazahmed0) GetExchangeRate(ctx context.Context, from, to string, t time.Time,
) (n float64, dt time.Time, err error) {
	ts := t.UTC().Format(time.DateOnly)
	if t.IsZero() {
		ts = "latest"
	}

	from, to = strings.ToLower(from), strings.ToLower(to)
	base := "https://cdn.jsdelivr.net/npm/@fawazahmed0/currency-api"
	url := fmt.Sprintf("%s@%s/v1/currencies/%s.json", base, ts, from)

	hres, err := httpz.RawJSON(ctx, f.hc, http.MethodGet, url, nil)
	if err != nil {
		return
	}

	if hres.StatusCode == http.StatusNotFound {
		defer multierr.AppendFunc(&err, hres.Body.Close)
		err = ErrNotFound
		return
	}

	resp, err := httpz.RespJSON[Fawazahmed0Resp](hres)
	if err != nil {
		return
	}

	defer func() {
		// To recover from type assertions
		if r := recover(); r != nil {
			err = fmt.Errorf("panic: %v", r)
		}
	}()

	dateStr := (*resp)["date"].(string)
	date, err := time.Parse(time.DateOnly, dateStr)
	if err != nil {
		return
	}

	n = (*resp)[from].(map[string]any)[to].(float64)
	if n == 0 {
		err = ErrNotFound
		return
	}

	return n, date, err
}
