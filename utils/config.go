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
	Include string
}

// func TemplatesFromUrl(url string) map[string]Template {
// 	t := map[string]Template{}
// 	c := http.Client{}
// 	resp, err := c.Get(url)
// 	if err != nil {
// 		log.Printf("Error when reading from %v: %v", url, err)
// 		return t
// 	}
// 	if resp.StatusCode > 300 {
// 		log.Printf("Error when reading from %v: Status code %v", url, resp.StatusCode)
// 		return t
// 	}
// 	byt, err := ioutil.ReadAll(resp.Body)
// 	if err != nil {
// 		log.Printf("Error when reading from %v: %v", url, err)
// 		return t
// 	}
// 	err = json.Unmarshal(byt, &t)
// 	if err != nil {
// 		log.Printf("Error when parsing json: %v\n %q", err, byt)
// 	}

// 	return t
// }

// func TemplatesFromFile(path string) map[string]Template {
// 	t := map[string]Template{}
// 	byt, err := ioutil.ReadFile(path)
// 	if err != nil {
// 		log.Printf("Error when reading from %v: %v", path, err)
// 		return t
// 	}
// 	err = json.Unmarshal(byt, &t)
// 	if err != nil {
// 		log.Printf("Error when parsing json: %v\n %q", err, byt)
// 	}
// 	return t
// }

///TryIncluding check if sectionBytes contains the include keyword. If it does TryIncluding replaces sectionBytes with the content of the given path
func TryInlcuding(sectionBytes []byte) {
	var ci ConfigIncluder
	err := json.Unmarshal(sectionBytes, &ci)
	if err != nil || len(ci.Include) == 0 {
		return
	}

}

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

	TryInlcuding(sectionBytes)

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
