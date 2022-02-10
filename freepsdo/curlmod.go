package freepsdo

import (
	"encoding/json"
	"net/http"
	"net/url"
)

type CurlMod struct {
}

func (m *CurlMod) DoWithJSON(function string, jsonStr []byte, jrw *JsonResponse) {
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
		jrw.WriteError(http.StatusInternalServerError, "CurlMod\nFunction: %v\nArgs: %v\nError: %v", function, vars, string(err.Error()))
		return
	}
	jrw.WriteError(resp.StatusCode, "CurlMod: %v, %v", vars, resp)
}

var _ Mod = &CurlMod{}
