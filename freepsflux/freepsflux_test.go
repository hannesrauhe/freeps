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

func TestDeviceListFromFile(t *testing.T) {
	byteValue, err := ioutil.ReadFile("./_testdata/devicelist.xml")
	assert.NilError(t, err)

	var data *freepslib.AvmDeviceList
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)

	/*
		Kinderzimmer\ links,fb=6490,hostname=raspi temp=21.5,temp_set=20.0 1640538647
		Wohnzimmer\ rechts,fb=6490,hostname=raspi temp=23.5,temp_set=23.0 1640538647
		Kinderzimmer\ rechts,fb=6490,hostname=raspi temp=21.0,temp_set=20.0 1640538647
		Salon,fb=6490,hostname=raspi temp=24.0,temp_set=23.0 1640538647
		Badezimmer,fb=6490,hostname=raspi temp=24.0,temp_set=23.0 1640538647
		Kueche,fb=6490,hostname=raspi temp=22.5,temp_set=23.0 1640538647
		Schlafzimmer,fb=6490,hostname=raspi temp=22.0,temp_set=22.0 1640538647
		Steckdose\ Salon,fb=6490,hostname=raspi energy=123841.0,power=0.0,switch_state="OFF",temp=22.0,temp_set=0.0 1640538647
		uptime,fb=6490,hostname=raspi seconds=1140085i 1640538647
		bytes_received,fb=6490,hostname=raspi bytes=220642746601i 1640538647
		bytes_sent,fb=6490,hostname=raspi bytes=40754261454i 1640538647
		transmission_rate_up,fb=6490,hostname=raspi bps=6877i 1640538647
		transmission_rate_down,fb=6490,hostname=raspi bps=26317i 1640538647
	*/
}

func fileToPoint(t *testing.T, fileName string, expectedString string) {
	byteValue, err := ioutil.ReadFile(fileName)
	assert.NilError(t, err)

	var data *freepslib.AvmDeviceList
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)

	mtime := time.Unix(1, 0)
	lp, err := CreateLineProtocol(data, mtime)
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
