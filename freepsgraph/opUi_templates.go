package freepsgraph

const templateFooter = `
<iframe name="outputframe" style="min-width: 500px; height:400px; display:flex; margin:0; padding:0; resize:both; overflow:hidden" id="outputframe" src="{{ .Output }}"></iframe>
<footer style="clear: both">
		<a href="/ui">Home</a> <a href="/ui/edit">New Graph</a> <a href="/ui/config">Edit Config</a>
		<a href="/system/reload" target="outputframe">Reload Freeps</a> <a href="/system/stop" target="outputframe">Stop Freeps</a>
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

<p>
	Arguments:
{{ range $arg, $argmap := .ArgSuggestions }}
<p>
	{{ $arg }}
	{{ range $key, $value := $argmap }}
	<button name="arg.{{ $arg }}" value="{{ $value }}">{{ $key }}</button>
	{{ end }}
	<button name="arg.{{ $arg }}" value="">_empty_</button>
</p>
{{ end }}
</p>

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
`

const templateShowGraphs = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<div style="float:left">
<ul>
		{{ range $key, $value := .Graphs }}
				<li> <a href="/system/getGraph?name={{ $value }}" target="outputframe">{{ $value }}</a> <a href="/graph/{{ $value }}" target="outputframe">Execute</a> <a href="/ui/edit?graph={{ $value }}">Edit</a> </li>
		{{ end }}
</ul>
</div>
<div>
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

const templateFritzDeviceList = `
<meta name="viewport" content="width=device-width, initial-scale=1">
<div>
	{{ range $key, $value := .Device }}
		{{ if $value.Temperature }}
		<div style="float: left; width: 130px; height: 150px; border: 1px solid gray; margin: 10px; overflow:hidden; padding:3px; box-sizing: border-box; ">
			<div style="height: 50px; margin: auto"> {{ $value.Name }} - {{ $value.Temperature.Celsius }} </div>
				{{ if $value.HKR }}
				<div style="margin: auto">

				<form action="/fritz/sethkrtsoll" method="GET" target="outputframe" style="display: flex; justify-content: center">
				<input style="width:50px;" type="number" name="param" min="16" max="56" value="{{ $value.HKR.Tsoll }}" />
				<button name="ain" value="{{ $value.AIN }}">Set</button>
				</form>

				<form action="/fritz/sethkrtsoll" method="GET" target="outputframe">
				<input type="hidden" name="ain" value="{{ $value.AIN }}">
				<div style="display: flex; justify-content: center">
				<button name="param" value="32">16°C</button>
				<button name="param" value="38">19°C</button>
				</div>
				<div style="display: flex; justify-content: center">
				<button name="param" value="44">22°C</button>
				<button name="param" value="50">25°C</button>
				</div>
				</form>
				</div>
				{{ end}}
		</div>
		{{ end}}
	{{ end }}
</div>
`
