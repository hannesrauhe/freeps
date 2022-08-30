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
		breader := bytes.NewReader([]byte(vars["body"]))
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
