package freepsstore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

// process-global store used by the Hook and the Operator
var store = Store{namespaces: map[string]StoreNamespace{}}

// StoreEntry contains data and metadata of a single entry
type StoreEntry struct {
	data       *base.OperatorIO
	timestamp  time.Time
	modifiedBy *base.Context
}

// ReadableStoreEntry is a StoreEntry with a more readable timestamp
type ReadableStoreEntry struct {
	Value      string
	RawValue   interface{}
	Age        string
	ModifiedBy string
	Reason     string
}

// NotFoundEntry is a StoreEntry with a 404 error
var NotFoundEntry = StoreEntry{base.MakeOutputError(http.StatusNotFound, "Key not found"), time.Unix(0, 0), nil}

// MakeEntryError creates a StoreEntry that contains an error
func MakeEntryError(code int, format string, args ...interface{}) StoreEntry {
	return StoreEntry{base.MakeOutputError(code, format, args...), time.Now(), nil}
}

// GetHumanReadable returns a readable version of the entry
func (v StoreEntry) GetHumanReadable() ReadableStoreEntry {
	id := ""
	reason := ""
	if v.modifiedBy != nil {
		id = v.modifiedBy.GetID()
		reason = v.modifiedBy.GetReason()
	}
	return ReadableStoreEntry{v.data.GetString(), v.data.Output, time.Now().Sub(v.timestamp).String(), id, reason}
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (v StoreEntry) MarshalJSON() ([]byte, error) {
	readable := v.GetHumanReadable()
	return json.Marshal(readable)
}

// GetData returns the data of the entry
func (v StoreEntry) GetData() *base.OperatorIO { return v.data }

// GetData returns the data of the entry
func (v StoreEntry) ParseJSON(obj interface{}) error {
	if v.data == nil {
		return fmt.Errorf("No Data")
	}
	if v.data.IsError() {
		return v.GetError()
	}
	return v.data.ParseJSON(obj)
}

// GetTimestamp returns the timestamp of the entry
func (v StoreEntry) GetTimestamp() time.Time { return v.timestamp }

// GetModifiedBy returns the modifiedBy of the entry
func (v StoreEntry) GetModifiedBy() string {
	if v.modifiedBy == nil {
		return ""
	}
	return v.modifiedBy.GetID()
}

// GetReason returns the modifiedBy of the entry
func (v StoreEntry) GetReason() string {
	if v.modifiedBy == nil {
		return ""
	}
	return v.modifiedBy.GetReason()
}

// IsError returns true if the entry contains an error
func (v StoreEntry) IsError() bool { return v.data != nil && v.data.IsError() }

// GetError returns an error, if StoreEntry contains one
func (v StoreEntry) GetError() error {
	if v.data != nil {
		return v.data.GetError()
	} else {
		return nil
	}
}

// StoreNamespace defines all functions to retrieve and modify data in the store
type StoreNamespace interface {
	CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy *base.Context) StoreEntry
	DeleteOlder(maxAge time.Duration) int
	Trim(k int) int
	DeleteValue(key string)
	GetAllValues(limit int) map[string]*base.OperatorIO
	GetKeys() []string
	Len() int
	GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry
	GetValue(key string) StoreEntry
	GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry
	OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy *base.Context) StoreEntry
	SetValue(key string, io *base.OperatorIO, modifiedBy *base.Context) StoreEntry
	SetAll(valueMap map[string]interface{}, modifiedBy *base.Context) *base.OperatorIO
	UpdateTransaction(key string, fn func(base.OperatorIO) *base.OperatorIO, modifiedBy *base.Context) *base.OperatorIO
}

// Store is a collection of different namespaces in which values can be stored
type Store struct {
	namespaces map[string]StoreNamespace
	globalLock sync.Mutex
	config     *StoreConfig
}

// GetNamespaceNoError from the store, create InMemoryNamespace if it does not exist
func (s *Store) GetNamespaceNoError(ns string) StoreNamespace {
	nsStore, err := s.GetNamespace(ns)
	if err != nil {
		panic(err)
	}
	return nsStore
}

// GetNamespaceNoError from the store, create InMemoryNamespace if it does not exist
func (s *Store) GetNamespace(ns string) (StoreNamespace, error) {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	nsStore, ok := s.namespaces[ns]
	if ok { // namespace exists or no config given (testing)
		return nsStore, nil
	}

	// create new namespace on the fly from config is there is one
	hasConfig := false
	var namespaceConfig StoreNamespaceConfig
	if s.config != nil { // may not be initialized in testing
		namespaceConfig, hasConfig = s.config.Namespaces[ns]
	}
	if !hasConfig || namespaceConfig.NamespaceType == "" {
		nsStore = &inMemoryStoreNamespace{entries: map[string]StoreEntry{}, nsLock: sync.Mutex{}}
	} else {
		var err error
		switch namespaceConfig.NamespaceType {
		case "files":
			nsStore, err = newFileStoreNamespace(namespaceConfig)
			if err != nil {
				return nil, fmt.Errorf("Cannot create store namespace \"%v\" of type \"%v\": %v", ns, namespaceConfig.NamespaceType, err)
			}
		case "postgres":
			if s.config.PostgresConnStr == "" {
				// fall back to memory store if there is no postgres connection defined
				nsStore = &inMemoryStoreNamespace{entries: map[string]StoreEntry{}, nsLock: sync.Mutex{}}
			}
			if db == nil {
				return nil, fmt.Errorf("Cannot create store namespace \"%v\" of type \"%v\": Postgres connection has not been established.", ns, namespaceConfig.NamespaceType)
			}
			nsStore, err = newPostgresStoreNamespace(ns, namespaceConfig)
			if err != nil {
				return nil, fmt.Errorf("Cannot create store namespace \"%v\" of type \"%v\": %v", ns, namespaceConfig.NamespaceType, err)
			}
		case "memory":
			nsStore = &inMemoryStoreNamespace{entries: map[string]StoreEntry{}, nsLock: sync.Mutex{}}
		case "log":
			nsStore = &logStoreNamespace{entries: []StoreEntry{}, offset: 0, nsLock: sync.Mutex{}, AutoTrim: namespaceConfig.AutoTrim}
		case "null":
			nsStore = &NullStoreNamespace{}
		default:
			return nil, fmt.Errorf("Cannot create store namespace \"%v\", type \"%v\" is unknown", ns, namespaceConfig.NamespaceType)
		}
	}

	s.namespaces[ns] = nsStore
	return nsStore, nil
}

// GetNamespaces returns all namespaces
func (s *Store) GetNamespaces() []string {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	ns := []string{}
	for n := range s.namespaces {
		ns = append(ns, n)
	}
	for n := range s.config.Namespaces {
		_, ok := s.namespaces[n]
		if !ok {
			ns = append(ns, n)
		}
	}
	return ns
}

// GetGlobalStore returns the store shared by everything in freeps
func GetGlobalStore() *Store {
	return &store
}

// GetFileStore returns the store for files
func GetFileStore() StoreNamespace {
	return store.namespaces["_files"]
}
