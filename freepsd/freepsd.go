package main

import (
	"context"
	"flag"
	"log"
	"net/url"
	"os"

	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/freepsgraph"
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

	running := true
	for running {
		cr, err := utils.NewConfigReader(configpath)
		if err != nil {
			log.Fatal(err)
		}

		log.Printf("Loading graph engine")
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()

		doer := freepsdo.NewTemplateMod(cr)
		ge := freepsgraph.NewGraphEngine(cr, cancel)

		if mod != "" {
			args, _ := url.ParseQuery(argstring)
			output := ge.ExecuteOperatorByName(mod, fn, utils.URLArgsToMap(args), freepsgraph.MakeEmptyOutput())
			output.WriteTo(os.Stdout)
			return
		}

		log.Printf("Starting Listeners")
		http := freepslisten.NewFreepsHttp(cr, ge)

		rest := freepslisten.NewRestEndpoint(cr, doer, cancel)
		mqtt := freepslisten.NewMqttSubscriber(cr, ge)
		telg := freepslisten.NewTelegramBot(cr, ge, cancel)

		select {
		case <-ctx.Done():
			// Shutdown the server when the context is canceled
			rest.Shutdown(ctx)
			if mqtt != nil {
				mqtt.Shutdown()
			}
			if telg != nil {
				telg.Shutdown(ctx)
			}
			http.Shutdown(ctx)
		}
		running = ge.ReloadRequested()
		log.Printf("Stopping Listeners")
	}
}
