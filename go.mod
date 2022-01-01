module github.com/hannesrauhe/freeps

go 1.15

require (
	github.com/123Haynes/go-http-digest-auth-client v0.3.1-0.20171226204513-4c2ff1556cab
	github.com/google/go-cmp v0.5.6 // indirect
	github.com/gorilla/mux v1.8.0
	github.com/influxdata/influxdb-client-go/v2 v2.6.0
	gopkg.in/check.v1 v1.0.0-20201130134442-10cb98267c6c // indirect
	gotest.tools/v3 v3.0.3
)

replace github.com/hannesrauhe/freeps/lib => ./lib

replace github.com/hannesrauhe/freeps/utils => ./utils

replace github.com/hannesrauhe/freeps/freepslib/fritzboxmetrics => ./freepslib/fritzboxmetrics
