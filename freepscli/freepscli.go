package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"

	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/utils"
)

func main() {
	var configpath, fn, dev string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&fn, "f", "freepsflux", "Specify function")
	flag.StringVar(&dev, "d", "", "Specify device")
	verb := flag.Bool("v", false, "Verbose output")

	flag.Parse()

	conf := freepslib.DefaultConfig
	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.ReadSectionWithDefaults("freepslib", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	f, err := freepslib.NewFreepsLib(&conf)
	f.Verbose = *verb

	var jsonbytes []byte

	switch fn {
	case "freepsflux":
		{
			ff, err2 := freepsflux.NewFreepsFlux(f)
			if err2 != nil {
				log.Fatalf("Error while executing function: %v\n", err2)
			}
			err = ff.Push()
		}
	case "getdevicelistinfos":
		{
			devl, err2 := f.GetDeviceList()
			if err2 != nil {
				log.Fatalf("Error while executing function: %v\n", err2)
			}
			jsonbytes, err = json.MarshalIndent(devl, "", "  ")

		}
	case "gettemplatelistinfos":
		{
			devl, err2 := f.GetTemplateList()
			if err2 != nil {
				log.Fatalf("Error while executing function: %v\n", err2)
			}
			jsonbytes, err = json.MarshalIndent(devl, "", "  ")
		}
	case "getdata":
		{
			devl, err2 := f.GetData()
			if err2 != nil {
				log.Fatalf("Error while executing function: %v\n", err2)
			}
			jsonbytes, err = json.MarshalIndent(devl, "", "  ")
		}
	case "metrics":
		{
			metrics, err := f.GetMetrics()
			if err != nil {
				log.Fatalf("could not load UPnP service: %v", err)
			}
			fmt.Printf("%v\n", metrics)
		}
	default:
		{
			arg := make(map[string]string)
			result, err2 := f.HomeAutomation(fn, dev, arg)
			if err2 != nil {
				log.Fatalf("Error while executing function: %v\n", err2)
			}
			jsonbytes, err = json.MarshalIndent(result, "", "  ")
		}
	}
	if err != nil {
		log.Fatalf("Error while parsing response: %v\n", err)
	}
	var b bytes.Buffer
	b.Write(jsonbytes)
	fmt.Println(b.String())
}
