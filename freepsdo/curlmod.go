package freepsdo

import (
	"fmt"
	"net/http"
	"net/url"
)

type CurlMod struct {
}

func (m *CurlMod) Do(function string, args map[string][]string, w http.ResponseWriter) {
	c := http.Client{}

	var resp *http.Response
	var err error
	switch function {
	case "PostForm":
		data := url.Values{}
		for k, v := range args {
			if k == "url" {
				continue
			}
			data.Set(k, v[0])
		}
		resp, err = c.PostForm(args["url"][0], data)
	default:
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprintf(w, "function %v unknown", function)
		return
	}

	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		fmt.Fprintf(w, "CurlMod\nFunction: %v\nArgs: %v\nError: %v", function, args, string(err.Error()))
		return
	}
	fmt.Fprintf(w, "CurlMod: %v, %v", args, resp)
}

func (m *CurlMod) DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter) {

}
