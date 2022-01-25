package freepsdo

import (
	"net/http"
)

type RestonatorMod interface {
	Do(function string, args map[string][]string, w http.ResponseWriter)
}
