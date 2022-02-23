package freepsdo

type Mod interface {
	DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector)
	GetFunctions() []string
	GetPossibleArgs(fn string) []string
	GetArgSuggestions(fn string, arg string) map[string]string
}
