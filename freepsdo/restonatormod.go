package freepsdo

import (
	"net/http"
)

type RestonatorMod interface {
	Do(function string, args map[string][]string, w http.ResponseWriter)
	DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter)
}
