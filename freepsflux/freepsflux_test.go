package freepsflux

import (
	"encoding/xml"
	"io/ioutil"
	"strings"
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/freepslib"
	"gotest.tools/v3/assert"
)

func TestMetricsToPoints(t *testing.T) {
	met := freepslib.FritzBoxMetrics{DeviceModelName: "7777",
		Uptime:               2,
		DeviceFriendlyName:   "myb",
		BytesReceived:        15,
		BytesSent:            12,
		TransmissionRateUp:   43,
		TransmissionRateDown: 23}

	mtime := time.Unix(1, 0)
	lp, err := MetricsToLineProtocol(met, mtime)
	assert.NilError(t, err)
	expectedString :=
		`uptime,fb=7777,name=myb seconds=2u 1
bytes_received,fb=7777,name=myb bytes=15u 1
bytes_sent,fb=7777,name=myb bytes=12u 1
transmission_rate_up,fb=7777,name=myb bps=43u 1
transmission_rate_down,fb=7777,name=myb bps=23u 1`
	assert.Equal(t, strings.TrimSpace(lp), expectedString)
}

func fileToPoint(t *testing.T, fileName string, expectedString string) {
	byteValue, err := ioutil.ReadFile(fileName)
	assert.NilError(t, err)

	var data *freepslib.AvmDeviceList
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)

	mtime := time.Unix(1, 0)
	lp, err := DeviceListToLineProtocol(data, mtime)
	assert.Equal(t, strings.TrimSpace(lp), expectedString)
}

func TestSteckdoseToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/steckdose.xml", "Steckdose\\ Salon, energy=123841,offset=0,power=0,switch_state=false,temp=22,voltage=229.756 1")
}

func TestHKRToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/hkr.xml", "Salon, offset=0,temp=24,temp_set=23,window_open=false 1")
}

func TestLampeToPoint(t *testing.T) {
	fileToPoint(t, "./_testdata/lampe.xml", "Wohnzimmer\\ Lampe, color_hue=0i,color_saturation=0i,color_temp=2700i,level=135 1")
}

func TestExampleDeviceList(t *testing.T) {
	fileToPoint(t, "./_testdata/devicelist.xml", `Kinderzimmer\ links, temp=21.5,temp_set=20.0 1
Wohnzimmer\ rechts, temp=23.5,temp_set=23.0 1
Kinderzimmer\ rechts, temp=21.0,temp_set=20.0 1
Salon, temp=24.0,temp_set=23.0 1
Badezimmer, temp=24.0,temp_set=23.0 1
Kueche, temp=22.5,temp_set=23.0 1
Schlafzimmer, temp=22.0,temp_set=22.0 1
Steckdose\ Salon, energy=123841.0,power=0.0,switch_state="OFF",temp=22.0,temp_set=0.0 1
`)
}
