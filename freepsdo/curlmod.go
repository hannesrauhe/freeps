package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
)

type CurlMod struct {
}

func (m *CurlMod) DoWithJSON(function string, jsonStr []byte, w http.ResponseWriter) {
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
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "function %v unknown", function)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "CurlMod\nFunction: %v\nArgs: %v\nError: %v", function, vars, string(err.Error()))
		return
	}
	w.WriteHeader(resp.StatusCode)
	fmt.Fprintf(w, "CurlMod: %v, %v", vars, resp)
}

var _ Mod = &CurlMod{}
