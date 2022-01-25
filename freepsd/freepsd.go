package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"net/url"

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
		w := utils.StoreWriter{StoredHeader: make(http.Header)}
		args, _ := url.ParseQuery(argstring)
		doer.ExecuteModWithArgs(mod, fn, args, &w)
		w.Print()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	rest := freepslisten.NewRestEndpoint(cr, doer, cancel)
	mqtt := freepslisten.NewMqttSubscriber(cr)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		rest.Shutdown(ctx)
		mqtt.Shutdown()
	}
	log.Printf("Server stopped")
}
