//go:build nopostgress

package freepsstore

import (
	"fmt"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"time"
)

type dummydb struct{}

var db *dummydb = nil

func (s *Store) initPostgresStores() error {
	panic("postgres support not compiled, method should not be called")
}

func (s *Store) createPostgresNamespace(name string) error {
	return fmt.Errorf("Postgres support not available")
}

type postgresStoreNamespace struct {
}

var _ StoreNamespace = &postgresStoreNamespace{}

func (p *postgresStoreNamespace) CompareAndSwap(key string, expected string, newValue *freepsgraph.OperatorIO, modifiedBy string) *freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) DeleteValue(key string) {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetAllValues() map[string]*freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetKeys() []string {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetValue(key string) *freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) OverwriteValueIfOlder(key string, io *freepsgraph.OperatorIO, maxAge time.Duration, modifiedBy string) *freepsgraph.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) SetValue(key string, io *freepsgraph.OperatorIO, modifiedBy string) error {
	panic("postgres support not compiled, method should not be called")
}
