// Package fritzboxmetrics provides metrics fro the UPnP and Tr64 interface
package fritzboxmetrics

// Copyright 2016 Nils Decker
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	dac "github.com/123Haynes/go-http-digest-auth-client"
)

const (
	RFC3339_WITHOUT_TZ = "2006-01-02T15:04:05"
)

// curl http://fritz.box:49000/igddesc.xml
// curl http://fritz.box:49000/any.xml
// curl http://fritz.box:49000/igdconnSCPD.xml
// curl http://fritz.box:49000/igdicfgSCPD.xml
// curl http://fritz.box:49000/igddslSCPD.xml
// curl http://fritz.box:49000/igd2ipv6fwcSCPD.xml

const textXML = `text/xml; charset="utf-8"`

// ErrInvalidSOAPResponse will be thrown if we've got an invalid SOAP response
var ErrInvalidSOAPResponse = errors.New("invalid SOAP response")

// Root of the UPNP tree
type Root struct {
	BaseURL  string
	Username string
	Password string
	Device   Device              `xml:"device"`
	Services map[string]*Service // Map of all services indexed by .ServiceType
}

// Device represents an UPNP device
type Device struct {
	root *Root

	DeviceType       string `xml:"deviceType"`
	FriendlyName     string `xml:"friendlyName"`
	Manufacturer     string `xml:"manufacturer"`
	ManufacturerURL  string `xml:"ManufacturerURL"`
	ModelDescription string `xml:"modelDescription"`
	ModelName        string `xml:"modelName"`
	ModelNumber      string `xml:"modelNumber"`
	ModelURL         string `xml:"ModelURL"`
	UDN              string `xml:"UDN"`

	Services []*Service `xml:"serviceList>service"` // Service of the device
	Devices  []*Device  `xml:"deviceList>device"`   // Sub-Devices of the device

	PresentationURL string `xml:"PresentationURL"`
}

// Service represents an UPnP Service
type Service struct {
	Device *Device

	ServiceType string `xml:"serviceType"`
	ServiceID   string `xml:"serviceId"`
	ControlURL  string `xml:"controlURL"`
	EventSubURL string `xml:"eventSubURL"`
	SCPDURL     string `xml:"SCPDURL"`

	Actions        map[string]*Action // All actions available on the service
	StateVariables []*StateVariable   // All state variables available on the service
}

type scpdRoot struct {
	Actions        []*Action        `xml:"actionList>action"`
	StateVariables []*StateVariable `xml:"serviceStateTable>stateVariable"`
}

// Action represents an UPnP Action on a Service
type Action struct {
	service *Service

	Name        string               `xml:"name"`
	Arguments   []*Argument          `xml:"argumentList>argument"`
	ArgumentMap map[string]*Argument // Map of arguments indexed by .Name
}

// IsGetOnly returns if the action seems to be a query for information.
// This is determined by checking if the action has no input arguments and at least one output argument.
func (a *Action) IsGetOnly() bool {
	for _, a := range a.Arguments {
		if a.Direction == "in" {
			return false
		}
	}
	return len(a.Arguments) > 0
}

// An Argument to an action
type Argument struct {
	Name                 string `xml:"name"`
	Direction            string `xml:"direction"`
	RelatedStateVariable string `xml:"relatedStateVariable"`
	StateVariable        *StateVariable
}

// StateVariable is a variable that can be manipulated through actions
type StateVariable struct {
	Name         string `xml:"name"`
	DataType     string `xml:"dataType"`
	DefaultValue string `xml:"defaultValue"`
}

// Result are all output argements of the Call():
// The map is indexed by the name of the state variable.
// The type of the value is string, uint64 or bool depending of the DataType of the variable.
type Result map[string]interface{}

// load the whole tree
func (r *Root) load() error {
	response, err := http.Get(fmt.Sprintf("%s/igddesc.xml", r.BaseURL))
	if err != nil {
		return fmt.Errorf("could not get igddesc.xml: %w", err)
	}

	dec := xml.NewDecoder(response.Body)

	if err = dec.Decode(r); err != nil {
		return fmt.Errorf("could not decode XML: %w", err)
	}

	r.Services = make(map[string]*Service)
	return r.Device.fillServices(r)
}

func (r *Root) loadTr64() error {
	igddesc, err := http.Get(fmt.Sprintf("%s/tr64desc.xml", r.BaseURL))
	if err != nil {
		return fmt.Errorf("could not fetch tr64desc.xml: %w", err)
	}

	dec := xml.NewDecoder(igddesc.Body)
	if err = dec.Decode(r); err != nil {
		return fmt.Errorf("could not decode XML: %w", err)
	}

	r.Services = make(map[string]*Service)
	return r.Device.fillServices(r)
}

// load all service descriptions
func (d *Device) fillServices(r *Root) error {
	d.root = r

	for _, s := range d.Services {
		s.Device = d

		response, err := http.Get(r.BaseURL + s.SCPDURL)
		if err != nil {
			return fmt.Errorf("could not get service descriptions: %w", err)
		}

		var scpd scpdRoot

		dec := xml.NewDecoder(response.Body)
		if err = dec.Decode(&scpd); err != nil {
			return fmt.Errorf("could not decode xml: %w", err)
		}

		s.Actions = make(map[string]*Action)
		for _, a := range scpd.Actions {
			s.Actions[a.Name] = a
		}
		s.StateVariables = scpd.StateVariables

		for _, a := range s.Actions {
			a.service = s
			a.ArgumentMap = make(map[string]*Argument)

			for _, arg := range a.Arguments {
				for _, svar := range s.StateVariables {
					if arg.RelatedStateVariable == svar.Name {
						arg.StateVariable = svar
					}
				}

				a.ArgumentMap[arg.Name] = arg
			}
		}

		r.Services[s.ServiceType] = s
	}
	for _, d2 := range d.Devices {
		if err := d2.fillServices(r); err != nil {
			return fmt.Errorf("could not fill services: %w", err)
		}
	}
	return nil
}

// Call an action.
// Currently only actions without input arguments are supported.
func (a *Action) Call() (Result, error) {
	bodystr := fmt.Sprintf(`
        <?xml version='1.0' encoding='utf-8'?>
        <s:Envelope s:encodingStyle='http://schemas.xmlsoap.org/soap/encoding/' xmlns:s='http://schemas.xmlsoap.org/soap/envelope/'>
            <s:Body>
                <u:%s xmlns:u='%s' />
            </s:Body>
        </s:Envelope>
    `, a.Name, a.service.ServiceType)

	url := a.service.Device.root.BaseURL + a.service.ControlURL
	body := strings.NewReader(bodystr)

	req, err := http.NewRequest("POST", url, body)
	if err != nil {
		return nil, fmt.Errorf("could not create new request: %w", err)
	}

	action := fmt.Sprintf("%s#%s", a.service.ServiceType, a.Name)

	req.Header.Set("Content-Type", textXML)
	req.Header.Set("SoapAction", action)

	// Add digest authentification
	t := dac.NewTransport(a.service.Device.root.Username, a.service.Device.root.Password)
	resp, err := t.RoundTrip(req)
	if err != nil {
		return nil, fmt.Errorf("could not roundtrip digest authentification: %w", err)
	}
	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("authorization required")
	}

	data := new(bytes.Buffer)
	if _, err := data.ReadFrom(resp.Body); err != nil {
		return nil, fmt.Errorf("could not read body: %w", err)
	}

	return a.parseSoapResponse(data)

}

func (a *Action) parseSoapResponse(r io.Reader) (Result, error) {
	res := make(Result)
	dec := xml.NewDecoder(r)

	for {
		t, err := dec.Token()
		if err == io.EOF {
			return res, nil
		}

		if err != nil {
			return nil, err
		}

		if se, ok := t.(xml.StartElement); ok {
			arg, ok := a.ArgumentMap[se.Name.Local]

			if ok {
				t2, err := dec.Token()
				if err != nil {
					return nil, err
				}

				var val string
				switch element := t2.(type) {
				case xml.EndElement:
					val = ""
				case xml.CharData:
					val = string(element)
				default:
					return nil, ErrInvalidSOAPResponse
				}

				converted, err := convertResult(val, arg)
				if err != nil {
					return nil, err
				}
				res[arg.StateVariable.Name] = converted
			}
		}

	}
}

func convertResult(val string, arg *Argument) (interface{}, error) {
	switch arg.StateVariable.Name {
	case "X_AVM_DE_TotalBytesSent64", "X_AVM_DE_TotalBytesReceived64":
		res, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse uint: %w", err)
		}
		return uint64(res), nil
	}

	switch arg.StateVariable.DataType {
	case "string":
		return val, nil
	case "boolean":
		return bool(val == "1"), nil

	case "ui1", "ui2", "ui4":
		// type ui4 can contain values greater than 2^32!
		res, err := strconv.ParseUint(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse uint: %w", err)
		}
		return uint64(res), nil

	case "i1", "i2", "i4":
		// type i4 can contain values greater than 2^32!, 2^64 to be precise. ParseInt returns int64 anyways.
		res, err := strconv.ParseInt(val, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("could not parse int: %w", err)
		}
		return int64(res), nil

	case "dateTime":
		// UPnP uses ISO8601 (non-strict RFC3339) with optional TZ.
		// try RFC3339 first
		res, err := time.Parse(time.RFC3339, val)
		if err == nil {
			// RFC3339 complaient. Yay.
			return res, nil
		}
		// if RFC3339 fails, try without TZ
		res, err = time.Parse(RFC3339_WITHOUT_TZ, val)
		if err != nil {
			return nil, fmt.Errorf("could not parse dateTime: %w", err)
		}
		return res, nil

	case "dateTime.tz":
		res, err := time.Parse(time.RFC3339, val)
		if err != nil {
			return nil, fmt.Errorf("could not parse dateTime.tz: %w", err)
		}
		return res, nil
	default:
		return nil, fmt.Errorf("unknown datatype: %s", arg.StateVariable.DataType)
	}
}

// LoadServices loads the services tree from a device.
func LoadServices(device string, port uint16, username string, password string) (*Root, error) {
	root := &Root{
		BaseURL:  fmt.Sprintf("http://%s:%d", device, port),
		Username: username,
		Password: password,
	}

	if err := root.load(); err != nil {
		return nil, fmt.Errorf("could not load root element: %w", err)
	}

	rootTr64 := &Root{
		BaseURL:  fmt.Sprintf("http://%s:%d", device, port),
		Username: username,
		Password: password,
	}

	if err := rootTr64.loadTr64(); err != nil {
		return nil, fmt.Errorf("could not load Tr64: %w", err)
	}

	for k, v := range rootTr64.Services {
		root.Services[k] = v
	}

	return root, nil
}
