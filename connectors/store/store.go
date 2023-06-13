package freepsstore

import (
	"encoding/json"
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
	modifiedBy string
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (v StoreEntry) MarshalJSON() ([]byte, error) {
	readable := struct {
		Value      string
		Age        string
		ModifiedBy string
	}{v.data.GetString(), time.Now().Sub(v.timestamp).String(), v.modifiedBy}
	return json.Marshal(readable)
}

// GetData returns the data of the entry
func (v StoreEntry) GetData() *base.OperatorIO { return v.data }

// GetTimestamp returns the timestamp of the entry
func (v StoreEntry) GetTimestamp() time.Time { return v.timestamp }

// GetModifiedBy returns the modifiedBy of the entry
func (v StoreEntry) GetModifiedBy() string { return v.modifiedBy }

// StoreNamespace defines all functions to retrieve and modify data in the store
type StoreNamespace interface {
	CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO
	DeleteOlder(maxAge time.Duration) int
	DeleteValue(key string)
	GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*base.OperatorIO
	GetAllValues(limit int) map[string]*base.OperatorIO
	GetKeys() []string
	Len() int
	GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry
	GetValue(key string) *base.OperatorIO
	GetValueBeforeExpiration(key string, maxAge time.Duration) *base.OperatorIO
	OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO
	SetValue(key string, io *base.OperatorIO, modifiedBy string) *base.OperatorIO
	SetAll(valueMap map[string]interface{}, modifiedBy string) *base.OperatorIO
	UpdateTransaction(key string, fn func(*base.OperatorIO) *base.OperatorIO, modifiedBy string) *base.OperatorIO
}

// Store is a collection of different namespaces in which values can be stored
type Store struct {
	namespaces map[string]StoreNamespace
	globalLock sync.Mutex
	config     *StoreConfig
}

// GetNamespace from the store, create InMemoryNamespace if it does not exist
func (s *Store) GetNamespace(ns string) StoreNamespace {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	nsStore, ok := s.namespaces[ns]
	if !ok {
		nsStore = &inMemoryStoreNamespace{entries: map[string]StoreEntry{}, nsLock: sync.Mutex{}}
		s.namespaces[ns] = nsStore
	}
	return nsStore
}

// GetNamespaces returns all namespaces
func (s *Store) GetNamespaces() []string {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	ns := []string{}
	for n := range s.namespaces {
		ns = append(ns, n)
	}
	return ns
}

// GetGlobalStore returns the store shared by everything in freeps
func GetGlobalStore() *Store {
	return &store
}
