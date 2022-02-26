package utils

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
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

// OverwriteValuesWithJson will use the values in jsonBytes to append/overwrite the data in valueMap
// and returns the json string of the combined values
func OverwriteValuesWithJson(jsonBytes []byte, valueMap map[string]interface{}) ([]byte, error) {
	err := json.Unmarshal(jsonBytes, &valueMap)
	if err != nil {
		return nil, err
	}
	return json.Marshal(valueMap)
}

func MergeJsonWithDefaults(jsonBytes []byte, configStruct interface{}) error {
	valueMap, err := StructToMap(configStruct)
	if err != nil {
		return err
	}
	mergedBytes, err := OverwriteValuesWithJson(jsonBytes, valueMap)
	if err != nil {
		return err
	}
	err = json.Unmarshal(mergedBytes, &configStruct)
	if err != nil {
		return err
	}
	return nil
}

type ConfigIncluder struct {
	IncludeFromFile string
	IncludeFromURL  string
}

func ReadBytesFromUrl(url string) []byte {
	var byt []byte
	c := http.Client{}
	resp, err := c.Get(url)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return []byte{}
	}
	if resp.StatusCode > 300 {
		log.Printf("Error when reading from %v: Status code %v", url, resp.StatusCode)
		return []byte{}
	}
	byt, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return []byte{}
	}
	return byt
}

func ReadBytesFromFile(path string) []byte {
	var err error
	var byt []byte
	byt, err = ioutil.ReadFile(path)
	if err != nil {
		log.Printf("Error when reading from %v: %v", path, err)
		return []byte{}
	}
	return byt
}

// ReadSectionWithDefaults parses the content of the first-level-JSON object in <sectionName> into configStruct
// if the section exists, the content is merged with configStruct;
// if the section does not exist, the serialized content of configStruct is returned (assuming these are default values)
func ReadSectionWithDefaults(jsonBytes []byte, sectionName string, configStruct interface{}) ([]byte, error) {
	sectionsMap := make(map[string]interface{})
	var err error

	if len(jsonBytes) > 0 {
		err = json.Unmarshal(jsonBytes, &sectionsMap)
		if err != nil {
			return []byte{}, err
		}
	}

	if sectionsMap[sectionName] == nil {
		// section is missing, will include the default in the JSON string
		sectionsMap[sectionName], err = StructToMap(configStruct)
		if err != nil {
			return []byte{}, err
		}
		return json.MarshalIndent(sectionsMap, "", "  ")
	}
	sectionBytes, err := json.Marshal(sectionsMap[sectionName])
	if err != nil {
		return []byte{}, err
	}

	// checking if the section just redirects to another config
	var ci ConfigIncluder
	err = json.Unmarshal(sectionBytes, &ci)
	if err == nil {
		externalBytes := []byte{}
		if len(ci.IncludeFromFile) > 0 {
			externalBytes = ReadBytesFromFile(ci.IncludeFromFile)
		} else if len(ci.IncludeFromURL) > 0 {
			externalBytes = ReadBytesFromUrl(ci.IncludeFromURL)
		}
		if len(externalBytes) > 0 {
			// redirected file contains values, merging
			return []byte{}, MergeJsonWithDefaults(externalBytes, configStruct)
		}
	}

	return []byte{}, MergeJsonWithDefaults(sectionBytes, configStruct)
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
