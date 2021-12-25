package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

func readFreepsConfig(configpath string) (freepslib.FBconfig, error) {
	conf := freepslib.DefaultConfig

	byteValue, err := ioutil.ReadFile(configpath)
	if err != nil {
		return conf, err
	}

	newbytes, err := utils.ReadConfigWithDefaults(byteValue, "freepslib", &conf)
	if err != nil {
		return conf, err
	}
	if len(newbytes) > 0 {
		ioutil.WriteFile(configpath, newbytes, 0644)
	}

	return conf, err
}

func main() {
	dir, _ := os.UserConfigDir()
	var configpath, fn, dev string
	flag.StringVar(&configpath, "c", dir+"/freeps.conf", "Specify config file to use")
	flag.StringVar(&fn, "f", "getdevicelistinfos", "Specify function")
	flag.StringVar(&dev, "d", "", "Specify device")
	verb := flag.Bool("v", false, "Verbose output")

	flag.Parse() // after declaring flags we need to call it

	conf, err := readFreepsConfig(configpath)
	if err != nil {
		fmt.Printf("Couldn't initialize config: %v\n", err)
		return
	}
	f, err := freepslib.NewFreepsLib(&conf)
	f.Verbose = *verb

	var jsonbytes []byte

	if fn == "getdevicelistinfos" {
		devl, err2 := f.GetDeviceList()
		if err2 != nil {
			fmt.Printf("Error while executing function: %v\n", err2)
			return
		}
		jsonbytes, err = json.MarshalIndent(devl, "", "  ")
	} else if fn == "gettemplatelistinfos" {
		devl, err2 := f.GetTemplateList()
		if err2 != nil {
			fmt.Printf("Error while executing function: %v\n", err2)
			return
		}
		jsonbytes, err = json.MarshalIndent(devl, "", "  ")
	} else {
		arg := make(map[string]string)
		result, err2 := f.HomeAutomation(fn, dev, arg)
		if err2 != nil {
			fmt.Printf("Error while executing function: %v\n", err2)
			return
		}
		jsonbytes, err = json.MarshalIndent(result, "", "  ")
	}
	if err != nil {
		fmt.Printf("Error while parsing response: %v\n", err)
		return
	}
	var b bytes.Buffer
	b.Write(jsonbytes)
	fmt.Println(b.String())
}
