package utils

import (
	"encoding/json"
	"testing"

	"gotest.tools/v3/assert"
)

type tStruct struct {
	Foo  string
	Foo2 string
}

var defaultConfig = tStruct{"defaultfoo", "defaultfoo2"}

func TestStructToMap(t *testing.T) {
	tMap, err := StructToMap(&defaultConfig)
	assert.NilError(t, err)

	assert.Equal(t, tMap["Foo"], defaultConfig.Foo)
}

func TestMergeConfig(t *testing.T) {

	myConfig := defaultConfig
	configFileContent := map[string]string{"Foo": "myfoo"}
	bytes, err := json.Marshal(configFileContent)
	assert.NilError(t, err)
	MergeJsonWithDefaults(bytes, &myConfig)

	assert.Equal(t, myConfig.Foo, "myfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")

	myConfig = defaultConfig
	configFileContent = map[string]string{"ignorethis": "notrelevant"}
	bytes, err = json.Marshal(configFileContent)
	assert.NilError(t, err)
	MergeJsonWithDefaults(bytes, &myConfig)

	assert.Equal(t, myConfig.Foo, "defaultfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")
}

func TestReadMergeConfig(t *testing.T) {
	sectionContent := map[string]string{"Foo": "myfoo"}
	configFileContent := make(map[string]map[string]string)
	configFileContent["mysection"] = sectionContent
	configFileBytes, err := json.Marshal(configFileContent)
	assert.NilError(t, err)

	myConfig := defaultConfig
	newb, err := ReadSectionWithDefaults(configFileBytes, "mysection", &myConfig)
	assert.Equal(t, len(newb), 0)
	assert.Equal(t, myConfig.Foo, "myfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")

	myConfig = defaultConfig
	newb, err = ReadSectionWithDefaults(configFileBytes, "mynonexistingsection", &myConfig)
	assert.Assert(t, len(newb) > 0)
	assert.Equal(t, myConfig.Foo, "defaultfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")
	newConfigFileContent := make(map[string]map[string]string)
	json.Unmarshal(newb, &newConfigFileContent)
	assert.Equal(t, len(newConfigFileContent), 2)
	assert.Equal(t, newConfigFileContent["mysection"]["Foo"], "myfoo")
	_, ok := newConfigFileContent["mysection"]["Foo2"]
	assert.Equal(t, ok, false)
	assert.Equal(t, newConfigFileContent["mynonexistingsection"]["Foo"], "defaultfoo")
	assert.Equal(t, newConfigFileContent["mynonexistingsection"]["Foo2"], "defaultfoo2")
}
