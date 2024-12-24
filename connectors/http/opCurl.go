package freepshttp

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"os"
	"path"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

type OpCurl struct {
	CR       *utils.ConfigReader
	GE       *freepsflow.FlowEngine
	Config   HTTPConfig
	listener *FreepsHttpListener
}

var _ base.FreepsOperatorWithShutdown = &OpCurl{}
var _ base.FreepsOperatorWithConfig = &OpCurl{}

// GetDefaultConfig returns the default config for the http connector
func (o *OpCurl) GetDefaultConfig() interface{} {
	return &HTTPConfig{
		Port:                  8080,
		EnablePprof:           false,
		FlowProcessingTimeout: 120,
	}
}

// InitCopyOfOperator creates a copy of the operator
func (o *OpCurl) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	cfg := config.(*HTTPConfig)
	return &OpCurl{CR: o.CR, GE: o.GE, Config: *cfg}, nil
}

// CurlArgs are the common arguments for all curl functions
type CurlArgs struct {
	URL         string  `json:"url"`
	Body        *string `json:"body"`
	ContentType *string `json:"content-type"`
	OutputFile  *string `json:"file"`
}

// PostForm executes a POST request to the given URL with the given form fields and returns either the response body or information about the downloaded file if an output file is specified
func (o *OpCurl) PostForm(ctx *base.Context, mainInput *base.OperatorIO, args CurlArgs, formFields base.FunctionArguments) *base.OperatorIO {
	c := http.Client{}
	resp, err := c.PostForm(args.URL, formFields.GetOriginalCaseMap())
	return o.handleResponse(resp, err, ctx, args)
}

// Post executes a POST request to the given URL and returns either the response body or information about the downloaded file if an output file is specified
func (o *OpCurl) Post(ctx *base.Context, mainInput *base.OperatorIO, args CurlArgs) *base.OperatorIO {
	c := http.Client{}

	var b []byte
	if args.Body != nil {
		b = []byte(*args.Body)
	} else {
		var err error
		b, err = mainInput.GetBytes()
		if err != nil {
			return base.MakeOutputError(http.StatusBadRequest, err.Error())
		}
	}
	breader := bytes.NewReader(b)
	contentType := "application/octet-stream"
	if args.ContentType != nil {
		contentType = *args.ContentType
	}
	resp, err := c.Post(args.URL, contentType, breader)
	return o.handleResponse(resp, err, ctx, args)
}

// Get executes a GET request to the given URL and returns either the response body or information about the downloaded file if an output file is specified
func (o *OpCurl) Get(ctx *base.Context, mainInput *base.OperatorIO, args CurlArgs) *base.OperatorIO {
	c := http.Client{}
	resp, err := c.Get(args.URL)
	return o.handleResponse(resp, err, ctx, args)
}

func (o *OpCurl) handleResponse(resp *http.Response, err error, ctx *base.Context, args CurlArgs) *base.OperatorIO {
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	defer resp.Body.Close()

	if args.OutputFile == nil {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
		}
		return &base.OperatorIO{HTTPCode: resp.StatusCode, Output: b, OutputType: base.Byte, ContentType: resp.Header.Get("Content-Type")}
	}
	// sanitize outputFile:
	outputFile := path.Base(*args.OutputFile)
	dir, err := utils.GetTempDir()
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	var dstFile *os.File
	if outputFile == "" || outputFile == "/" || outputFile == "." {
		extensions, _ := mime.ExtensionsByType(resp.Header.Get("Content-Type"))
		ext := ""
		if len(extensions) > 0 {
			ext = extensions[0]
		}
		dstFile, err = os.CreateTemp(dir, "freeps-opcurl*"+ext)
	} else {
		dstFile, err = os.OpenFile(path.Join(dir, outputFile), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0644)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	r := map[string]interface{}{}
	r["size"], err = io.Copy(dstFile, resp.Body)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	r["name"] = path.Base(dstFile.Name())
	return base.MakeObjectOutput(r)
}

// StartListening starts the http server
func (o *OpCurl) StartListening(ctx *base.Context) {
	o.listener = NewFreepsHttp(ctx, o.Config, o.GE)
}

// Shutdown shuts down the http server
func (o *OpCurl) Shutdown(ctx *base.Context) {
	if o.listener == nil {
		return
	}
	o.listener.Shutdown(ctx.GoContext)
}
