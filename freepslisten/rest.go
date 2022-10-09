package freepslisten

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type Restonator struct {
	Modinator *freepsdo.TemplateMod
	srv       *http.Server
	ui        *HTMLUI
}

func (r *Restonator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	jrw := freepsdo.NewResponseCollector(fmt.Sprintf("rest API: %v", req.RemoteAddr))
	var byteargs []byte
	var err error

	if req.Method == "POST" {
		defer req.Body.Close()
		byteargs, err = io.ReadAll(req.Body)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error reading request body: %v", err)
			return
		}
	} else {
		args := req.URL.Query()
		device, exists := vars["device"]
		if exists {
			args["device"] = make([]string, 1)
			args["device"][0] = device
		}
		byteargs = utils.URLArgsToJSON(args)
	}
	r.Modinator.ExecuteModWithJson(vars["mod"], vars["function"], byteargs, jrw)
	status, otype, bytes := jrw.GetFinalResponse(false)
	ctype, _ := otype.ToString()
	w.Header().Set("Content-Type", ctype)
	w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	w.WriteHeader(status)
	if _, err := w.Write(bytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unable to write bytes to response: %v", err.Error())
	}
}

func (r *Restonator) Shutdown(ctx context.Context) {
	r.srv.Shutdown(ctx)
}

func NewRestEndpoint(cr *utils.ConfigReader, doer *freepsdo.TemplateMod, cancel context.CancelFunc) *Restonator {
	rest := &Restonator{Modinator: doer, ui: NewHTMLUI(doer)}
	r := mux.NewRouter()
	r.Handle("/{mod}/{function}", rest)
	r.Handle("/{mod}/{function}/{device}", rest)
	r.Handle("/ui", rest.ui)
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
