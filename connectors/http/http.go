package freepshttp

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/http/pprof"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"

	"github.com/gorilla/mux"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	// "net/http/pprof"
)

//go:embed static_server_content/*
var staticContent embed.FS

type FreepsHttpListener struct {
	flowengine  *freepsflow.FlowEngine
	srv         *http.Server
	baseContext *base.Context
}

func (r *FreepsHttpListener) ParseRequest(req *http.Request) (mainArgs base.FunctionArguments, mainInput *base.OperatorIO, err error) {
	mainInput = base.MakeEmptyOutput()
	mainArgs = base.NewFunctionArgumentsFromURLValues(req.URL.Query())
	var byteinput []byte

	// a simple get request, no input
	if req.Method != "POST" {
		return
	}
	ct := req.Header.Get("Content-Type")
	// a form containing a file
	if strings.Split(ct, ";")[0] == "multipart/form-data" {
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
			if err != nil {
				return
			}
			mainInput = base.MakeByteOutputWithContentType(byteinput, http.DetectContentType(byteinput))
			return
		}
		return
	}

	req.ParseForm() // does nothing if not the correct content type

	// a regular curl call or something alike
	if req.PostForm == nil || len(req.PostForm) == 0 {
		defer req.Body.Close()
		byteinput, err = io.ReadAll(req.Body)
		if ct == "" {
			ct = http.DetectContentType(byteinput)
		}
		mainInput = base.MakeByteOutputWithContentType(byteinput, ct)
		return
	}

	// it's an html form without an attached file
	mainInput = base.MakeObjectOutputWithContentType(req.PostForm, "application/x-www-form-urlencoded")

	formData, err := mainInput.ParseFormData()
	if err != nil {
		return
	}
	// if the form data contains a field called input and input-content-type, these will be used as input and the rest will be passed on as arguments
	if formData.Has("input-content-type") && formData.Has("input") {
		mainInput = base.MakeByteOutputWithContentType([]byte(formData.Get("input")), formData.Get("input-content-type"))
		for k, v := range formData {
			if !mainArgs.Has(k) && k != "input" && k != "input-content-type" {
				mainArgs.Append(k, v...)
			}
		}
		// if mainArgs is empty, the form data will be passed as args, this way post requests can be sent to the same url as get requests
	} else if mainArgs.IsEmpty() {
		mainArgs = base.NewFunctionArgumentsFromURLValues(formData)
	}
	return
}

func (r *FreepsHttpListener) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	mainArgs, mainInput, err := r.ParseRequest(req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprintf(w, "Error reading request body: %v", err)
		return
	}

	// allows to redirect if an empty success response was returned
	redirectLocation := mainArgs.Get("redirect")

	ctx := base.CreateContextWithField(r.baseContext, "component", "http", "HTTP request from "+req.RemoteAddr)
	opio := &base.OperatorIO{}
	if vars["mod"] == "flow" {
		opio = r.flowengine.ExecuteFlow(ctx, vars["function"], mainArgs, mainInput)
	} else {
		opio = r.flowengine.ExecuteOperatorByName(ctx, vars["mod"], vars["function"], mainArgs, mainInput)
	}

	w.Header().Set("X-Freeps-ID", ctx.GetID())
	if redirectLocation != "" && opio.IsEmpty() {
		http.Redirect(w, req, redirectLocation, http.StatusFound)
		return
	}
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

func (r *FreepsHttpListener) Shutdown(ctx context.Context) {
	if r.srv == nil {
		return
	}
	r.srv.Shutdown(ctx)
}

func (r *FreepsHttpListener) handleStaticContent(w http.ResponseWriter, req *http.Request) {
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

func NewFreepsHttp(ctx *base.Context, cfg HTTPConfig, ge *freepsflow.FlowEngine) *FreepsHttpListener {
	rest := &FreepsHttpListener{flowengine: ge, baseContext: ctx}
	r := mux.NewRouter()

	r.HandleFunc("/", rest.handleStaticContent)
	if cfg.EnablePprof {
		r.HandleFunc("/debug/pprof/profile", pprof.Profile)
		r.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
		r.HandleFunc("/debug/pprof/trace", pprof.Trace)
		r.HandleFunc("/debug/pprof/{profile}", pprof.Index)
		r.HandleFunc("/debug/pprof/", pprof.Index)
	}
	r.HandleFunc("/{file}", rest.handleStaticContent)
	r.Handle("/{mod}/", rest)
	r.Handle("/{mod}/{function}", rest)
	r.Handle("/{mod}/{function}/", rest)
	r.Handle("/{mod}/{function}/{device}", rest)

	// tHandler := http.TimeoutHandler(r, time.Duration(cfg.FlowProcessingTimeout)*time.Second, "flow proceesing timeout - flow might still be running")
	rest.srv = &http.Server{
		Handler: r,
		Addr:    fmt.Sprintf(":%v", cfg.Port),
	}

	go func() {
		ctx.GetLogger().Info("Starting HTTP Server")
		if err := rest.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
	}()
	return rest
}
