package freeps

import (
	"fmt"
	"log"
	"os"

	lib "github.com/hannesrauhe/freeps/freepslib"
)

func NewFreeps(configpath string) (*lib.Freeps, error) {
	conf, err := lib.ReadFreepsConfig(configpath)
	if os.IsNotExist(err) {
		err = lib.WriteFreepsConfig(configpath, nil)
		if err != nil {
			log.Print("Failed to create default config file")
			return nil, err
		}
		err = fmt.Errorf("created default config at %v, please set values", configpath)
		return nil, err
	}
	if err != nil {
		log.Print("Failed to read config file")
		return nil, err
	}
	return lib.NewFreepsLib(conf)
}
