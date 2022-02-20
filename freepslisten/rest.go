package freepslisten

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type Restonator struct {
	Modinator *freepsdo.TemplateMod
	srv       *http.Server
}

func (r *Restonator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	args := req.URL.Query()
	device, exists := vars["device"]
	if exists {
		args["device"] = make([]string, 1)
		args["device"][0] = device
	}
	jrw := freepsdo.NewResponseCollector()
	r.Modinator.ExecuteModWithJson(vars["mod"], vars["function"], utils.URLArgsToJSON(args), jrw)
	status, otype, bytes := jrw.GetFinalResponse()
	w.Header().Set("Content-Type", otype)
	w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	if _, err := w.Write(bytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unable to write bytes to response: %v", err.Error())
	}
	w.WriteHeader(status)
}

func (r *Restonator) Shutdown(ctx context.Context) {
	r.srv.Shutdown(ctx)
}

func NewRestEndpoint(cr *utils.ConfigReader, doer *freepsdo.TemplateMod, cancel context.CancelFunc) *Restonator {
	rest := &Restonator{Modinator: doer}
	r := mux.NewRouter()
	r.Handle("/{mod}/{function}", rest)
	r.Handle("/{mod}/{function}/{device}", rest)
	r.HandleFunc("/shutdown", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Shutdown Request Sucess"))
		// Cancel the context on request
		cancel()
	})

	rest.srv = &http.Server{
		Handler:      r,
		Addr:         ":8000",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Starting Server")
		if err := rest.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	return rest
}
