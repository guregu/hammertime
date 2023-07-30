package hammertime

import "time"

// Clock can be used to customize the current time.
type Clock interface {
	// Now reports the current time.
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

// SystemClock uses the host OS time.
var SystemClock Clock = systemClock{}

// FixedClock always reports the same time (itself).
type FixedClock time.Time

func (fc FixedClock) Now() time.Time {
	return time.Time(fc)
}
