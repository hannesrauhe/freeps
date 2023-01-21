package freepsstore

import (
	"encoding/json"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
)

// process-global store used by the Hook and the Operator
var store = Store{namespaces: map[string]StoreNamespace{}}

// StoreEntry contains data and metadata of a single entry
type StoreEntry struct {
	data       *freepsgraph.OperatorIO
	timestamp  time.Time
	modifiedBy string
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (v *StoreEntry) MarshalJSON() ([]byte, error) {
	readable := struct {
		Value      string
		Age        string
		ModifiedBy string
	}{v.data.GetString(), time.Now().Sub(v.timestamp).String(), v.modifiedBy}
	return json.Marshal(readable)
}

// StoreNamespace defines all functions to retrieve and modify data in the store
type StoreNamespace interface {
	CompareAndSwap(key string, expected string, newValue *freepsgraph.OperatorIO, modifiedBy string) *freepsgraph.OperatorIO
	DeleteOlder(maxAge time.Duration) int
	DeleteValue(key string)
	GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*freepsgraph.OperatorIO
	GetAllValues() map[string]*freepsgraph.OperatorIO
	GetKeys() []string
	GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry
	GetValue(key string) *freepsgraph.OperatorIO
	GetValueBeforeExpiration(key string, maxAge time.Duration) *freepsgraph.OperatorIO
	OverwriteValueIfOlder(key string, io *freepsgraph.OperatorIO, maxAge time.Duration, modifiedBy string) *freepsgraph.OperatorIO
	SetValue(key string, io *freepsgraph.OperatorIO, modifiedBy string)
}

// Store is a collection of different namespaces in which values can be stored
type Store struct {
	namespaces map[string]StoreNamespace
	globalLock sync.Mutex
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
