//go:build !noinflux

package freepsflux

import (
	"errors"
	"strings"
	"time"

	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	log "github.com/sirupsen/logrus"

	"github.com/hannesrauhe/freepslib"
	influxdb2 "github.com/influxdata/influxdb-client-go/v2"
	"github.com/influxdata/influxdb-client-go/v2/api"
	"github.com/influxdata/influxdb-client-go/v2/api/write"
)

type FreepsFlux struct {
	f         *freepslib.Freeps
	config    FreepsFluxConfig
	Verbose   bool
	writeApis []api.WriteAPI
}

func NewFreepsFlux(conf *FreepsFluxConfig, f *freepslib.Freeps) (*FreepsFlux, error) {
	return &FreepsFlux{f, *conf, false, []api.WriteAPI{}}, nil
}

func (ff *FreepsFlux) InitInflux(reinit bool) error {
	if len(ff.writeApis) > 0 && !reinit {
		return nil
	}

	if len(ff.config.InfluxdbConnections) == 0 {
		return errors.New("No InfluxDB connections configured")
	}

	for _, connConfig := range ff.config.InfluxdbConnections {
		influxOptions := influxdb2.DefaultOptions()
		client := influxdb2.NewClientWithOptions(connConfig.URL, connConfig.Token, influxOptions)
		writeAPI := client.WriteAPI(connConfig.Org, connConfig.Bucket)
		ff.writeApis = append(ff.writeApis, writeAPI)
	}
	return nil
}

func (ff *FreepsFlux) PushFields(measurement string, tags map[string]string, fields map[string]interface{}, ctx *base.Context) error {
	err := ff.InitInflux(false)
	if err != nil {
		return err
	}

	if fields == nil || len(fields) == 0 {
		return nil
	}

	ns := freepsstore.GetGlobalStore().GetNamespace(ff.config.Namespace)
	ns.SetValue(measurement, base.MakeObjectOutput(fields), ctx.GetID())

	for _, writeAPI := range ff.writeApis {
		p := influxdb2.NewPoint(measurement, tags, fields, time.Now())
		writeAPI.WritePoint(p)
		writeAPI.Flush()
	}
	return nil
}

func (ff *FreepsFlux) PushFreepsDeviceList(devl *freepslib.AvmDeviceList) (error, string) {
	err := ff.InitInflux(false)
	if err != nil {
		return err, ""
	}

	mTime := time.Now()

	retString := ""
	for i, writeAPI := range ff.writeApis {
		builder := strings.Builder{}
		ff.DeviceListToPoints(devl, mTime, func(point *write.Point) {
			writeAPI.WritePoint(point)
			write.PointToLineProtocolBuffer(point, &builder, time.Second)
		})
		writeAPI.Flush()
		if i == 0 {
			retString = builder.String()
		}
		if ff.Verbose {
			if i == 0 {
				log.Println(retString)
			}
			log.Printf("Written to Connection %v", ff.config.InfluxdbConnections[i].URL)
		}
	}

	return nil, retString
}
func (ff *FreepsFlux) PushFreepsNetDeviceList(devl *freepslib.AvmDataResponse) (error, string) {
	err := ff.InitInflux(false)
	if err != nil {
		return err, ""
	}

	mTime := time.Now()

	retString := ""
	for i, writeAPI := range ff.writeApis {
		builder := strings.Builder{}
		ff.NetDeviceListToPoints(devl, mTime, func(point *write.Point) {
			writeAPI.WritePoint(point)
			write.PointToLineProtocolBuffer(point, &builder, time.Second)
		})
		writeAPI.Flush()
		if i == 0 {
			retString = builder.String()
		}
		if ff.Verbose {
			if i == 0 {
				log.Println(retString)
			}
			log.Printf("Written to Connection %v", ff.config.InfluxdbConnections[i].URL)
		}
	}

	return nil, retString
}

func (ff *FreepsFlux) PushFreepsMetrics(met *freepslib.FritzBoxMetrics) (error, string) {
	err := ff.InitInflux(false)
	if err != nil {
		return err, ""
	}

	mTime := time.Now()

	retString := ""
	for i, writeAPI := range ff.writeApis {
		builder := strings.Builder{}
		ff.MetricsToPoints(met, mTime, func(point *write.Point) {
			writeAPI.WritePoint(point)
			write.PointToLineProtocolBuffer(point, &builder, time.Second)
		})
		writeAPI.Flush()
		if i == 0 {
			retString = builder.String()
		}
		if ff.Verbose {
			if i == 0 {
				log.Println(retString)
			}
			log.Printf("Written to Connection %v", ff.config.InfluxdbConnections[i].URL)
		}
	}

	return nil, retString
}

func (ff *FreepsFlux) Push() error {
	if ff.f == nil {
		return errors.New("Freepslib unintialized")
	}
	err := ff.InitInflux(false)
	if err != nil {
		return err
	}

	mTime := time.Now()
	devl, err := ff.f.GetDeviceList()
	if err != nil {
		return err
	}
	time1 := time.Now().Unix() - mTime.Unix()
	met, err := ff.f.GetMetrics()
	if err != nil {
		return err
	}
	time2 := time.Now().Unix() - mTime.Unix()
	netd, err := ff.f.GetData()
	if err != nil {
		return err
	}
	time3 := time.Now().Unix() - mTime.Unix()

	if ff.Verbose {
		log.Printf("Retrieving FB data to push to Influx took %vs/%vs/%vs", time1, time2, time3)
	}

	// influxOptions.AddDefaultTag("fb", "6490").AddDefaultTag("hostname", ff.config.Hostname)

	for i, writeAPI := range ff.writeApis {
		ff.DeviceListToPoints(devl, mTime, func(point *write.Point) { writeAPI.WritePoint(point) })
		ff.MetricsToPoints(&met, mTime, func(point *write.Point) { writeAPI.WritePoint(point) })
		ff.NetDeviceListToPoints(netd, mTime, func(point *write.Point) { writeAPI.WritePoint(point) })
		writeAPI.Flush()
		if ff.Verbose {
			log.Printf("Written to Connection %v", ff.config.InfluxdbConnections[i].URL)
		}
	}

	if ff.Verbose {
		builder := strings.Builder{}
		ff.MetricsToPoints(&met, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
		ff.DeviceListToPoints(devl, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
		ff.NetDeviceListToPoints(netd, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
		log.Println(builder.String())
	}

	return nil
}

func (ff *FreepsFlux) MetricsToLineProtocol(met *freepslib.FritzBoxMetrics, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	ff.MetricsToPoints(met, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
	return builder.String(), nil
}

func (ff *FreepsFlux) DeviceListToLineProtocol(devl *freepslib.AvmDeviceList, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	ff.DeviceListToPoints(devl, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
	return builder.String(), nil
}

func (ff *FreepsFlux) NetDeviceListToLineProtocol(resp *freepslib.AvmDataResponse, mTime time.Time) (string, error) {
	builder := strings.Builder{}
	ff.NetDeviceListToPoints(resp, mTime, func(point *write.Point) { write.PointToLineProtocolBuffer(point, &builder, time.Second) })
	return builder.String(), nil
}

func (ff *FreepsFlux) MetricsToPoints(met *freepslib.FritzBoxMetrics, mTime time.Time, f func(*write.Point)) {
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

func (ff *FreepsFlux) DeviceListToPoints(devl *freepslib.AvmDeviceList, mTime time.Time, f func(*write.Point)) error {
	for _, dev := range devl.Device {
		if ff.config.IgnoreNotPresent && !dev.Present {
			continue
		}

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

func (ff *FreepsFlux) NetDeviceListToPoints(resp *freepslib.AvmDataResponse, mTime time.Time, f func(*write.Point)) error {
	p := influxdb2.NewPointWithMeasurement("NetDevices").SetTime(mTime)
	devCount := map[string]uint{}

	for _, v := range resp.Data.Active {
		devCount["active_"+v.Type] = devCount["active_"+v.Type] + 1
	}
	for _, v := range resp.Data.Passive {
		devCount["inactive_"+v.Type] = devCount["inactive_"+v.Type] + 1
	}
	for k, v := range devCount {
		p.AddField(k, v)
	}

	f(p)
	return nil
}
