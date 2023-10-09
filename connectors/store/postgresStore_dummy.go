//go:build nopostgress

package freepsstore

import (
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/base"
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

func (p *postgresStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) DeleteValue(key string) {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetKeys() []string {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetValue(key string) StoreEntry {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) SetAll(valueMap map[string]interface{}, modifiedBy string) *base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) UpdateTransaction(key string, fn func(*base.OperatorIO) *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	panic("postgres support not compiled, method should not be called")
}

func (p *postgresStoreNamespace) Len() int {
	panic("postgres support not compiled, method should not be called")
}
