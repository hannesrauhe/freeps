package freeps

import (
	"log"

	"github.com/hannesrauhe/freeps/freepslib"
	lib "github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

func NewFreeps(configpath string) (*lib.Freeps, error) {
	conf := freepslib.DefaultConfig
	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Print("Failed to open config file")
		return nil, err
	}

	err = cr.ReadSectionWithDefaults("freepslib", &conf)
	if err != nil {
		log.Print("Failed to read section of config file")
		return nil, err
	}

	err = cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print("Failed to write config file")
		return nil, err
	}

	return freepslib.NewFreepsLib(&conf)
}
