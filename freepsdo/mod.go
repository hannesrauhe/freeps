package freepsdo

import (
	"net/http"
)

type Mod interface {
	DoWithJSON(fn string, jsonStr []byte, w http.ResponseWriter)
}
