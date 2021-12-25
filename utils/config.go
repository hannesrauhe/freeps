package utils

import (
	"encoding/json"
)

func StructToMap(someStruct interface{}) (map[string]interface{}, error) {
	jsonbytes, err := json.MarshalIndent(someStruct, "", "  ")
	if err != nil {
		return nil, err
	}
	ret := make(map[string]interface{})
	err = json.Unmarshal(jsonbytes, &ret)

	if err != nil {
		return nil, err
	}
	return ret, nil
}

func MergeJsonWithDefaults(jsonBytes []byte, configStruct interface{}) error {
	valueMap, err := StructToMap(configStruct)
	if err != nil {
		return err
	}
	err = json.Unmarshal(jsonBytes, &valueMap)
	if err != nil {
		return err
	}
	mergedBytes, err := json.Marshal(valueMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(mergedBytes, &configStruct)
	if err != nil {
		return err
	}
	return nil
}

func ReadConfigWithDefaults(jsonBytes []byte, sectionName string, configStruct interface{}) ([]byte, error) {
	sectionsMap := make(map[string]interface{})
	var retbytes []byte

	err := json.Unmarshal(jsonBytes, &sectionsMap)

	if err != nil {
		return retbytes, err
	}
	if sectionsMap[sectionName] == nil {
		sectionsMap[sectionName], err = StructToMap(configStruct)
		if err != nil {
			return retbytes, err
		}
		return json.Marshal(sectionsMap)
	}
	sectionBytes, err := json.Marshal(sectionsMap[sectionName])
	if err != nil {
		return retbytes, err
	}

	return retbytes, MergeJsonWithDefaults(sectionBytes, configStruct)
}
