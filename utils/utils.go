package utils

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"regexp"
	"strconv"
	"strings"

	log "github.com/sirupsen/logrus"
)

// URLArgsToMap converts the string map of arrays to a string map of strings by dropping
// all but the first elements from the map, it returns the resulting map
func URLArgsToMap(args map[string][]string) map[string]string {
	retMap := map[string]string{}
	for k, v := range args {
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

// ArgsMapToObject converts a string map to an object via JSON encode/decode
func ArgsMapToObject(args map[string]string, obj interface{}) error {
	data, err := json.Marshal(args)
	if err != nil {
		return err
	}
	err = json.Unmarshal(data, obj)
	return err
}

func ClearString(str string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]+`)
	return nonAlphanumericRegex.ReplaceAllString(str, "")
}

// ParseBool returns the bool value represented by string
func ParseBool(str string) bool {
	v, err := strconv.ParseBool(str)
	if err != nil {
		str = strings.ToLower(str)
		switch str {
		case "on", "yes":
			return true
		default:
			return false
		}
	}
	return v
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
	return map[string]string{"1s": "1s", "10s": "10s", "100s": "100s"}
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
