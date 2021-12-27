package freepsflux

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/freepslib"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type InfluxdbConfig struct {
	URL    string
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
	influxOptions := influxdb2.DefaultOptions()
	influxOptions.AddDefaultTag("fb", "6490").AddDefaultTag("hostname", ff.config.Hostname)
	mTime := time.Now()

	for _, connConfig := range ff.config.InfluxdbConnections {
		client := influxdb2.NewClientWithOptions(connConfig.URL, connConfig.Token, influxOptions)
		writeAPI := client.WriteAPI(connConfig.Org, connConfig.Bucket)

		for _, v := range devl.Device {
			p, err := DeviceToPoint(&v, mTime)
			if err != nil {
				return err
			}

			writeAPI.WritePoint(p)
		}

		writeAPI.Flush()
	}

	return nil
}

func CreateLineProtocol(devl *freepslib.AvmDeviceList, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	for _, v := range devl.Device {
		p, err := DeviceToPoint(&v, mTime)
		if err != nil {
			return "", err
		}
		write.PointToLineProtocolBuffer(p, &builder, time.Second)
	}
	return builder.String(), nil
}

func DeviceToPoint(dev *freepslib.AvmDevice, mTime time.Time) (*write.Point, error) {
	p := influxdb2.NewPointWithMeasurement(dev.Name).SetTime(mTime)
	if dev.Switch != nil {
		p.AddField("switch_state", dev.Switch.State)
	}
	if dev.Powermeter != nil {
		p.AddField("energy", float64(dev.Powermeter.Energy))
		p.AddField("voltage", float64(dev.Powermeter.Voltage)/1000)
		p.AddField("power", float64(dev.Powermeter.Power)/1000)
	}
	if dev.Temperature != nil {
		p.AddField("temp", float32(dev.Temperature.Celsius)/10)
		p.AddField("offset", float32(dev.Temperature.Offset)/10)
	}
	if dev.HKR != nil {
		p.AddField("temp_set", float32(dev.HKR.Tsoll)/2)
		p.AddField("window_open", dev.HKR.Windowopenactive)
	}
	if dev.ColorControl != nil {
		p.AddField("color_temp", dev.ColorControl.Temperature)
		p.AddField("color_hue", dev.ColorControl.Hue)
		p.AddField("color_saturation", dev.ColorControl.Saturation)
	}
	if dev.LevelControl != nil {
		p.AddField("level", dev.LevelControl.Level)
	}
	p.SortFields()
	return p, nil

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
}
