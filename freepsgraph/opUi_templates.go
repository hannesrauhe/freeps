package freepsgraph

const templateEditGraph = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<form action="#" method="POST">
<label for="NumOps">Number of operations:</label>
<input type="number" id="NumOps" name="NumOps" min="1" max="50">
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

<p>
	FwdTemplateName:
		{{ range $key, $value := .Templates }}
			{{ if $value}}
				<button name="FwdTemplateName" value="{{ $key }}" disabled="true" >{{ $key }}</button>
			{{ else}}
				<button name="FwdTemplateName" value="{{ $key }}">{{ $key }}</button>
			{{ end }}
		{{ end }}
</p>

<textarea name="TemplateJSON" cols="50" rows="10">
{{ .TemplateJSON }}
</textarea>
</p>

<button type="submit" name="Execute" value="Execute">Execute</button>
<input type="text" name="TemplateName" />
<button type="submit" name="SaveTemplate">Save Template</button>
<button type="submit" name="TemplateJSON" value="" />Reset</button>
</form>

{{ if .Output }}
<div style="background-color: lightgoldenrodyellow;">
<pre><code>
{{ .Output }}
</code></pre>
</div>
{{ end }}
`

const templateShowGraphs = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<div style="float:left">
<ul>
		{{ range $key, $value := .Graphs }}
				<li>{{ $value }} <a href="#?graph={{ $value }}">Show</a> <a href="/ui/edit?graph={{ $value }}">Edit</a> </li>
		{{ end }}
</ul>
</div>
<div>
<textarea name="dot" cols="50" rows="10">
{{ .Dot }}
</textarea>
<a href="https://dreampuf.github.io/GraphvizOnline/#{{ .Dot | urlescaper }}">Show Online</a>
</div>
`
