package freepslisten

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type FreepsHttp struct {
	graphengine *freepsgraph.GraphEngine
	srv         *http.Server
}

func (r *FreepsHttp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	log.Printf("rest API: %v", req.RemoteAddr)
	var mainArgs map[string]string
	var mainInput freepsgraph.OperatorIO

	if req.Method == "POST" {
		defer req.Body.Close()
		byteargs, err := io.ReadAll(req.Body)
		mainInput = *freepsgraph.MakeByteOutput(byteargs)
		if err != nil {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprintf(w, "Error reading request body: %v", err)
			return
		}
	}

	query := req.URL.Query()
	device, exists := vars["device"]
	if exists {
		query["device"] = make([]string, 1)
		query["device"][0] = device
	}
	mainArgs = utils.URLArgsToMap(query)

	opio := r.graphengine.ExecuteOperatorByName(vars["mod"], vars["function"], mainArgs, &mainInput)

	bytes, err := opio.GetBytes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating response: %v", err)
	}
	w.Header().Set("Content-Type", string(opio.OutputType))
	w.Header().Set("Content-Length", strconv.Itoa(len(bytes)))
	w.WriteHeader(int(opio.HTTPCode))
	if _, err := w.Write(bytes); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "unable to write bytes to response: %v", err.Error())
	}
}

func (r *FreepsHttp) Shutdown(ctx context.Context) {
	r.srv.Shutdown(ctx)
}

func NewFreepsHttp(cr *utils.ConfigReader, ge *freepsgraph.GraphEngine, cancel context.CancelFunc) *FreepsHttp {
	rest := &FreepsHttp{graphengine: ge}
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
		Addr:         ":8080",
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
