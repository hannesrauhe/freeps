package freepsgraph

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
)

type OpCurl struct {
}

var _ FreepsOperator = &OpCurl{}

func (o *OpCurl) Execute(function string, vars map[string]string, mainInput *OperatorIO) *OperatorIO {
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
				return MakeOutputError(http.StatusBadRequest, err.Error())
			}
		}
		breader := bytes.NewReader(b)
		resp, err = c.Post(vars["url"], vars["content-type"], breader)
	case "Get":
		resp, err = c.Get(vars["url"])
	default:
		return MakeOutputError(http.StatusNotFound, "function %v unknown", function)
	}

	if err != nil {
		return MakeOutputError(http.StatusInternalServerError, "%v", err.Error())
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	return &OperatorIO{HTTPCode: uint32(resp.StatusCode), Output: b, OutputType: Byte}
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
