package utils

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// URLArgsToMap converts the string map of arrays to a string map of strings by joining multiple values per key
// this simplfies handling of single key-value pairs, while consciously sacrificing keys with multiple values
func URLArgsToMap(args url.Values) map[string]string {
	retMap := map[string]string{}
	for k, v := range args {
		if len(v) > 1 {
			strings.Join(v, ",")
		}
		retMap[k] = v[0]
	}
	return retMap
}

// URLArgsToJSON converts the string map of arrays to a string map of strings by dropping
// all but the first elements from the map, it returns the json serialization of the map
func URLArgsToJSON(args map[string][]string) []byte {
	retMap := URLArgsToMap(args)
	byt, _ := json.Marshal(retMap)
	return byt
}

// URLParseQuery parses a query string and returns a map of the values
func URLParseQuery(query string) (map[string]string, error) {
	v, err := url.ParseQuery(query)
	if err != nil {
		return nil, err
	}
	return URLArgsToMap(v), nil
}

// MapToURLArgs converts a string map to a string map of arrays and splits multiple values per key
func MapToURLArgs(args map[string]string) url.Values {
	retMap := url.Values{}
	for k, v := range args {
		retMap[k] = strings.Split(v, ",")
	}
	return retMap
}

// ReadObjectFromURL decodes a JSON object from an http stream
func ReadObjectFromURL(url string, obj interface{}) error {
	c := http.Client{}
	resp, err := c.Get(url)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return err
	}
	if resp.StatusCode > 300 {
		log.Printf("Error when reading from %v: Status code %v", url, resp.StatusCode)
		return err
	}
	d := json.NewDecoder(resp.Body)
	err = d.Decode(obj)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return err
	}

	return nil
}

// MapToObject converts a map to an object via JSON encode/decode
func MapToObject(args map[string]interface{}, obj interface{}) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, obj)
	return err
}

// ArgsMapToObject converts a string map to an object via JSON encode/decode
func ArgsMapToObject(args map[string]string, obj interface{}) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, obj)
	return err
}

// ObjectToArgsMap converts and object to a string map via JSON encode/decode
func ObjectToArgsMap(obj interface{}) (map[string]string, error) {
	args := map[string]string{}
	data, err := json.Marshal(obj)
	if err != nil {
		return args, err
	}

	anyMap := map[string]interface{}{}
	err = json.Unmarshal(data, &anyMap)
	for k, v := range anyMap {
		args[k] = fmt.Sprintf("%v", v)
	}
	return args, err
}

// ObjectToMap converts and object to a map via JSON encode/decode
func ObjectToMap(obj interface{}) (map[string]interface{}, error) {
	anyMap := map[string]interface{}{}
	data, err := json.Marshal(obj)
	if err != nil {
		return anyMap, err
	}

	err = json.Unmarshal(data, &anyMap)
	return anyMap, err
}

func ClearString(str string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

func ParseColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	if len(s) == 0 {
		err = fmt.Errorf("empty color")
		return
	}
	if s[0] == '#' {
		return ParseHexColor(s)
	}
	switch s {
	case "transparent":
		c.A = 0x0
	default:
		err = fmt.Errorf("Unknown color: %v", s)
	}
	return
}

func ParseHexColor(s string) (c color.RGBA, err error) {
	c.A = 0xff
	switch len(s) {
	case 7:
		_, err = fmt.Sscanf(s, "#%02x%02x%02x", &c.R, &c.G, &c.B)
	case 4:
		_, err = fmt.Sscanf(s, "#%1x%1x%1x", &c.R, &c.G, &c.B)
		// Double the hex digits:
		c.R *= 17
		c.G *= 17
		c.B *= 17
	default:
		err = fmt.Errorf("invalid length, must be 7 or 4")
	}
	return
}

// GetHexColor returns the hex represantion of a Go-color
func GetHexColor(c color.Color) string {
	r, g, b, _ := c.RGBA()
	r = r >> 8
	g = g >> 8
	b = b >> 8
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

// GetDurationMap returns a map of typical durations for operator argument suggestions
func GetDurationMap() map[string]string {
	return map[string]string{"1s": "1s", "10s": "10s", "100s": "100s", "1m": "1m", "10m": "10m", "100m": "100m", "1h": "1h", "12h": "12h"}
}

// DeleteElemFromSlice swaps i-th and last Element and deletes the last
func DeleteElemFromSlice(s []string, i int) []string {
	if i >= len(s) || i < 0 {
		return s
	}
	if i < len(s)-1 {
		s[i] = s[len(s)-1]
	}
	return s[:len(s)-1]
}

func StringToIdentifier(input string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9_]+")
	if err != nil {
		log.Fatal(err)
	}
	output := reg.ReplaceAllString(input, "")
	return strings.ToLower(output)
}

func StringStartsWith(input string, prefix string) bool {
	return len(input) >= len(prefix) && input[0:len(prefix)] == prefix
}

func StringEndsWith(input string, suffix string) bool {
	return len(input) >= len(suffix) && input[len(input)-len(suffix):] == suffix
}

// StringToLower converts a string to lower case
func StringToLower(input string) string {
	return strings.ToLower(input)
}

// StringCmpIgnoreCase compares two strings ignoring case
func StringCmpIgnoreCase(a string, b string) bool {
	return strings.ToLower(a) == strings.ToLower(b)
}

// StringToBool converts a string to a bool
func StringToBool(input string) bool {
	return ParseBool(input)
}

// StringToFloat64 converts a string to a float64
func StringToFloat64(input string) (float64, error) {
	return strconv.ParseFloat(input, 64)
}

// StringToInt converts a string to an int
func StringToInt(input string) (int, error) {
	return strconv.Atoi(input)
}

// StringPtr returns a pointer to a string
func StringPtr(input string) *string {
	return &input
}

// KeysToLower converts all keys in a map to lower case (always returns a new map, even if the input map is nil)
func KeysToLower(input map[string]string) map[string]string {
	lowercaseMap := map[string]string{}
	if input == nil {
		return lowercaseMap
	}
	for k, v := range input {
		lowercaseMap[StringToLower(k)] = v
	}
	return lowercaseMap
}
