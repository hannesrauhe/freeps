package freepslib

import (
	"crypto/md5"
	"crypto/tls"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"unicode/utf16"
)

type FBconfig struct {
	FB_address string
	FB_user    string
	FB_pass    string
}

var DefaultConfig = FBconfig{"fritz.box", "user", "pass"}

type Freeps struct {
	conf    FBconfig
	SID     string
	Verbose bool
}

func NewFreepsLib(conf *FBconfig) (*Freeps, error) {
	var err error
	f := &Freeps{*conf, "", false}
	f.SID, err = f.getSid()
	if err != nil {
		log.Print("Failed to authenticate")
		return nil, err
	}
	return f, nil
}

func (f *Freeps) getHttpClient() *http.Client {
	tr := &http.Transport{}
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{Transport: tr}
}

/****** AUTH *****/

type AvmSessionInfo struct {
	SID       string
	Challenge string
}

func (f *Freeps) calculateChallengeURL(challenge string) string {
	login_url := "https://" + f.conf.FB_address + "/login_sid.lua"

	// python: hashlib.md5('{}-{}'.format(challenge, password).encode('utf-16-le')).hexdigest()
	u := utf16.Encode([]rune(challenge + "-" + f.conf.FB_pass))
	b := make([]byte, 2*len(u))
	for index, value := range u {
		binary.LittleEndian.PutUint16(b[index*2:], value)
	}
	h := md5.New()
	h.Write(b)
	chal_repsonse := hex.EncodeToString(h.Sum(nil))

	return fmt.Sprintf("%v?username=%v&response=%v-%v", login_url, f.conf.FB_user, challenge, chal_repsonse)
}

func (f *Freeps) getSid() (string, error) {
	login_url := "https://" + f.conf.FB_address + "/login_sid.lua"
	client := f.getHttpClient()
	// get Challenge:
	first_resp, err := client.Get(login_url)
	if err != nil {
		return "", err
	}
	defer first_resp.Body.Close()

	var unauth AvmSessionInfo
	byt, err := ioutil.ReadAll(first_resp.Body)
	if err != nil {
		return "", err
	}
	xml.Unmarshal(byt, &unauth)

	// respond to Challenge and get SID
	second_resp, err := client.Get(f.calculateChallengeURL(unauth.Challenge))
	if err != nil {
		return "", err
	}
	defer second_resp.Body.Close()

	byt, err = ioutil.ReadAll(second_resp.Body)
	if err != nil {
		return "", err
	}
	var authenticated AvmSessionInfo
	err = xml.Unmarshal(byt, &authenticated)
	if err != nil {
		return "", err
	}
	if authenticated.SID == "0000000000000000" {
		return "", errors.New("Authentication failed: wrong user/password")
	}
	return authenticated.SID, nil
}

/****** WebInterface functions *****/

type AvmDeviceInfo struct {
	Mac  string
	UID  string
	Name string
	Type string
}

type AvmDataObject struct {
	Active   []*AvmDeviceInfo
	Passive  []*AvmDeviceInfo
	btn_wake string
}

type AvmDataResponse struct {
	Data *AvmDataObject
}

func (f *Freeps) QueryData(payload map[string]string, AvmResponse interface{}) error {
	data_url := "https://" + f.conf.FB_address + "/data.lua"
	data := url.Values{}
	for key, value := range payload {
		data.Set(key, value)
	}

	data_resp, err := f.getHttpClient().PostForm(data_url, data)
	if err != nil {
		return errors.New("cannot PostForm")
	}
	defer data_resp.Body.Close()

	byt, err := ioutil.ReadAll(data_resp.Body)
	if err != nil {
		return errors.New("cannot read response")
	}
	if data_resp.StatusCode != 200 {
		log.Printf("Unexpected http status: %v, Body:\n %v", data_resp.Status, byt)
		return errors.New("http status code != 200")
	}

	err = json.Unmarshal(byt, &AvmResponse)
	if err != nil {
		log.Printf("Cannot parse JSON: %v", byt)
		return errors.New("cannot parse JSON response")
	}

	if f.Verbose {
		log.Printf("Received data:\n %q\n", byt)
	}
	return nil
}

func (f *Freeps) GetData() (*AvmDataResponse, error) {
	var avmResp *AvmDataResponse
	payload := map[string]string{
		"sid":   f.SID,
		"page":  "netDev",
		"xhrId": "all",
	}

	err := f.QueryData(payload, &avmResp)
	return avmResp, err
}

func getDeviceUID(fb_response AvmDataResponse, mac string) string {
	for _, dev := range append(fb_response.Data.Active, fb_response.Data.Passive...) {
		if dev.Mac == mac {
			return dev.UID
		}
	}
	return ""
}

func (f *Freeps) GetDeviceUID(mac string) (string, error) {
	d, err := f.GetData()

	if err != nil {
		return "", err
	}
	return getDeviceUID(*d, mac), nil
}

func (f *Freeps) WakeUpDevice(uid string) error {
	var avmResp *AvmDataResponse
	payload := map[string]string{
		"sid":      f.SID,
		"dev":      uid,
		"oldpage":  "net/edit_device.lua",
		"page":     "edit_device",
		"btn_wake": "",
	}

	err := f.QueryData(payload, &avmResp)
	if avmResp.Data.btn_wake != "ok" {
		log.Printf("%v", avmResp)
		return errors.New("device wakeup seems to have failed")
	}
	return err
}

/**** HOME AUTOMATION *****/

type AvmDeviceSwitch struct {
	State      bool   `xml:"state"`
	Lock       bool   `xml:"lock"`
	Devicelock bool   `xml:"devicelock"`
	Mode       string `xml:"mode"`
}

type AvmDevicePowermeter struct {
	Power   int `xml:"power"`
	Energy  int `xml:"energy"`
	Voltage int `xml:"voltage"`
}

type AvmDeviceTemperature struct {
	Celsius int `xml:"celsius"`
	Offset  int `xml:"offset"`
}

type AvmDeviceSimpleonoff struct {
	State bool `xml:"state"`
}

type AvmDeviceLevelcontrol struct {
	Level           float32 `xml:"level"`
	LevelPercentage float32 `xml:"levelpercentage"`
}

type AvmDeviceColorcontrol struct {
	Hue        int `xml:"hue"`
	Saturation int `xml:"saturation"`
}

type AvmDeviceHkr struct {
	Tist             int  `xml:"tist"`
	Tsoll            int  `xml:"tsoll"`
	Windowopenactive bool `xml:"windowsopenactiv"` // cannot ignore the typo here
}

type AvmDevice struct {
	Name         string                 `xml:"name" json:",omitempty"`
	AIN          string                 `xml:"identifier,attr"`
	ProductName  string                 `xml:"productname,attr" json:",omitempty"`
	Present      bool                   `xml:"present" json:",omitempty"`
	Switch       *AvmDeviceSwitch       `xml:"switch" json:",omitempty"`
	Temperature  *AvmDeviceTemperature  `xml:"temperature" json:",omitempty"`
	Powermeter   *AvmDevicePowermeter   `xml:"powermeter" json:",omitempty"`
	SimpleOnOff  *AvmDeviceSimpleonoff  `xml:"simpleonoff" json:",omitempty"`
	LevelControl *AvmDeviceLevelcontrol `xml:"levelcontrol" json:",omitempty"`
	ColorControl *AvmDeviceColorcontrol `xml:"colorcontrol" json:",omitempty"`
	HKR          *AvmDeviceHkr          `xml:"hkr" json:",omitempty"`
}

type AvmDeviceList struct {
	Device []AvmDevice `xml:"device"`
}

type AvmTemplate struct {
	Name       string         `xml:"name"`
	Identifier string         `xml:"identifier,attr"`
	ID         string         `xml:"id,attr"`
	Devices    *AvmDeviceList `xml:"devices"`
}

type AvmTemplateList struct {
	Template []AvmTemplate `xml:"template"`
}

func (f *Freeps) queryHomeAutomation(switchcmd string, ain string, payload map[string]string) ([]byte, error) {
	base_url := "https://" + f.conf.FB_address + "/webservices/homeautoswitch.lua"
	var data_url string
	if len(ain) == 0 {
		data_url = fmt.Sprintf("%v?sid=%v&switchcmd=%v", base_url, f.SID, switchcmd)
	} else {
		data_url = fmt.Sprintf("%v?sid=%v&switchcmd=%v&ain=%v", base_url, f.SID, switchcmd, ain)
	}
	for key, value := range payload {
		data_url += "&" + key + "=" + value
	}

	data_resp, err := f.getHttpClient().Get(data_url)
	if err != nil {
		return nil, errors.New("cannot get")
	}
	defer data_resp.Body.Close()

	byt, err := ioutil.ReadAll(data_resp.Body)
	if err != nil {
		return nil, errors.New("cannot read response")
	}
	if data_resp.StatusCode != 200 {
		log.Printf("Unexpected http status: %v, Body:\n %q", data_resp.Status, byt)
		return nil, errors.New("http status code != 200")
	}

	if f.Verbose {
		log.Printf("Received data:\n %q\n", byt)
	}
	return byt, nil
}

func (f *Freeps) GetDeviceList() (*AvmDeviceList, error) {
	byt, err := f.queryHomeAutomation("getdevicelistinfos", "", make(map[string]string))
	if err != nil {
		return nil, err
	}

	var avm_resp *AvmDeviceList
	err = xml.Unmarshal(byt, &avm_resp)
	if err != nil {
		log.Printf("Cannot parse XML: %q, err: %v", byt, err)
		return nil, errors.New("cannot parse XML response")
	}

	return avm_resp, nil
}

func (f *Freeps) GetTemplateList() (*AvmTemplateList, error) {
	byt, err := f.queryHomeAutomation("gettemplatelistinfos", "", make(map[string]string))
	if err != nil {
		return nil, err
	}

	var avm_resp *AvmTemplateList
	err = xml.Unmarshal(byt, &avm_resp)
	if err != nil {
		log.Printf("Cannot parse XML: %q, err: %v", byt, err)
		return nil, errors.New("cannot parse XML response")
	}

	return avm_resp, nil
}

func (f *Freeps) HomeAutoSwitch(switchcmd string, ain string, payload map[string]string) error {
	_, err := f.queryHomeAutomation(switchcmd, ain, payload)
	return err
}

func (f *Freeps) HomeAutomation(switchcmd string, ain string, payload map[string]string) (map[string]interface{}, error) {
	byt, err := f.queryHomeAutomation(switchcmd, ain, payload)
	if err != nil {
		return nil, err
	}
	var result map[string]interface{}

	err = xml.Unmarshal(byt, &result)
	if err != nil {
		return map[string]interface{}{
			"result": string(byt),
		}, nil
	}
	return result, nil
}

func (f *Freeps) SwitchDevice(ain string) error {
	_, err := f.queryHomeAutomation("setsimpleonoff", ain, make(map[string]string))
	return err
}

func (f *Freeps) SetLevel(ain string, level int) error {
	payload := map[string]string{
		"level": fmt.Sprint(level),
	}
	_, err := f.queryHomeAutomation("setlevel", ain, payload)
	return err
}
