package optime

import (
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/keep94/sunrise"
)

type OpTime struct {
}

// SunriseOutput is a struct to hold the sunrise information
type SunriseOutput struct {
	Begin     time.Time
	End       time.Time
	Phase     string
	Latitude  float64
	Longitude float64
	Since     time.Duration
	Until     time.Duration
}

var _ base.FreepsOperator = &OpTime{}

// GeoLocation is a struct to hold latitude and longitude
type GeoLocation struct {
	Latitude  float64
	Longitude float64
}

func (o *OpTime) sunrise(g GeoLocation) SunriseOutput {
	now := time.Now()
	dayOrNight, start, end := sunrise.DayOrNight(g.Latitude, g.Longitude, time.Now())
	s := SunriseOutput{Begin: start, End: end, Phase: "day", Latitude: g.Latitude, Longitude: g.Longitude, Since: now.Sub(start), Until: end.Sub(now)}
	if dayOrNight == sunrise.Night {
		s.Phase = "night"
	}
	return s
}

// IsDay returns the SunriseOutput for the given location if it is day, otherwise an error
func (o *OpTime) IsDay(ctx *base.Context, input *base.OperatorIO, g GeoLocation) *base.OperatorIO {
	res := o.sunrise(g)
	if res.Phase != "day" {
		return base.MakeOutputError(http.StatusExpectationFailed, "It's dark!")
	}
	return base.MakeObjectOutput(res)
}

// IsNight returns the SunriseOutput for the given location if it is night, otherwise an error
func (o *OpTime) IsNight(ctx *base.Context, input *base.OperatorIO, g GeoLocation) *base.OperatorIO {
	res := o.sunrise(g)
	if res.Phase != "night" {
		return base.MakeOutputError(http.StatusExpectationFailed, "It's day!")
	}
	return base.MakeObjectOutput(res)
}

// Sunrise returns the SunriseOutput for the given location
func (o *OpTime) Sunrise(ctx *base.Context, input *base.OperatorIO, g GeoLocation) *base.OperatorIO {
	res := o.sunrise(g)
	return base.MakeObjectOutput(res)
}

// SleepParameter is a struct to hold the duration to sleep
type SleepParameter struct {
	Duration time.Duration
}

// Sleep sleeps for the given duration
func (o *OpTime) Sleep(ctx *base.Context, input *base.OperatorIO, s SleepParameter) *base.OperatorIO {
	time.Sleep(s.Duration)
	return base.MakeEmptyOutput()
}

// NowParameter is a struct to hold the format to use
type NowParameter struct {
	Format *string
}

// Now returns the current time
func (o *OpTime) Now(ctx *base.Context, input *base.OperatorIO, n NowParameter) *base.OperatorIO {
	if n.Format != nil {
		return base.MakePlainOutput("%v", time.Now().Format(*n.Format))
	}
	return base.MakePlainOutput("%v", time.Now())
}
