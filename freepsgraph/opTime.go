package freepsgraph

import (
	"fmt"
	"net/http"
	"strconv"
	"time"

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

var _ FreepsOperator = &OpTime{}

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

func (o *OpTime) Execute(function string, vars map[string]string, mainInput *OperatorIO) *OperatorIO {
	switch function {
	case "sunrise":
		res, err := o.sunrise(vars)
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		return MakeObjectOutput(*res)
	case "now":
		return MakePlainOutput(time.Now().GoString())
	default:
		return MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}
}

func (o *OpTime) GetFunctions() []string {
	return []string{"sunrise", "now"}
}

func (o *OpTime) GetPossibleArgs(fn string) []string {
	return []string{"latitude", "longitude"}
}

func (o *OpTime) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}
