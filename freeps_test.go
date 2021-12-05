package freeps

import (
	"encoding/json"
	"encoding/xml"
	"io/ioutil"
	"os"
	"testing"

	"gotest.tools/v3/assert"
	is "gotest.tools/v3/assert/cmp"
)

func TestFreepsConfig(t *testing.T) {
	tmpFile, err := ioutil.TempFile(os.TempDir(), "freepstest-")
	assert.NilError(t, err)
	tmpFileName := tmpFile.Name()
	assert.NilError(t, tmpFile.Close())
	defer os.Remove(tmpFileName)

	// write default conf and read it
	assert.NilError(t, WriteFreepsConfig(tmpFileName, nil))
	conf, err := ReadFreepsConfig(tmpFileName)
	assert.NilError(t, err)
	expected_conf := &FBconfig{"fritz.box", "user", "pass"}
	assert.DeepEqual(t, conf, expected_conf)

	// read prepared conf file
	conf, err = ReadFreepsConfig("./config_for_gotest.json")
	assert.NilError(t, err)
	expected_conf = &FBconfig{"a", "u", "p"}
	assert.DeepEqual(t, conf, expected_conf)

	// write non-default conf and read it
	assert.NilError(t, WriteFreepsConfig(tmpFileName, expected_conf))
	conf, err = ReadFreepsConfig(tmpFileName)
	assert.NilError(t, err)
	assert.DeepEqual(t, conf, expected_conf)

	// check error for non-existent conf
	conf, err = ReadFreepsConfig("./gibtsnich")
	assert.ErrorType(t, err, os.IsNotExist)
	assert.Assert(t, is.Nil(conf))
}

func TestChallenge(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest.json")
	assert.NilError(t, err)
	expected_url := "https://a/login_sid.lua?username=u&response=a51eacbd-05f2dd791db47141584e0f220b12c7e1"

	assert.Equal(t, f.calculateChallengeURL("a51eacbd"), expected_url)
}

func TestGetUID(t *testing.T) {
	byteValue, err := ioutil.ReadFile("./test_data.json")
	assert.NilError(t, err)

	mac := "40:8D:5C:5B:63:2D"
	var data *avm_data_response
	err = json.Unmarshal(byteValue, &data)
	assert.NilError(t, err)
	assert.Equal(t, getDeviceUID(*data, mac), "landevice3489")
}

func TestSID(t *testing.T) {
	c, err := ReadFreepsConfig("./config_for_gotest_real.json")
	assert.NilError(t, err)
	f := &Freeps{*c, ""}
	sid, err := f.getSid()
	assert.NilError(t, err)
	assert.Assert(t, sid != "")
}

func TestData(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	mac := "40:8D:5C:5B:63:2D"
	uid, err := f.GetDeviceUID(mac)
	assert.NilError(t, err)
	assert.Equal(t, uid, "landevice3489")
}

func TestWakeUp(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.WakeUpDevice("landevice3489")
	assert.NilError(t, err)
}

func TestDeviceListUnmarshal(t *testing.T) {
	byteValue, err := ioutil.ReadFile("./test_devicelist.xml")
	assert.NilError(t, err)

	var data *avm_devicelist
	err = xml.Unmarshal(byteValue, &data)
	assert.NilError(t, err)
	assert.Equal(t, data.Device[0].Name, "Steckdose")
}

func TestDeviceList(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	_, err = f.GetDeviceList()
	assert.NilError(t, err)
}

func TestSwitchLampOff(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.SetLevel("13077 0013108-1", 0)
	assert.NilError(t, err)
}

func TestSwitchLampOn(t *testing.T) {
	f, err := NewFreeps("./config_for_gotest_real.json")
	assert.NilError(t, err)

	err = f.SetLevel("13077 0013108-1", 37)
	assert.NilError(t, err)
}
