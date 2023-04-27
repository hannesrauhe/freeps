package freepsgraph

import (
	"io/ioutil"
	"net/http"
	"path"

	owm "github.com/briandowns/openweathermap"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	log "github.com/sirupsen/logrus"
)

type OpenWeatherMapConfig struct {
	APIKey   string
	Location string
	Units    string
	Lang     string
}

var DefaultOwmConfig = OpenWeatherMapConfig{Units: "c", Lang: "en"}

type OpWeather struct {
	cr   *utils.ConfigReader
	conf OpenWeatherMapConfig
}

func NewWeatherOp(cr *utils.ConfigReader) *OpWeather {
	conf := DefaultOwmConfig
	err := cr.ReadSectionWithDefaults("openweathermap", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	return &OpWeather{cr: cr, conf: conf}
}

var _ base.FreepsOperator = OpWeather{}

// GetName returns the name of the operator
func (o OpWeather) GetName() string {
	return "weather"
}

func (o OpWeather) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	err := utils.ArgsMapToObject(vars, &o.conf)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	switch function {
	case "current":
		wm, err := owm.NewCurrent(o.conf.Units, o.conf.Lang, o.conf.APIKey)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
		wm.CurrentByName(o.conf.Location)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
		if wm.ID == 0 {
			return base.MakeOutputError(http.StatusInternalServerError, "ID of response is 0")
		}
		return base.MakeObjectOutput(wm)
	case "icon":
		d, _ := utils.GetTempDir()
		icon := path.Base(vars["icon"])
		if len(icon) <= 1 {
			return base.MakeOutputError(http.StatusBadRequest, "Provide a valid icon name")
		}
		icon = icon + ".png"
		_, err := owm.RetrieveIcon(d, icon)
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
		b, err := ioutil.ReadFile(path.Join(d, icon))
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
		return base.MakeByteOutput(b)
	}
	//
	return base.MakeOutputError(http.StatusNotFound, "Function %v not found", function)
}

func (o OpWeather) GetFunctions() []string {
	return []string{"current", "icon"}
}

func (o OpWeather) GetPossibleArgs(fn string) []string {
	return []string{"location", "units", "lang"}
}

func (o OpWeather) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o OpWeather) Shutdown(ctx *base.Context) {
}
