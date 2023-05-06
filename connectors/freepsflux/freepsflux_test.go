//go:build !noinflux

package freepsflux

import (
	"encoding/xml"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/hannesrauhe/freepslib"
	"gotest.tools/v3/assert"
)

func TestMetricsToPoints(t *testing.T) {
	ff, err := NewFreepsFlux(&DefaultConfig, nil)
	assert.NilError(t, err)

	met := freepslib.FritzBoxMetrics{DeviceModelName: "7777",
		Uptime:               2,
		DeviceFriendlyName:   "myb",
		BytesReceived:        15,
		BytesSent:            12,
		TransmissionRateUp:   43,
		TransmissionRateDown: 23}

	mtime := time.Unix(1, 0)
	lp, err := ff.MetricsToLineProtocol(&met, mtime)
	assert.NilError(t, err)
	expectedString :=
		`uptime,fb=7777,name=myb seconds=2i 1
bytes_received,fb=7777,name=myb bytes=15i 1
bytes_sent,fb=7777,name=myb bytes=12i 1
transmission_rate_up,fb=7777,name=myb bps=43i 1
transmission_rate_down,fb=7777,name=myb bps=23i 1`
	assert.Equal(t, strings.TrimSpace(lp), expectedString)
}

func fileToPoint(t *testing.T, fileName string, expectedString string, conf FreepsFluxConfig) {
	ff, err := NewFreepsFlux(&conf, nil)
	assert.NilError(t, err)
	byteValue, err := ioutil.ReadFile(fileName)
	assert.NilError(t, err)

	var data *freepslib.AvmDeviceList
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)

	mtime := time.Unix(1, 0)
	lp, err := ff.DeviceListToLineProtocol(data, mtime)
	assert.Equal(t, strings.TrimSpace(lp), expectedString)
}

func TestSteckdoseToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/steckdose.xml", "Steckdose\\ Salon, energy=123841,offset=0,power=0,switch_state_bool=false,temp=22,voltage=229.756 1", DefaultConfig)
}

func TestHKRToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/hkr.xml", "Salon, offset=0,temp=24,temp_set=23,window_open=false 1", DefaultConfig)
}

func TestLampeToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/lampe.xml", "Wohnzimmer\\ Lampe, color_hue=0i,color_saturation=0i,color_temp=2700i,level=135 1", DefaultConfig)
}

func TestExampleDeviceList(t *testing.T) {
	fileToPoint(t, "./_testdata/devicelist.xml", `Salon, offset=0,temp=24,temp_set=23,window_open=false 1
Badezimmer, offset=0,temp=23.5,temp_set=23,window_open=false 1
Kueche, offset=0,temp=23,temp_set=23,window_open=false 1
Schlafzimmer, offset=0,temp=19.5,temp_set=0,window_open=false 1
Kinderzimmer\ links, offset=0,temp=21,temp_set=20,window_open=false 1
Kinderzimmer\ rechts, offset=0,temp=20.5,temp_set=20,window_open=false 1
unused, offset=0,temp=0,temp_set=0,window_open=false 1
Wohnzimmer\ rechts, offset=0,temp=24,temp_set=23,window_open=true 1
Wohnzimmer\ Lampe, color_hue=0i,color_saturation=0i,color_temp=2700i,level=135 1
Steckdose\ Salon, energy=123841,offset=0,power=0,switch_state_bool=false,temp=22,voltage=229.756 1`, DefaultConfig)

	conf := DefaultConfig
	conf.IgnoreNotPresent = true
	fileToPoint(t, "./_testdata/devicelist.xml", `Salon, offset=0,temp=24,temp_set=23,window_open=false 1
Badezimmer, offset=0,temp=23.5,temp_set=23,window_open=false 1
Kueche, offset=0,temp=23,temp_set=23,window_open=false 1
Schlafzimmer, offset=0,temp=19.5,temp_set=0,window_open=false 1
Kinderzimmer\ links, offset=0,temp=21,temp_set=20,window_open=false 1
Kinderzimmer\ rechts, offset=0,temp=20.5,temp_set=20,window_open=false 1
Wohnzimmer\ rechts, offset=0,temp=24,temp_set=23,window_open=true 1
Wohnzimmer\ Lampe, color_hue=0i,color_saturation=0i,color_temp=2700i,level=135 1
Steckdose\ Salon, energy=123841,offset=0,power=0,switch_state_bool=false,temp=22,voltage=229.756 1`, conf)
}
