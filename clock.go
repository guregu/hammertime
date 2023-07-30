package mywasi

import "time"

type Clock interface {
	Now() time.Time
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

var SystemClock Clock = systemClock{}

type FixedClock time.Time

func (fc FixedClock) Now() time.Time {
	return time.Time(fc)
}
