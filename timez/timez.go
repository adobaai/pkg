package timez

import (
	"fmt"
	"time"
)

const (
	// Day is the duration of one day.
	Day = 24 * time.Hour
	// Week is the duration of one week.
	Week = 7 * Day
)

// ToJSON gets the time as a JSON string.
func ToJSON(t time.Time) string {
	return t.Format(time.RFC3339)
}

// LocalDate gets the local date for the given year, month and day.
func LocalDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.Local)
}

// UTCDate gets the UTC date for the given year, month and day.
func UTCDate(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

// Return the date part of the given time.
func DateOf(t time.Time) time.Time {
	return DayStart(t)
}

// DayStart gets the start of the day for the given time.
func DayStart(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

// DayEnd gets the end of the day for the given time.
func DayEnd(t time.Time) time.Time {
	return DateOf(t).AddDate(0, 0, 1).Add(-1)
}

// Today gets the current date.
func Today() time.Time {
	return DateOf(time.Now())
}

// Yesterday gets the yesterday's date.
func Yesterday() time.Time {
	return DateOf(YesterdayNow())
}

// YesterdayNow gets the yesterday's date.
func YesterdayNow() time.Time {
	return time.Now().AddDate(0, 0, -1)
}

// Duration is a wrapper for [time.Duration] with JSON support.
type Duration struct {
	time.Duration
}

func Dur(d time.Duration) Duration {
	return Duration{Duration: d}
}

func (d Duration) IsZero() bool {
	return d.Duration == 0
}

func (d Duration) MarshalJSON() (b []byte, err error) {
	return []byte(`"` + d.String() + `"`), nil
}

func (d *Duration) UnmarshalJSON(b []byte) (err error) {
	length := len(b)
	if len(b) <= 2 || b[0] != '"' || b[length-1] != '"' {
		return fmt.Errorf("invalid token: %s", b)
	}
	d.Duration, err = time.ParseDuration(string(b[1 : length-1]))
	return
}
