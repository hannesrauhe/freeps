package weather

import (
	"net/http"
	"os"
	"path"

	owm "github.com/briandowns/openweathermap"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type OpenWeatherMapConfig struct {
	APIKey   string
	Location string
	Units    string
	Lang     string
}

type OpWeather struct {
	conf OpenWeatherMapConfig
}

var _ base.FreepsOperatorWithConfig = &OpWeather{}

func (o *OpWeather) GetDefaultConfig() interface{} {
	return &OpenWeatherMapConfig{Units: "c", Lang: "en"}
}

func (o *OpWeather) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*OpenWeatherMapConfig)
	return &OpWeather{conf: *cfg}, nil
}

type WeatherArgs struct {
	Location *string
	Units    *string
	Lang     *string
}

var _ base.FreepsFunctionParametersWithInit = &WeatherArgs{}

// Init initializes the arguments
func (o *WeatherArgs) Init(ctx *base.Context, op base.FreepsOperator, fn string) {
	c := op.(*OpWeather).conf
	o.Location = &c.Location
	o.Units = &c.Units
	o.Lang = &c.Lang
}

func (o *OpWeather) Current(ctx *base.Context, mainInput *base.OperatorIO, args WeatherArgs) *base.OperatorIO {
	wm, err := owm.NewCurrent(*args.Units, *args.Lang, o.conf.APIKey)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	wm.CurrentByName(*args.Location)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	if wm.ID == 0 {
		return base.MakeOutputError(http.StatusInternalServerError, "ID of response is 0")
	}
	return base.MakeObjectOutput(wm)
}

type IconArgs struct {
	Icon string
}

func (o *OpWeather) Icon(ctx *base.Context, mainInput *base.OperatorIO, args IconArgs) *base.OperatorIO {
	d, _ := utils.GetTempDir()
	icon := path.Base(args.Icon)
	icon = icon + ".png"
	_, err := owm.RetrieveIcon(d, icon)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	b, err := os.ReadFile(path.Join(d, icon))
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return base.MakeByteOutput(b)
}
