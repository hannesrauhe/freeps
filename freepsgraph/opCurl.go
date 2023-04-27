package freepsgraph

import (
	"bytes"
	"io"
	"mime"
	"net/http"
	"net/url"
	"os"
	"path"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type OpCurl struct {
}

var _ base.FreepsOperator = &OpCurl{}

// GetName returns the name of the operator
func (o *OpCurl) GetName() string {
	return "curl"
}

func (o *OpCurl) Execute(ctx *base.Context, function string, vars map[string]string, mainInput *base.OperatorIO) *base.OperatorIO {
	c := http.Client{}

	var resp *http.Response
	var err error
	switch function {
	case "PostForm":
		data := url.Values{}
		for k, v := range vars {
			if k == "url" {
				continue
			}
			data.Set(k, v)
		}
		resp, err = c.PostForm(vars["url"], data)
	case "Post":
		var b []byte
		if vars["body"] != "" {
			b = []byte(vars["body"])
		} else {
			b, err = mainInput.GetBytes()
			if err != nil {
				return base.MakeOutputError(http.StatusBadRequest, err.Error())
			}
		}
		breader := bytes.NewReader(b)
		resp, err = c.Post(vars["url"], vars["content-type"], breader)
	case "Get":
		resp, err = c.Get(vars["url"])
	default:
		return base.MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	defer resp.Body.Close()

	outputFile, WriteToFile := vars["file"]
	if !WriteToFile {
		b, err := io.ReadAll(resp.Body)
		if err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
		}
		return &base.OperatorIO{HTTPCode: resp.StatusCode, Output: b, OutputType: base.Byte, ContentType: resp.Header.Get("Content-Type")}
	}
	// sanitize outputFile:
	outputFile = path.Base(outputFile)
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

func (o *OpCurl) GetFunctions() []string {
	return []string{"PostForm", "Post", "Get"}
}

func (o *OpCurl) GetPossibleArgs(fn string) []string {
	return []string{"url", "body", "content-type"}
}

func (o *OpCurl) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpCurl) Shutdown(ctx *base.Context) {
}
