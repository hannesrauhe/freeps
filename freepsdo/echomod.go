package freepsdo

type EchoMod struct {
}

func (m *EchoMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	jrw.WriteSuccessf("Function: %v\nArgs: %q\n", fn, jsonStr)
}

var _ Mod = &EchoMod{}
