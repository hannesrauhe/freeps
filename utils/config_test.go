package utils

import (
	"encoding/json"
	"io/ioutil"
	"os"
	"path"
	"testing"

	"gotest.tools/v3/assert"
)

type tStruct struct {
	Foo  string
	Foo2 string
}

var defaultConfig = tStruct{"defaultfoo", "defaultfoo2"}

func TestReadMergeConfig(t *testing.T) {
	sectionContent := map[string]string{"Foo": "myfoo"}
	configFileContent := make(map[string]map[string]string)
	configFileContent["mysection"] = sectionContent

	tmpFile, err := ioutil.TempFile(os.TempDir(), "freeps-")
	assert.NilError(t, err)
	defer os.Remove(tmpFile.Name())

	externalStuff := tStruct{Foo: "external", Foo2: "external"}
	c, err := json.Marshal(externalStuff)
	assert.NilError(t, err)
	_, err = tmpFile.Write(c)
	assert.NilError(t, err)
	configFileContent["myAbsoluteIncludedSection"] = map[string]string{"Include": tmpFile.Name(), "Foo2": "configFileContent"}
	configFileContent["myRelativeIncludedSection"] = map[string]string{"Include": path.Base(tmpFile.Name())}

	configFileBytes, err := json.Marshal(configFileContent)
	assert.NilError(t, err)

	myConfig := defaultConfig
	newb, err := ReadSectionWithDefaults(configFileBytes, "mysection", &myConfig, "")
	assert.NilError(t, err)
	assert.Equal(t, len(newb), 0)
	assert.Equal(t, myConfig.Foo, "myfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")

	myConfig = defaultConfig
	newb, err = ReadSectionWithDefaults(configFileBytes, "mynonexistingsection", &myConfig, "")
	assert.NilError(t, err)
	assert.Assert(t, len(newb) > 0)
	assert.Equal(t, myConfig.Foo, "defaultfoo")
	assert.Equal(t, myConfig.Foo2, "defaultfoo2")
	newConfigFileContent := make(map[string]map[string]string)
	json.Unmarshal(newb, &newConfigFileContent)
	assert.Equal(t, len(newConfigFileContent), 4)
	assert.Equal(t, newConfigFileContent["mysection"]["Foo"], "myfoo")
	_, ok := newConfigFileContent["mysection"]["Foo2"]
	assert.Equal(t, ok, false)
	assert.Equal(t, newConfigFileContent["mynonexistingsection"]["Foo"], "defaultfoo")
	assert.Equal(t, newConfigFileContent["mynonexistingsection"]["Foo2"], "defaultfoo2")

	myConfig = defaultConfig
	newb, err = ReadSectionWithDefaults(configFileBytes, "myAbsoluteIncludedSection", &myConfig, "")
	assert.NilError(t, err)
	assert.Equal(t, len(newb), 0)
	assert.Equal(t, myConfig.Foo, "external")
	assert.Equal(t, myConfig.Foo2, "configFileContent")

	myConfig = defaultConfig
	newb, err = ReadSectionWithDefaults(configFileBytes, "myRelativeIncludedSection", &myConfig, path.Dir(tmpFile.Name()))
	assert.NilError(t, err)
	assert.Equal(t, len(newb), 0)
	assert.Equal(t, myConfig.Foo, "external")
	assert.Equal(t, myConfig.Foo2, "external")

	myConfig.Foo2 = "overwritenInConfig"

	newConfigBytes, err := WriteSection(configFileBytes, "myRelativeIncludedSection", myConfig)
	assert.NilError(t, err)
	assert.Assert(t, len(newConfigBytes) > 0)

	myConfig = defaultConfig
	newb, err = ReadSectionWithDefaults(newConfigBytes, "myRelativeIncludedSection", &myConfig, path.Dir(tmpFile.Name()))
	assert.NilError(t, err)
	assert.Equal(t, len(newb), 0)
	assert.Equal(t, myConfig.Foo, "external")
	assert.Equal(t, myConfig.Foo2, "overwritenInConfig")
}
