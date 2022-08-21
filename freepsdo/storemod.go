package freepsdo

import (
	"encoding/json"
	"net/http"
	"sync"
	"time"
)

type StoreEntry struct {
	Value         interface{}
	StoredAt      time.Time
	RetentionTime time.Duration
}

type StoreMod struct {
	store map[string]map[string]StoreEntry
	lck   sync.Mutex
}

var _ Mod = &StoreMod{}

func NewStoreMod() *StoreMod {
	s := make(map[string]map[string]StoreEntry)
	s[""] = make(map[string]StoreEntry)
	return &StoreMod{store: s}
}

type StoreArgs struct {
	Namespace     string
	Key           string
	RetentionTime string
	Value         interface{}
}

func (m *StoreMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	var args StoreArgs
	err := json.Unmarshal(jsonStr, &args)
	if err != nil {
		jrw.WriteError(http.StatusBadRequest, "request cannot be parsed into a map: %v", err)
		return
	}

	m.lck.Lock()
	defer m.lck.Unlock()

	switch fn {
	case "store":
		fallthrough
	case "set":
		retTime := time.Duration(0)
		if args.RetentionTime != "" {
			retTime, err = time.ParseDuration(args.RetentionTime)
			if err != nil {
				jrw.WriteError(http.StatusBadRequest, "cannot parse retention time: %v", err)
				return
			}
		}
		m.store[args.Namespace][args.Key] = StoreEntry{Value: args.Value, StoredAt: time.Now(), RetentionTime: retTime}
		jrw.WriteSuccess()
	case "get":
		ns, exists := m.store[args.Namespace]
		if !exists {
			jrw.WriteError(http.StatusNotFound, "No such namespace \"%v\"", args.Namespace)
			return
		}
		entry, exists := ns[args.Key]
		if !exists {
			jrw.WriteError(http.StatusNotFound, "No such key \"%v\" in namespace \"%v\"", args.Key, args.Namespace)
			return
		}
		if entry.RetentionTime > 0 {
			expires := entry.StoredAt.Add(entry.RetentionTime)
			if time.Now().After(expires) {
				jrw.WriteError(http.StatusNotFound, "Key \"%v\" in namespace \"%v\" expired", args.Key, args.Namespace)
				return
			}
			args.RetentionTime = expires.Sub(time.Now()).String()
		}
		args.Value = entry.Value
		jrw.WriteSuccessMessage(args)
	default:
		jrw.WriteError(http.StatusBadRequest, "No such function \"%v\"", fn)
	}
}

func (m *StoreMod) GetFunctions() []string {
	return []string{"get", "store", "set"}
}

func (m *StoreMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *StoreMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	switch arg {
	case "retention":
		return map[string]string{}
	case "key":
		return map[string]string{}
	case "value":
		return map[string]string{}
	}
	return map[string]string{}
}
