package freepsdo

import (
	"encoding/json"
	"fmt"
)

type OutputModeT int

const (
	OutputModeFirstNonEmpty = iota
	OutputModeIgnore
)

func (g OutputModeT) MarshalJSON() ([]byte, error) {
	var s string
	switch g {
	case OutputModeIgnore:
		s = "Ignore"
	case OutputModeFirstNonEmpty:
		s = "FirstNonEmpty"
	default:
		return nil, fmt.Errorf("Unknown OutputMode")
	}
	return []byte(s), nil
}

func (g *OutputModeT) UnmarshalJSON(b []byte) error {
	var j string
	var err error
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	switch j {
	case "Ignore":
		*g = OutputModeIgnore
	case "FirstNonEmpty":
		*g = OutputModeFirstNonEmpty
	default:
		return fmt.Errorf("Unknown OutputMode")
	}
	return err
}

type ResponseType int

const (
	ResponseTypeNone = iota
	ResponseTypePlainText
	ResponseTypeJSON
	ResponseTypeJPEG
)

func (g ResponseType) ToString() (string, error) {
	var s string
	switch g {
	case ResponseTypeNone:
		s = "none"
	case ResponseTypePlainText:
		s = "text/plain"
	case ResponseTypeJSON:
		s = "application/json"
	case ResponseTypeJPEG:
		s = "image/jpeg"
	default:
		return "", fmt.Errorf("Unknown ResponseType")
	}
	return s, nil
}

func (g ResponseType) MarshalJSON() ([]byte, error) {
	s, err := g.ToString()
	return []byte(s), err
}

func (g *ResponseType) UnmarshalJSON(b []byte) error {
	var j string
	var err error
	if err := json.Unmarshal(b, &j); err != nil {
		return err
	}
	switch j {
	case "none":
		*g = ResponseTypePlainText
	case "text/plain":
		*g = ResponseTypePlainText
	case "application/json":
		*g = ResponseTypeJSON
	case "image/jpeg":
		*g = ResponseTypeJPEG
	default:
		return fmt.Errorf("Unknown ResponseType")
	}
	return err
}
