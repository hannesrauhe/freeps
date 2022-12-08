package freepslisten

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

//go:embed static_server_content/*
var staticContent embed.FS

type FreepsHttp struct {
	graphengine *freepsgraph.GraphEngine
	srv         *http.Server
}

func (r *FreepsHttp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	httplogger := log.WithField("restAPI", req.RemoteAddr)
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
	mainArgs = utils.URLArgsToMap(query)
	device, exists := vars["device"]
	if exists {
		mainArgs["device"] = device
	}

	opio := r.graphengine.ExecuteOperatorByName(utils.NewContext(httplogger), vars["mod"], vars["function"], mainArgs, &mainInput)
	opio.Log(httplogger)

	bytes, err := opio.GetBytes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating response: %v", err)
	}
	w.Header().Set("Content-Type", opio.ContentType)
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

func (r *FreepsHttp) handleStaticContent(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	fc, err := staticContent.ReadFile("static_server_content/" + vars["file"])
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			http.Redirect(w, req, "/ui/", http.StatusFound)
			return
		}
		log.Errorf("Error when reading from embedded file: %v", err.Error())
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Write(fc)
}

func NewFreepsHttp(cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) *FreepsHttp {
	rest := &FreepsHttp{graphengine: ge}
	r := mux.NewRouter()

	r.HandleFunc("/", rest.handleStaticContent)
	r.HandleFunc("/{file}", rest.handleStaticContent)
	r.Handle("/{mod}/", rest)
	r.Handle("/{mod}/{function}", rest)
	r.Handle("/{mod}/{function}/", rest)
	r.Handle("/{mod}/{function}/{device}", rest)

	rest.srv = &http.Server{
		Handler:      r,
		Addr:         ":8080",
		WriteTimeout: 15 * time.Second,
		ReadTimeout:  15 * time.Second,
	}

	go func() {
		log.Println("Starting HTTP Server")
		if err := rest.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	return rest
}
