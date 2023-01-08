package freepsstore

import (
	"net/http"
	"sync"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
)

type StoreNamespace struct {
	data       map[string]*freepsgraph.OperatorIO
	timestamps map[string]time.Time
	nsLock     sync.Mutex
}

type InMemoryStore struct {
	namespaces map[string]*StoreNamespace
	globalLock sync.Mutex
}

var store = InMemoryStore{namespaces: map[string]*StoreNamespace{}}

// GetNamespace from the store, create if it does not exist
func (s *InMemoryStore) GetNamespace(ns string) *StoreNamespace {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	nsStore, ok := s.namespaces[ns]
	if !ok {
		nsStore = &StoreNamespace{data: map[string]*freepsgraph.OperatorIO{}, timestamps: map[string]time.Time{}, nsLock: sync.Mutex{}}
		s.namespaces[ns] = nsStore
	}
	return nsStore
}

// GetNamespaces returns all namespaces
func (s *InMemoryStore) GetNamespaces() []string {
	s.globalLock.Lock()
	defer s.globalLock.Unlock()
	ns := []string{}
	for n := range s.namespaces {
		ns = append(ns, n)
	}
	return ns
}

// GetValue from the StoreNamespace
func (s *StoreNamespace) GetValue(key string) *freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	io, ok := s.data[key]
	if !ok {
		return freepsgraph.MakeOutputError(http.StatusNotFound, "Key not found")
	}
	return io
}

// GetValueBeforeExpiration gets the value from the StoreNamespace, but returns error if older than maxAge
func (s *StoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	io, ok := s.data[key]
	if !ok {
		return freepsgraph.MakeOutputError(http.StatusNotFound, "Key not found")
	}
	ts, ok := s.timestamps[key]
	if !ok {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "no timestamp for key")
	}
	if ts.Add(maxAge).Before(time.Now()) {
		return freepsgraph.MakeOutputError(http.StatusGone, "key is older than %v", maxAge)
	}
	return io
}

func (s *StoreNamespace) setValueUnlocked(key string, newValue *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	s.data[key] = newValue
	s.timestamps[key] = time.Now()
	return freepsgraph.MakeEmptyOutput()
}

// SetValue in the StoreNamespace
func (s *StoreNamespace) SetValue(key string, io *freepsgraph.OperatorIO) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	s.setValueUnlocked(key, io)
}

// CompareAndSwap sets the value if the string representation of the already stored value is as expected
func (s *StoreNamespace) CompareAndSwap(key string, expected string, newValue *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	oldV, exists := s.data[key]
	if !exists {
		return freepsgraph.MakeOutputError(http.StatusNotFound, "key does not exist yet")
	}
	if oldV == nil || oldV.GetString() != expected {
		return freepsgraph.MakeOutputError(http.StatusConflict, "old value is different from expectation")
	}
	return s.setValueUnlocked(key, newValue)
}

// OverwriteValueIfOlder sets the value only if the key does not exist or has been written before maxAge
func (s *StoreNamespace) OverwriteValueIfOlder(key string, io *freepsgraph.OperatorIO, maxAge time.Duration) *freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	n := time.Now()
	ts, keyExists := s.timestamps[key]
	if keyExists && ts.Add(maxAge).After(n) {
		return freepsgraph.MakeOutputError(http.StatusConflict, "%v already exists and is only %v old", key, n.Sub(ts))
	}
	return s.setValueUnlocked(key, io)
}

// DeleteValue from the StoreNamespace
func (s *StoreNamespace) DeleteValue(key string) {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	delete(s.data, key)
	delete(s.timestamps, key)
}

// GetKeys returns all keys in the StoreNamespace
func (s *StoreNamespace) GetKeys() []string {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	keys := []string{}
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

// GetAllValues from the StoreNamespace
func (s *StoreNamespace) GetAllValues() map[string]*freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	copy := map[string]*freepsgraph.OperatorIO{}
	for k, v := range s.data {
		copy[k] = v
	}
	return copy
}

// GetAllValuesBeforeExpiration gets the value from the StoreNamespace younger than maxAge
func (s *StoreNamespace) GetAllValuesBeforeExpiration(maxAge time.Duration) map[string]*freepsgraph.OperatorIO {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	copy := map[string]*freepsgraph.OperatorIO{}
	for k, v := range s.data {
		ts, ok := s.timestamps[k]
		if !ok {
			copy[k] = freepsgraph.MakeOutputError(http.StatusInternalServerError, "no timestamp for key")
		}
		if ts.Add(maxAge).After(tnow) {
			copy[k] = v
		}
	}
	return copy
}

// DeleteOlder deletes records older than maxAge
func (s *StoreNamespace) DeleteOlder(maxAge time.Duration) int {
	s.nsLock.Lock()
	defer s.nsLock.Unlock()
	tnow := time.Now()
	keys := []string{}
	for k, ts := range s.timestamps {
		if ts.Add(maxAge).Before(tnow) {
			delete(s.data, k)
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		delete(s.timestamps, k)
	}
	return len(keys)
}
