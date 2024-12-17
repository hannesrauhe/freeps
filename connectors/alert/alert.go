package opalert

import (
	"fmt"
	"time"
)

type Alert struct {
	Name              string
	Category          string
	Desc              *string `json:",omitempty"`
	Severity          int
	ExpiresInDuration *time.Duration `json:",omitempty"`
}

func getAlertName(name string, category string) string {
	if category == "" {
		return name
	}
	return fmt.Sprintf("%v.%v", category, name)
}

func (a *Alert) GetFullName() string {
	return getAlertName(a.Name, a.Category)
}

func (a *Alert) NameSuggestions(oc *OpAlert) map[string]string {
	return oc.nameSuggestions(&a.Category, false)
}

type AlertWithMetadata struct {
	Alert
	Counter      int
	First        time.Time
	Last         time.Time
	SilenceUntil time.Time
}

func (a *AlertWithMetadata) IsExpired() bool {
	if a.ExpiresInDuration == nil {
		return false
	}
	expiresAt := a.Last.Add(*a.ExpiresInDuration)
	return expiresAt.Before(time.Now())
}

func (a *AlertWithMetadata) IsSilenced() bool {
	return a.SilenceUntil.After(time.Now())
}

type ReadableAlert struct {
	Name               string
	Category           string
	Desc               string
	Severity           int
	ExpiresInDuration  time.Duration
	Counter            int
	DurationSinceFirst time.Duration
	DurationSinceLast  time.Duration
	SilenceDuration    time.Duration
	ModifiedBy         string
}

func (a *ReadableAlert) GetFullName() string {
	return getAlertName(a.Name, a.Category)
}

func NewReadableAlert(a AlertWithMetadata, modifiedBy string) ReadableAlert {
	r := ReadableAlert{Name: a.Name, Category: a.Category, Severity: a.Severity, Counter: a.Counter, ModifiedBy: modifiedBy}
	if a.Desc != nil {
		r.Desc = *a.Desc
	}
	if a.ExpiresInDuration != nil {
		r.ExpiresInDuration = *a.ExpiresInDuration
	}
	r.DurationSinceFirst = time.Now().Sub(a.First)
	r.DurationSinceLast = time.Now().Sub(a.Last)
	if a.SilenceUntil.After(time.Now()) {
		r.SilenceDuration = a.SilenceUntil.Sub(time.Now())
	}
	return r
}
