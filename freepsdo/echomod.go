package freepsdo

type EchoMod struct {
}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, jrw *JsonResponse) {
	jrw.WriteSuccessString("Function: %v\nArgs: %q\n", fn, jsonStr)
}

var _ Mod = &EchoMod{}
