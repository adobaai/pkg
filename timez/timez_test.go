package timez

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIt(t *testing.T) {
	first := time.Date(2024, 10, 1, 8, 11, 0, 0, time.Local)
	second := time.Date(2024, 11, 11, 14, 0, 9, 0, time.Local)

	t.Run("LocalDate", func(t *testing.T) {
		t1 := LocalDate(2023, 10, 1)
		t2 := LocalDate(2023, 10, 2)
		if t1.Year() != 2023 || t1.Month() != 10 || t1.Day() != 1 {
			t.Errorf("Expected date to be 2023-10-01, got %s", t1)
		}
		if t2.Year() != 2023 || t2.Month() != 10 || t2.Day() != 2 {
			t.Errorf("Expected date to be 2023-10-02, got %s", t2)
		}
	})

	t.Run("UTCDate", func(t *testing.T) {
		t1 := UTCDate(2023, 10, 1)
		t2 := UTCDate(2023, 10, 2)
		if t1.Year() != 2023 || t1.Month() != 10 || t1.Day() != 1 {
			t.Errorf("Expected date to be 2023-10-01, got %s", t1)
		}
		if t2.Year() != 2023 || t2.Month() != 10 || t2.Day() != 2 {
			t.Errorf("Expected date to be 2023-10-02, got %s", t2)
		}
	})

	t.Run("ToJSON", func(t *testing.T) {
		assert.Equal(t, "2024-10-01T08:11:00+08:00", ToJSON(first))
		assert.Equal(t, "2024-11-11T14:00:09+08:00", ToJSON(second))
	})

	t.Run("DateOf", func(t *testing.T) {
		dur := DateOf(second).Sub(DateOf(first))
		assert.Equal(t, 41, int(dur/Day))
	})

	t.Run("DayStart", func(t *testing.T) {
		d := time.Date(2020, 2, 1, 0, 0, 0, 0, time.Local)
		want := time.Date(2020, 2, 1, 0, 0, 0, 0, time.Local)
		assert.Equal(t, want, DayStart(d))
	})

	t.Run("DayEnd", func(t *testing.T) {
		d := time.Date(2020, 2, 1, 0, 0, 0, 0, time.Local)
		want := time.Date(2020, 2, 1, 23, 59, 59, 999999999, time.Local)
		assert.Equal(t, want, DayEnd(d))
	})

	t.Run("Today", func(t *testing.T) {
		assert.Equal(t, DateOf(time.Now()), Today())
	})

	t.Run("Yesterday", func(t *testing.T) {
		assert.Equal(t, DateOf(YesterdayNow()), Yesterday())
		assert.Equal(t, Today().AddDate(0, 0, -1), Yesterday())
	})
}

func TestDuration(t *testing.T) {
	assert.Equal(t, Dur(time.Hour), Duration{time.Hour})
	assert.True(t, Dur(0).IsZero())
	assert.False(t, Dur(1).IsZero())

	d := Duration{time.Second}
	b, err := json.Marshal(d)
	require.NoError(t, err)
	assert.Equal(t, []byte(`"1s"`), b)

	err = json.Unmarshal([]byte(`"1ns"`), &d)
	require.NoError(t, err)
	assert.Equal(t, Duration{1}, d)

	type Config struct {
		Timeout Duration
	}
	c := Config{
		Timeout: Duration{time.Minute},
	}
	b, err = json.Marshal(c)
	require.NoError(t, err)
	assert.Equal(t, []byte(`{"Timeout":"1m0s"}`), b)

	err = json.Unmarshal([]byte(`{"Timeout":"1ms"}`), &c)
	require.NoError(t, err)
	assert.Equal(t, Config{Timeout: Duration{time.Millisecond}}, c)

	err = json.Unmarshal([]byte(`{"Timeout":10}`), &c)
	assert.EqualError(t, err, "invalid token: 10")
}
