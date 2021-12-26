package freepsflux

import (
	"bytes"
	"encoding/json"
	"fmt"

	"github.com/hannesrauhe/freeps/freepslib"
)

type InfluxdbConfig struct {
	ULR    string
	Token  string
	Org    string
	Bucket string
}

type FreepsFluxConfig struct {
	InfluxdbConnections []InfluxdbConfig
	Hostname            string
}

type FreepsFlux struct {
	f      *freepslib.Freeps
	config FreepsFluxConfig
}

var DefaultConfig = FreepsFluxConfig{[]InfluxdbConfig{}, "hostname"}

func NewFreepsFlux(f *freepslib.Freeps) (*FreepsFlux, error) {
	return &FreepsFlux{f, DefaultConfig}, nil
}

func (ff *FreepsFlux) Push() error {
	devl, err := ff.f.GetDeviceList()
	if err != nil {
		return err
	}
	jsonbytes, err := json.MarshalIndent(devl, "", "  ")
	var b bytes.Buffer
	b.Write(jsonbytes)
	fmt.Println(b.String())
	return ff.PushPoints(devl)
}

func (ff *FreepsFlux) PushPoints(devl *freepslib.AvmDeviceList) error {
	// client := influxdb2.NewClient("http://localhost:9999", "my-token")
	// writeAPI := client.WriteAPI("my-org", "my-bucket")
	// mTime := time.Now()

	// for _, v := range devl.Device {
	// 	p := influxdb2.NewPoint(
	// 		v.Name,
	// 		map[string]string{
	// 			"fb":       "6490",
	// 			"hostname": "myhost",
	// 		},
	// 		map[string]interface{}{
	// 			"temperature": rand.Float64() * 80.0,
	// 			"disk_free":   rand.Float64() * 1000.0,
	// 			"disk_total":  (i/10 + 1) * 1000000,
	// 			"mem_total":   (i/100 + 1) * 10000000,
	// 			"mem_free":    rand.Uint64(),
	// 		},
	// 		mTime)
	// 	// write asynchronously
	// 	writeAPI.WritePoint(p)
	// }

	// writeAPI.WritePoint()

	// 	json_body = {
	// 		"tags": {
	// 				"fb": "6490",
	// 				"hostname": self.config["hostname"]
	// 		},
	// 		"points": []
	// }

	// t = int(time.time())
	// for d in self.fh.device_informations():
	// 	name = d["NewDeviceName"]
	// 	fields = {}
	// 	if d["NewTemperatureCelsius"] > 0:
	// 		fields["temp"] = float(d["NewTemperatureCelsius"])/10
	// 		fields["temp_set"] = float(d['NewHkrSetTemperature'])/10
	// 	if d['NewMultimeterIsValid'] == "VALID":
	// 		fields["power"] = float(d["NewMultimeterPower"])/100
	// 		fields["energy"] = float(d["NewMultimeterEnergy"])
	// 	if d['NewSwitchIsValid'] == "VALID":
	// 		fields["switch_state"] = d["NewSwitchState"]

	// 	if len(fields) > 0:
	// 		m = {"measurement": name, "fields": fields, "time": t}
	// 		json_body["points"].append(m)

	// f_status = {
	// 		"uptime": (self.fs.uptime, "seconds"),
	// 		"bytes_received": (self.fs.bytes_received, "bytes"),
	// 		"bytes_sent": (self.fs.bytes_sent, "bytes"),
	// 		"transmission_rate_up": (self.fs.transmission_rate[0], "bps"),
	// 		"transmission_rate_down": (self.fs.transmission_rate[1], "bps")
	// }

	// for name, (v, f) in f_status.items():
	// 	m = {"measurement": name, "fields": {f: v}, "time": t}
	// 	json_body["points"].append(m)

	// lines = line_protocol.make_lines(json_body)
	// print(lines)
	return nil
}
