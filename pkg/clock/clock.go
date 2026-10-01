// Package clock abstracts time so domain rules are testable.
package clock

import (
	"sync"
	"time"
)

// Clock returns the current time.
type Clock interface {
	Now() time.Time
}

// Real is the system clock, in UTC.
type Real struct{}

func (Real) Now() time.Time { return time.Now().UTC() }

// Fake is a manually advanced clock for tests.
type Fake struct {
	mu sync.Mutex
	t  time.Time
}

func NewFake(t time.Time) *Fake { return &Fake{t: t} }

func (f *Fake) Now() time.Time {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.t
}

// Advance moves the clock forward by d.
func (f *Fake) Advance(d time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.t = f.t.Add(d)
}

// Addis is Ethiopia's time zone (UTC+3, no daylight saving). Business dates
// such as document expiry are calendar dates in this zone (design doc 9.5).
var Addis = time.FixedZone("EAT", 3*60*60)

// Today returns the current calendar date in Addis Ababa, as midnight UTC so
// it compares directly with dates read from the API or the database.
func Today(c Clock) time.Time {
	y, m, d := c.Now().In(Addis).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}
