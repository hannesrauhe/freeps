package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepsflux"
	"github.com/hannesrauhe/freeps/freepsmqtt"
	"github.com/hannesrauhe/freeps/restonatorx"
	"github.com/hannesrauhe/freeps/utils"
)

var verbose bool

func mqtt(cr *utils.ConfigReader) {
	ffc := freepsflux.DefaultConfig
	fmc := freepsmqtt.DefaultConfig
	err := cr.ReadSectionWithDefaults("freepsflux", &ffc)
	if err != nil {
		log.Fatal(err)
	}
	err = cr.ReadSectionWithDefaults("freepsmqtt", &fmc)
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
	fm := freepsmqtt.FreepsMqtt{&fmc, ff.PushFields}
	fm.Start()
}

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

	mods := make(map[string]restonatorx.RestonatorMod)
	mods["curl"] = &restonatorx.CurlMod{}
	mods["fritz"] = restonatorx.NewFritzMod(cr)
	mods["raspistill"] = &restonatorx.RaspistillMod{}
	modinator := restonatorx.NewTemplateModFromUrl("https://raw.githubusercontent.com/hannesrauhe/freeps/freepsd/restonatorx/templates.json", mods)
	mods["template"] = modinator

	if mod != "" {
		modinator.ExecuteMod(mod, fn, argstring)
		return
	}

	rest := &restonatorx.Restonator{Mods: mods}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	r := mux.NewRouter()
	r.Handle("/{mod}/{function}", rest)
	r.Handle("/{mod}/{function}/{device}", rest)
	r.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Shutdown Request Sucess"))
		// Cancel the context on request
		cancel()
	})

	srv := &http.Server{
		Handler:      r,
		Addr:         ":8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Starting Server")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()

	mqtt(cr)

	select {
	case <-ctx.Done():
		// Shutdown the server when the context is canceled
		srv.Shutdown(ctx)
	}
	log.Printf("Server stopped")
}
