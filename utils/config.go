package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
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

func ReadSectionWithDefaults(jsonBytes []byte, sectionName string, configStruct interface{}) ([]byte, error) {
	sectionsMap := make(map[string]interface{})
	var retbytes []byte
	var err error

	if len(jsonBytes) > 0 {
		err = json.Unmarshal(jsonBytes, &sectionsMap)
		if err != nil {
			return retbytes, err
		}
	}

	if sectionsMap[sectionName] == nil {
		sectionsMap[sectionName], err = StructToMap(configStruct)
		if err != nil {
			return retbytes, err
		}
		return json.MarshalIndent(sectionsMap, "", "  ")
	}
	sectionBytes, err := json.Marshal(sectionsMap[sectionName])
	if err != nil {
		return retbytes, err
	}

	return retbytes, MergeJsonWithDefaults(sectionBytes, configStruct)
}

func GetDefaultPath(productname string) string {
	dir, _ := os.UserConfigDir()
	return dir + "/" + productname + "/config.json"
}

type ConfigReader struct {
	configFilePath    string
	configFileContent []byte
	configChanged     bool
	lck               sync.Mutex
}

func NewConfigReader(configFilePath string) (*ConfigReader, error) {
	_, err := os.Stat(configFilePath)
	if os.IsNotExist(err) {
		return &ConfigReader{configFilePath: configFilePath, configChanged: true}, nil
	}

	byteValue, err := ioutil.ReadFile(configFilePath)
	if err != nil {
		return nil, err
	}
	return &ConfigReader{configFilePath: configFilePath, configFileContent: byteValue, configChanged: true}, nil
}

func (c *ConfigReader) ReadSectionWithDefaults(sectionName string, configStruct interface{}) error {
	c.lck.Lock()
	defer c.lck.Unlock()

	newb, err := ReadSectionWithDefaults(c.configFileContent, sectionName, configStruct)
	if len(newb) > 0 {
		c.configChanged = true
		c.configFileContent = newb
	}
	return err
}

func (c *ConfigReader) WriteBackConfigIfChanged() error {
	c.lck.Lock()
	defer c.lck.Unlock()

	if !c.configChanged {
		return nil
	}
	dir := filepath.Dir(c.configFilePath)
	err := os.MkdirAll(dir, 0751)
	if err != nil {
		return err
	}
	err = ioutil.WriteFile(c.configFilePath, c.configFileContent, 0644)
	if err == nil {
		c.configChanged = false
	}
	return err
}
