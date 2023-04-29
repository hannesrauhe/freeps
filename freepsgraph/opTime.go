package freepsgraph

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/keep94/sunrise"
)

type OpTime struct {
}

type SunriseOutput struct {
	Begin     time.Time
	End       time.Time
	Phase     string
	Latitude  float64
	Longitude float64
	Since     time.Duration
	Until     time.Duration
}

var _ base.FreepsBaseOperator = &OpTime{}

// GetName returns the name of the operator
func (o *OpTime) GetName() string {
	return "time"
}

func (o *OpTime) sunrise(vars map[string]string) (*SunriseOutput, error) {
	lats, ok := vars["latitude"]
	if !ok {
		return nil, fmt.Errorf("Latitude missing")
	}
	longs, ok := vars["longitude"]
	if !ok {
		return nil, fmt.Errorf("Longitude missing")
	}
	lat, err := strconv.ParseFloat(lats, 64)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse Latitude: %v", err.Error())
	}
	long, err := strconv.ParseFloat(longs, 64)
	if err != nil {
		return nil, fmt.Errorf("Cannot parse Longitude: %v", err.Error())
	}
	now := time.Now()
	dayOrNight, start, end := sunrise.DayOrNight(lat, long, time.Now())
	s := SunriseOutput{Begin: start, End: end, Phase: "day", Latitude: lat, Longitude: long, Since: now.Sub(start), Until: end.Sub(now)}
	if dayOrNight == sunrise.Night {
		s.Phase = "night"
	}
	return &s, nil
}

func (o *OpTime) sunriseFunctions(function string, vars map[string]string) *base.OperatorIO {
	res, err := o.sunrise(vars)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	switch function {
	case "isDay":
		if res.Phase != "day" {
			return base.MakeOutputError(http.StatusExpectationFailed, "It's dark!")
		}
	case "isNight":
		if res.Phase != "night" {
			return base.MakeOutputError(http.StatusExpectationFailed, "It's day!")
		}
	}

	return base.MakeObjectOutput(*res)
}

func (o *OpTime) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	switch function {
	case "sunrise":
		fallthrough
	case "isDay":
		fallthrough
	case "isNight":
		return o.sunriseFunctions(function, vars)
	case "sleep":
		dstr, ok := vars["duration"]
		if !ok {
			return base.MakeOutputError(http.StatusBadRequest, "duration missing")
		}
		d, err := time.ParseDuration(dstr)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, "duration parsing failed: %v", err)
		}
		time.Sleep(d)
		return base.MakeEmptyOutput()
	case "now":
		if vars["format"] != "" {
			return base.MakePlainOutput("%v", time.Now().Format(vars["format"]))
		}
		return base.MakePlainOutput("%v", time.Now())
	default:
		return base.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}
}

func (o *OpTime) GetFunctions() []string {
	return []string{"sunrise", "isDay", "isNight", "now", "sleep"}
}

func (o *OpTime) GetPossibleArgs(fn string) []string {
	switch fn {
	case "sunrise":
		fallthrough
	case "isDay":
		fallthrough
	case "isNight":
		return []string{"latitude", "longitude"}
	case "sleep":
		return []string{"duration"}
	case "now":
		return []string{"format"}
	}
	return []string{}
}

func (o *OpTime) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	switch arg {
	case "duration":
		return utils.GetDurationMap()
	}
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpTime) Shutdown(ctx *base.Context) {
}
