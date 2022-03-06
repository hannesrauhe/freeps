package freepslisten

import (
	"encoding/json"
	"html/template"
	"log"
	"net/http"
	"net/url"

	"github.com/hannesrauhe/freeps/freepsdo"
)

const templateString = `<form action="#" method="Get">
<p>
	Mod:
		{{ range $key, $value := .ModSuggestions }}
			{{ if $value}}
				<button name="mod" value="{{ $key }}" disabled="true" >{{ $key }}</button>
			{{ else}}
				<button name="mod" value="{{ $key }}">{{ $key }}</button>
			{{ end }}
		{{ end }}
</p>
<p>
	Function:
		{{ range $key, $value := .FnSuggestions }}
			{{ if $value}}
				<button name="fn" value="{{ $key }}" disabled="true" >{{ $key }}</button>
			{{ else}}
				<button name="fn" value="{{ $key }}">{{ $key }}</button>
			{{ end }}
		{{ end }}
</p>

{{ range $arg, $argmap := .ArgSuggestions }}
<p>
	{{ $arg }}
	{{ range $key, $value := $argmap }}
	<button name="arg.{{ $arg }}" value="{{ $value }}">{{ $key }}</button>
	{{ end }}
</p>
{{ end }}

<p>
<input type="text" name="newarg" /> <input type="text" name="newvalue" /><button name="addarg">Add Arg</button>
<p>
<textarea name="TemplateJSON" cols="50" rows="10">
{{ .TemplateJSON }}
</textarea>
</p>

<button type="submit" name="Execute" value="Execute">Execute</button>
<input type="text" name="TemplateName" />
<button type="submit" name="SaveTemplate">Save Template</button>
<button type="reset"  value="Reset" />Reset</button>
</form>

{{ if .Output }}
<p>
<textarea cols="50" rows="10" readonly="true">
{{ .Output }}
</textarea>
</p>
{{ end }}
`

type HTMLUI struct {
	modinator *freepsdo.TemplateMod
	tmpl      *template.Template
}

type TemplateData struct {
	Args           map[string]string
	ModSuggestions map[string]bool
	FnSuggestions  map[string]bool
	ArgSuggestions map[string]map[string]string
	TemplateJSON   string
	Output         string
}

// NewHTMLUI creates a UI interface based on the inline template above
func NewHTMLUI(modinator *freepsdo.TemplateMod) *HTMLUI {
	t := template.New("general")
	t, _ = t.Parse(templateString)
	h := HTMLUI{tmpl: t, modinator: modinator}

	return &h
}

func (r *HTMLUI) buildPartialTemplate(vars url.Values) *freepsdo.TemplateAction {
	ta := &freepsdo.TemplateAction{Mod: "echo", Fn: "hello", Args: map[string]interface{}{}}
	if vars == nil {
		return ta
	}
	if vars.Has("TemplateJSON") {
		json.Unmarshal([]byte(vars["TemplateJSON"][0]), ta)
	}
	for k, vArr := range vars {
		v := vArr[len(vArr)-1]
		if len(k) > 4 && k[0:4] == "arg." {
			ta.Args[k[4:]] = v
		}
		if k == "mod" {
			if _, ok := r.modinator.Mods[v]; ok {
				ta.Mod = v
			}
		}
		if k == "fn" {
			ta.Fn = v
		}
	}
	if vars.Get("newarg") != "" {
		ta.Args[vars.Get("newarg")] = vars.Get("newvalue")
	}

	return ta
}

func (r *HTMLUI) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	vars := req.URL.Query()
	ta := r.buildPartialTemplate(vars)
	td := &TemplateData{ModSuggestions: map[string]bool{}, FnSuggestions: map[string]bool{}, ArgSuggestions: make(map[string]map[string]string)}
	b, _ := json.MarshalIndent(ta, "", "  ")
	td.TemplateJSON = string(b)

	for k := range r.modinator.Mods {
		td.ModSuggestions[k] = (k == ta.Mod)
	}

	mod := r.modinator.Mods[ta.Mod]
	for _, k := range mod.GetFunctions() {
		td.FnSuggestions[k] = (k == ta.Fn)
	}

	for _, k := range mod.GetPossibleArgs(ta.Fn) {
		td.ArgSuggestions[k] = mod.GetArgSuggestions(ta.Fn, k, ta.Args)
	}

	if vars.Has("Execute") {
		jrw := freepsdo.NewResponseCollector()
		r.modinator.ExecuteTemplateAction(ta, jrw)
		_, _, bytes := jrw.GetFinalResponse()
		td.Output = string(bytes)
	}

	if vars.Has("SaveTemplate") {
		//TODO: use systemmod instead
		r.modinator.SaveTemplateAction(vars.Get("TemplateName"), ta)
		td.Output = "Saved " + vars.Get("TemplateName")
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	err := r.tmpl.Execute(w, td)
	if err != nil {
		log.Println(err)
	}
}
