package exchange

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExchangers(t *testing.T) {
	var (
		ctx = context.Background()
		hc  = http.DefaultClient
	)

	test := func(t *testing.T, ex Exchanger) {
		t.Run("Now", func(t *testing.T) {
			n, dt, err := ex.GetExchangeRate(ctx, "EUR", "USD", time.Now())
			require.NoError(t, err)
			t.Logf("EURUSD, d: %v, dt: %v", n, dt)
		})

		t.Run("LastMonth", func(t *testing.T) {
			n, dt, err := ex.GetExchangeRate(ctx, "EUR", "USD", time.Now().AddDate(0, -1, 0))
			require.NoError(t, err)
			t.Logf("EURUSD, d: %v, dt: %v", n, dt)
		})

		t.Run("Lowercase", func(t *testing.T) {
			date := time.Date(2025, 02, 03, 0, 0, 0, 0, time.UTC)
			n, dt, err := ex.GetExchangeRate(ctx, "cny", "USD", date)
			require.NoError(t, err)
			assert.Equal(t, date, dt)
			t.Logf("cnyUSD, d: %v, dt: %v", n, dt)
		})
	}

	t.Run("Fawazahmed0", func(t *testing.T) {
		ex := NewFawazahmed0(hc)
		test(t, ex)
	})

	t.Run("Frankfurter", func(t *testing.T) {
		ex := NewFrankfurter(hc)
		test(t, ex)

		t1 := time.Date(2025, 4, 21, 0, 0, 0, 0, time.UTC)
		d, dt, err := ex.GetExchangeRate(ctx, "EUR", "USD", t1)
		require.NoError(t, err)
		// Which is not stable
		assert.Equal(t, time.Date(2025, 4, 17, 0, 0, 0, 0, time.UTC), dt)
		t.Logf("EURUSD, d: %v, dt: %v", d, dt)
	})
}
