package utils

import "encoding/json"

// URLArgsToJSON converts the string map of arrays to a string map of strings by dropping
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
