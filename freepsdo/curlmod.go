package freepsdo

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
)

type CurlMod struct {
}

var _ Mod = &CurlMod{}

func (m *CurlMod) DoWithJSON(function string, jsonStr []byte, jrw *ResponseCollector) {
	var vars map[string]string
	json.Unmarshal(jsonStr, &vars)

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
		jrw.WriteError(http.StatusNotFound, "function %v unknown", function)
		return
	}

	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "%v", err.Error())
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	jrw.WriteResponseWithCodeAndType(resp.StatusCode, "text/plain", string(b))
}

func (m *CurlMod) GetFunctions() []string {
	return []string{"PostForm", "Get"}
}

func (m *CurlMod) GetPossibleArgs(fn string) []string {
	return []string{"url"}
}

func (m *CurlMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	return map[string]string{}
}
