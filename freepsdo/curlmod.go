package freepsdo

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
)

type CurlMod struct {
}

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
	case "Get":
		resp, err = c.Get(vars["url"])
	default:
		jrw.WriteError(http.StatusNotFound, "function %v unknown", function)
		return
	}

	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "%v", string(err.Error()))
		return
	}
	defer resp.Body.Close()
	b, err := io.ReadAll(resp.Body)
	jrw.WriteResponseWithCodeAndType(resp.StatusCode, "text/plain", string(b))
	log.Printf("%v , %v", err, string(b))
}

var _ Mod = &CurlMod{}
