package freepsflux

import (
	"errors"
	"log"
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
	IgnoreNotPresent    bool
}

type FreepsFlux struct {
	f       *freepslib.Freeps
	config  FreepsFluxConfig
	Verbose bool
}

var DefaultConfig = FreepsFluxConfig{[]InfluxdbConfig{}, "hostname", false}

func NewFreepsFlux(conf *FreepsFluxConfig, f *freepslib.Freeps) (*FreepsFlux, error) {
	return &FreepsFlux{f, *conf, false}, nil
}

func (ff *FreepsFlux) Push() error {
	if len(ff.config.InfluxdbConnections) == 0 {
		return errors.New("No InfluxDB connections configured")
	}

	devl, err := ff.f.GetDeviceList()
	if err != nil {
		return err
	}

	met, err := ff.f.GetMetrics()
	if err != nil {
		return err
	}

	influxOptions := influxdb2.DefaultOptions()
	// influxOptions.AddDefaultTag("fb", "6490").AddDefaultTag("hostname", ff.config.Hostname)
	mTime := time.Now()

	for _, connConfig := range ff.config.InfluxdbConnections {
		client := influxdb2.NewClientWithOptions(connConfig.URL, connConfig.Token, influxOptions)
		writeAPI := client.WriteAPI(connConfig.Org, connConfig.Bucket)

		DeviceListToPoints(devl, mTime, func(point *write.Point) { writeAPI.WritePoint(point) })
		MetricsToPoints(met, mTime, func(point *write.Point) { writeAPI.WritePoint(point) })
		writeAPI.Flush()
		log.Printf("Written to %v", connConfig.URL)
	}

	if ff.Verbose {
		builder := strings.Builder{}
		MetricsToPoints(met, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
		DeviceListToPoints(devl, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
		log.Println(builder.String())
	}

	return nil
}

func MetricsToLineProtocol(met freepslib.FritzBoxMetrics, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	MetricsToPoints(met, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
	return builder.String(), nil
}

func DeviceListToLineProtocol(devl *freepslib.AvmDeviceList, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	DeviceListToPoints(devl, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
	return builder.String(), nil
}

func MetricsToPoints(met freepslib.FritzBoxMetrics, mTime time.Time, f func(*write.Point)) {
	tags := map[string]string{"fb": met.DeviceModelName, "name": met.DeviceFriendlyName}

	p := influxdb2.NewPoint("uptime", tags, map[string]interface{}{
		"seconds": met.Uptime,
	}, mTime)
	f(p)

	p = influxdb2.NewPoint("bytes_received", tags, map[string]interface{}{
		"bytes": met.BytesReceived,
	}, mTime)
	f(p)

	p = influxdb2.NewPoint("bytes_sent", tags, map[string]interface{}{
		"bytes": met.BytesSent,
	}, mTime)
	f(p)

	p = influxdb2.NewPoint("transmission_rate_up", tags, map[string]interface{}{
		"bps": met.TransmissionRateUp,
	}, mTime)
	f(p)

	p = influxdb2.NewPoint("transmission_rate_down", tags, map[string]interface{}{
		"bps": met.TransmissionRateDown,
	}, mTime)
	f(p)
}

func DeviceListToPoints(devl *freepslib.AvmDeviceList, mTime time.Time, f func(*write.Point)) error {
	for _, dev := range devl.Device {

		p := influxdb2.NewPointWithMeasurement(dev.Name).SetTime(mTime)
		if dev.Switch != nil {
			p.AddField("switch_state_bool", dev.Switch.State)
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
			if dev.HKR.Tsoll == 253 {
				p.AddField("temp_set", float32(0))
			} else if dev.HKR.Tsoll == 254 {
				p.AddField("temp_set", float32(31))
			} else {
				p.AddField("temp_set", float32(dev.HKR.Tsoll)/2)
			}
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
		if len(p.FieldList()) != 0 {
			f(p)
		}
	}
	return nil
}
