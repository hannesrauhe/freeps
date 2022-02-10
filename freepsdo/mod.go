package freepsdo

type Mod interface {
	DoWithJSON(fn string, jsonStr []byte, jrw *JsonResponse)
}
