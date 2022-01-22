package restonatorx

import (
	"net/http"

	"github.com/gorilla/mux"
)

type RestonatorMod interface {
	Do(function string, args map[string][]string, w http.ResponseWriter)
}

type Restonator struct {
	Mods map[string]RestonatorMod
}

func (r *Restonator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	mod, exists := r.Mods[vars["mod"]]
	if !exists {
		w.WriteHeader(http.StatusNotFound)
		return
	}
	args := req.URL.Query()
	device, exists := vars["device"]
	if exists {
		args["device"] = make([]string, 1)
		args["device"][0] = device
	}
	mod.Do(vars["function"], args, w)
}
