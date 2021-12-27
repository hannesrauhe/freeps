module github.com/hannesrauhe/freeps

go 1.15

require (
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb-client-go/v2 v2.6.0 // indirect
	github.com/mxschmitt/fritzbox_exporter v0.0.0-20210413101219-fd36539bd7db // indirect
	github.com/pkg/errors v0.9.1 // indirect
	gotest.tools/v3 v3.0.3
)

replace github.com/hannesrauhe/freeps/lib => ./lib

replace github.com/hannesrauhe/freeps/utils => ./utils
