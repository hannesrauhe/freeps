package freeps

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
	"os"
	"unicode/utf16"
)

type FBconfig struct {
	FB_address string
	FB_user    string
	FB_pass    string
}

func WriteFreepsConfig(configpath string, conf *FBconfig) error {
	if conf == nil {
		conf = &FBconfig{"fritz.box", "user", "pass"}
	}

	jsonbytes, err := json.MarshalIndent(conf, "", "  ")

	if err != nil {
		return err
	}
	return ioutil.WriteFile(configpath, jsonbytes, 0644)
}

func ReadFreepsConfig(configpath string) (*FBconfig, error) {
	byteValue, err := ioutil.ReadFile(configpath)
	if err != nil {
		return nil, err
	}

	var conf *FBconfig

	err = json.Unmarshal(byteValue, &conf)

	if err != nil {
		return nil, err
	}

	return conf, nil
}

type Freeps struct {
	conf FBconfig
	SID  string
}

func NewFreeps(configpath string) (*Freeps, error) {
	conf, err := ReadFreepsConfig(configpath)
	if os.IsNotExist(err) {
		err = WriteFreepsConfig(configpath, nil)
		if err != nil {
			log.Print("Failed to create default config file")
			return nil, err
		}
		return nil, errors.New("created default config, please set values")
	}
	if err != nil {
		log.Print("Failed to read config file")
		return nil, err
	}
	f := &Freeps{*conf, ""}
	f.SID, err = f.getSid()
	if err != nil {
		log.Print("Failed to authenticate")
		return nil, err
	}
	return f, nil
}

type avm_session_info struct {
	SID       string
	Challenge string
}

type avm_device_info struct {
	Mac string
	UID string
}

type avm_data_object struct {
	Active  []*avm_device_info
	Passive []*avm_device_info
}

type avm_general_response struct {
	Data *avm_data_object
}

func (f *Freeps) getHttpClient() *http.Client {
	tr := &http.Transport{}
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	return &http.Client{Transport: tr}
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

	var unauth avm_session_info
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
	var authenticated avm_session_info
	err = xml.Unmarshal(byt, &authenticated)
	if err != nil {
		return "", err
	}
	return authenticated.SID, nil
}

func getDeviceUID(fb_response avm_general_response, mac string) string {
	for _, dev := range append(fb_response.Data.Active, fb_response.Data.Passive...) {
		if dev.Mac == mac {
			return dev.UID
		}
	}
	return ""
}

func (f *Freeps) GetData() (*avm_general_response, error) {
	data_url := "https://" + f.conf.FB_address + "/data.lua"

	data := url.Values{}
	data.Set("search_query", "pixar")
	data.Set("sid", f.SID)
	data.Set("page", "netDev")
	data.Set("xhrId", "all")

	data_resp, err := f.getHttpClient().PostForm(data_url, data)
	if err != nil {
		return nil, err
	}
	defer data_resp.Body.Close()

	var avm_resp avm_general_response
	byt, err := ioutil.ReadAll(data_resp.Body)
	if err != nil {
		return nil, err
	}
	if data_resp.StatusCode != 200 {
		log.Printf("Unexpected http status: %v, Body:\n %v", data_resp.Status, byt)
		return nil, errors.New("http status code != 200")
	}

	err = json.Unmarshal(byt, &avm_resp)
	if err != nil {
		log.Printf("Cannot parse JSON: %v", byt)
		return nil, err
	}
	return &avm_resp, nil
}

func (f *Freeps) GetDeviceUID(mac string) (string, error) {
	d, err := f.GetData()

	if err != nil {
		return "", err
	}
	return getDeviceUID(*d, mac), nil
}
