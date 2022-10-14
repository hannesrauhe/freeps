package freepsgraph

const templateFooter = `
<footer class="footer">
		<a href="/ui">Home</a> <a href="/ui/edit">New Graph</a> <a href="/ui/config">Edit Config</a>
		<a href="/system/reload">Reload Freeps</a> <a href="/system/stop">Stop Freeps</a>
</footer>
`

const templateEditGraph = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<form action="#" method="POST">
<p>
<label for="numop">Number of operation:</label>
<input type="number" id="numop" name="numop" min="0" max="50" value="{{ .Numop }}" />
<button name="goop">GO</button>
</p>
<p>
	Mod:
		{{ range $key, $value := .OpSuggestions }}
			{{ if $value}}
				<button name="op" value="{{ $key }}" disabled="true" >{{ $key }}</button>
			{{ else}}
				<button name="op" value="{{ $key }}">{{ $key }}</button>
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
	InputFrom:
		{{ range $key, $value := .InputFromSuggestions }}
			{{ if $value}}
				<button name="inputFrom" value="{{ $key }}" disabled="true" >{{ $key }}</button>
			{{ else}}
				<button name="inputFrom" value="{{ $key }}">{{ $key }}</button>
			{{ end }}
		{{ end }}
</p>

<textarea name="GraphJSON" cols="200" rows="50">
{{ .GraphJSON }}
</textarea>
</p>

<button type="submit" name="Execute" value="Execute">Execute</button>
<input type="text" name="GraphName" value="{{ .GraphName }}"/>
<button type="submit" name="SaveTemp">Save Temporarily</button>
<button type="submit" name="SaveGraph">Save Graph</button>
<button type="submit" name="GraphJSON" value="" />Reset</button>
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
				<li>{{ $value }} <a href="/ui/show?graph={{ $value }}">Show</a> <a href="/graph/{{ $value }}">Execute</a> <a href="/ui/edit?graph={{ $value }}">Edit</a> </li>
		{{ end }}
</ul>
</div>
<div>
<textarea readonly=true name="GraphJSON" cols="50" rows="10">
{{ .GraphJSON }}
</textarea>
</div>
`

const templateEditConfig = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<form action="#" method="POST">
<p>
<textarea name="ConfigText" cols="200" rows="50">
{{ .ConfigText }}
</textarea>
</p>

<button type="submit" name="SaveConfig">Save Config</button>
</form>
`
