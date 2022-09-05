package utils

import (
	"encoding/json"
	"log"
	"net/http"
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
