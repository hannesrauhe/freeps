package restonatorx

import (
	"net/http"

	"github.com/gorilla/mux"
)

type RestonatorMod interface {
	Do(function string, args map[string][]string, w http.ResponseWriter)
}

type Restonator struct {
	Modinator *TemplateMod
}

func (r *Restonator) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := mux.Vars(req)

	args := req.URL.Query()
	device, exists := vars["device"]
	if exists {
		args["device"] = make([]string, 1)
		args["device"][0] = device
	}
	r.Modinator.ExecuteModWithArgs(vars["mod"], vars["function"], args, w)
}
