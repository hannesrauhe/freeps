package utils

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"sync"
)

type ConfigIncluder struct {
	Include        string
	IncludeFromURL string
}

func ReadBytesFromUrl(url string) []byte {
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
	byt, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Error when reading from %v: %v", url, err)
		return []byte{}
	}
	return byt
}

func ReadBytesFromFile(filePath string, configFileDir string) []byte {
	if !path.IsAbs(filePath) {
		filePath = path.Join(configFileDir, filePath)
	}
	byt, err := ioutil.ReadFile(filePath)
	if err != nil {
		log.Printf("Error when reading from %v: %v", filePath, err)
		return []byte{}
	}
	return byt
}

func GetSectionsMap(jsonBytes []byte) (map[string]interface{}, error) {
	sectionsMap := make(map[string]interface{})
	var err error

	if len(jsonBytes) > 0 {
		err = json.Unmarshal(jsonBytes, &sectionsMap)
	}
	return sectionsMap, err
}

// ReadSectionWithDefaults parses the content of the first-level-JSON object in <sectionName> into configStruct
//
// if the section exists, configStruct will first get overwritten by an optional included file, then by the contents of the section
// (append+overwrite in both cases), returns an empty byte-slice
// if the section does not exist, the serialized content of configStruct (assuming these are default values) is added to jsonBytes and returned
func ReadSectionWithDefaults(jsonBytes []byte, sectionName string, configStruct interface{}, configFileDir string) ([]byte, error) {
	sectionsMap, err := GetSectionsMap(jsonBytes)
	if err != nil {
		return []byte{}, err
	}

	if sectionsMap[sectionName] == nil {
		// section is missing, will include the values from configStruct in the JSON string
		sectionsMap[sectionName] = configStruct
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
		externalSectionBytes := []byte{}
		if len(ci.Include) > 0 {
			externalSectionBytes = ReadBytesFromFile(ci.Include, configFileDir)
		} else if len(ci.IncludeFromURL) > 0 {
			externalSectionBytes = ReadBytesFromUrl(ci.IncludeFromURL)
		}
		if len(externalSectionBytes) > 0 {
			// redirected file contains values, merging
			err := json.Unmarshal(externalSectionBytes, configStruct)
			if err != nil {
				return []byte{}, err
			}
		}
	}

	// finally merge defaults, the external struct and the actual sectionBytes from the config file
	return []byte{}, json.Unmarshal(sectionBytes, configStruct)
}

// WriteSection puts the ConfigStruct object in the config file by preserving everything that is part of the section
func WriteSection(jsonBytes []byte, sectionName string, configStruct interface{}) ([]byte, error) {
	sectionsMap, err := GetSectionsMap(jsonBytes)
	if err != nil {
		return []byte{}, err
	}

	if sectionsMap[sectionName] == nil {
		// section is missing, will include the values from configStruct in the JSON string
		sectionsMap[sectionName] = configStruct
		return json.MarshalIndent(sectionsMap, "", "  ")
	}

	sectionBytes, err := json.Marshal(configStruct)
	if err != nil {
		return []byte{}, err
	}
	section, ok := sectionsMap[sectionName].(map[string]interface{})
	if !ok {
		return []byte{}, fmt.Errorf("Section %s in config file does not contain an object but %T", sectionName, sectionsMap[sectionName])
	}
	err = json.Unmarshal(sectionBytes, &section)
	if err != nil {
		return []byte{}, err
	}

	return json.MarshalIndent(sectionsMap, "", "  ")
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

// GetSectionBytes returns the bytes of the section given by sectionName
func (c *ConfigReader) GetSectionBytes(sectionName string) ([]byte, error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	sectionsMap, err := GetSectionsMap(c.configFileContent)
	if err != nil {
		return []byte{}, err
	}
	if sectionsMap[sectionName] == nil {
		return []byte{}, nil
	}
	return json.Marshal(sectionsMap[sectionName])
}

// GetSectionNames returns the names of all sections in the config file
func (c *ConfigReader) GetSectionNames() ([]string, error) {
	c.lck.Lock()
	defer c.lck.Unlock()

	sectionsMap, err := GetSectionsMap(c.configFileContent)
	if err != nil {
		return []string{}, err
	}
	keys := make([]string, 0, len(sectionsMap))
	for k := range sectionsMap {
		keys = append(keys, k)
	}
	return keys, nil
}

func (c *ConfigReader) ReadSectionWithDefaults(sectionName string, configStruct interface{}) error {
	c.lck.Lock()
	defer c.lck.Unlock()

	newb, err := ReadSectionWithDefaults(c.configFileContent, sectionName, configStruct, path.Dir(c.configFilePath))
	if len(newb) > 0 {
		c.configChanged = true
		c.configFileContent = newb
	}
	return err
}

func (c *ConfigReader) WriteSection(sectionName string, configStruct interface{}, persistImmediately bool) error {
	c.lck.Lock()
	defer c.lck.Unlock()

	newb, err := WriteSection(c.configFileContent, sectionName, configStruct)
	if len(newb) > 0 {
		c.configChanged = true
		c.configFileContent = newb
		if persistImmediately {
			err = c.writeConfig()
		}
	}
	return err
}

func (c *ConfigReader) writeConfig() error {
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

func (c *ConfigReader) WriteBackConfigIfChanged() error {
	c.lck.Lock()
	defer c.lck.Unlock()

	if !c.configChanged {
		return nil
	}
	return c.writeConfig()
}
