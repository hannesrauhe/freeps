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
	"strings"
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

func (r *FreepsHttp) ParseRequest(req *http.Request) (mainArgs map[string]string, mainInput *freepsgraph.OperatorIO, err error) {
	mainInput = freepsgraph.MakeEmptyOutput()
	query := req.URL.Query()
	mainArgs = utils.URLArgsToMap(query)
	var byteinput []byte

	if req.Method == "POST" {
		if strings.Split(req.Header.Get("Content-Type"), ";")[0] == "multipart/form-data" {
			err = req.ParseMultipartForm(1024 * 1024 * 2)
			if err != nil {
				return
			}
			if len(req.MultipartForm.File) > 1 {
				err = fmt.Errorf("Can only process one file per form, not %v", len(req.MultipartForm.File))
				return
			}
			for n, _ := range req.MultipartForm.File {
				f, _, serr := req.FormFile(n)
				if serr != nil {
					err = serr
					return
				}
				byteinput, err = io.ReadAll(f)
			}
		} else {
			defer req.Body.Close()
			byteinput, err = io.ReadAll(req.Body)
		}
		if err != nil {
			return
		}
		ct := http.DetectContentType(byteinput)
		mainInput = freepsgraph.MakeByteOutputWithContentType(byteinput, ct)
	}
	return
}

func (r *FreepsHttp) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)
	httplogger := log.WithField("restAPI", req.RemoteAddr)

	mainArgs, mainInput, err := r.ParseRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading request body: %v", err)
		return
	}

	// backward compatibility to a very early version
	device, exists := vars["device"]
	if exists {
		mainArgs["device"] = device
	}

	ctx := utils.NewContext(httplogger)
	defer ctx.MarkResponded()
	opio := r.graphengine.ExecuteOperatorByName(ctx, vars["mod"], vars["function"], mainArgs, mainInput)
	opio.Log(httplogger)

	bytes, err := opio.GetBytes()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "Error creating response: %v", err)
	}
	w.Header().Set("X-Freeps-ID", ctx.GetID())
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

	tHandler := http.TimeoutHandler(r, time.Minute, "graph proceesing timeout - graph might still be running")
	rest.srv = &http.Server{
		Handler: tHandler,
		Addr:    ":8080",
	}

	go func() {
		log.Println("Starting HTTP Server")
		if err := rest.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	return rest
}
