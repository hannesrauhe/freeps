package freepsdo

import (
	"net/http"
)

type Mod interface {
	Do(function string, args map[string][]string, w http.ResponseWriter)
	DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter)
}
