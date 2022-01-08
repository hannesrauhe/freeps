package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepslib"
	"github.com/hannesrauhe/freeps/freepsmqtt"
	"github.com/hannesrauhe/freeps/utils"
)

type ShellyDoorInfo struct {
	State       *bool
	Temperature *float32
	Lux         *int64
	Battery     *int16
	Error       *int64
}

var lastInfo ShellyDoorInfo

var verbose bool

func onMessageReceived(client MQTT.Client, message MQTT.Message) {
	t := strings.Split(message.Topic(), "/")
	fmt.Printf("Received message on topic: %s\nMessage: %s\n", t[len(t)-1], message.Payload())
}

func mqtt(cr *utils.ConfigReader) {
	hostname, _ := os.Hostname()
	clientid := hostname + strconv.Itoa(time.Now().Second())

	ffc := freepsflux.DefaultConfig
	fmc := freepsmqtt.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.ReadSectionWithDefaults("freepsmqtt", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	ff, err2 := freepsflux.NewFreepsFlux(&ffc, nil)
	ff.Verbose = verbose
	if err2 != nil {
		log.Fatalf("Error while executing function: %v\n", err2)
	}

	connOpts := MQTT.NewClientOptions().AddBroker(*server).SetClientID(*clientid).SetCleanSession(true)
	if *username != "" {
		connOpts.SetUsername(*username)
		if *password != "" {
			connOpts.SetPassword(*password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnect = func(c MQTT.Client) {
		if token := c.Subscribe(*topic, byte(*qos), onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		}
	}

	client := MQTT.NewClient(connOpts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", *server)
	}
}

func main() {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	var configpath, fn, dev string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&fn, "f", "freepsflux", "Specify function")
	flag.StringVar(&dev, "d", "", "Specify device")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Parse()

	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}

	if fn == "mqtt" {
		mqtt(cr)
	} else {
		conf := freepslib.DefaultConfig
		err = cr.ReadSectionWithDefaults("freepslib", &conf)
		if err != nil {
			log.Fatal(err)
		}
		cr.WriteBackConfigIfChanged()
		if err != nil {
			log.Print(err)
		}

		f, err := freepslib.NewFreepsLib(&conf)
		f.Verbose = verbose

		var jsonbytes []byte

		switch fn {
		case "freepsflux":
			{
				ffc := freepsflux.DefaultConfig
				err = cr.ReadSectionWithDefaults("freepsflux", &ffc)
				if err != nil {
					log.Fatal(err)
				}
				cr.WriteBackConfigIfChanged()
				if err != nil {
					log.Print(err)
				}

				ff, err2 := freepsflux.NewFreepsFlux(&ffc, f)
				if err2 != nil {
					log.Fatalf("Error while executing function: %v\n", err2)
				}
				ff.Verbose = f.Verbose
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

	<-c
}
