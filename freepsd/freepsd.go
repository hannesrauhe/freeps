package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/freepslisten"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool

func main() {
	var configpath, fn, mod, argstring string
	flag.StringVar(&configpath, "c", utils.GetDefaultPath("freeps"), "Specify config file to use")
	flag.StringVar(&mod, "m", "", "Specify mod to execute directly without starting rest server")
	flag.StringVar(&fn, "f", "", "Specify function to execute in mod")
	flag.StringVar(&argstring, "a", "", "Specify arguments to function as urlencoded string")
	flag.BoolVar(&verbose, "v", false, "Verbose output")

	flag.Parse()

	cr, err := utils.NewConfigReader(configpath)
	if err != nil {
		log.Fatal(err)
	}

	doer := freepsdo.NewTemplateMod(cr)

	if mod != "" {
		jrw := freepsdo.NewResponseCollector("freepsd command")
		args, _ := url.ParseQuery(argstring)
		doer.ExecuteModWithJson(mod, fn, utils.URLArgsToJSON(args), jrw)
		_, t, b := jrw.GetFinalResponse(true)
		if t == freepsdo.ResponseTypePlainText || t == freepsdo.ResponseTypeJSON {
			os.Stdout.Write(b)
			println("")
		} else {
			println("Binary response not printed")
		}
		if verbose {
			fmt.Printf("%q", jrw.GetResponseTree())
		}
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rest := freepslisten.NewRestEndpoint(cr, doer, cancel)
	mqtt := freepslisten.NewMqttSubscriber(cr, doer)
	telg := freepslisten.NewTelegramBot(cr, doer, cancel)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		rest.Shutdown(ctx)
		mqtt.Shutdown()
		if telg != nil {
			telg.Shutdown(ctx)
		}
	}
	log.Printf("Server stopped")
}
