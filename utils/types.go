package utils

import (
	"bytes"
	"encoding/json"
)

type OutputModeT int

const (
	OutputModeIgnore = iota,
		OutputModeFirstNonEmpty
)

var toString = map[OutputModeT]string{
	OutputModeIgnore:        "Ignore",
	OutputModeFirstNonEmpty: "FirstNonEmpty",
}

var toID = map[string]OutputModeT{
	"Ignore":        OutputModeIgnore,
	"FirstNonEmpty": OutputModeFirstNonEmpty,
}

func (g OutputModeT) MarshalJSON() ([]byte, error) {
	buffer := bytes.NewBufferString(`"`)
	buffer.WriteString(toString[g])
	buffer.WriteString(`"`)
	return buffer.Bytes(), nil
}

func (g *OutputModeT) UnmarshalJSON(b []byte) error {
	var j string
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	*g = toID[j]
	return nil
}
