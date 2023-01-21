//go:build !nopostgress

package freepsstore

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"

	_ "github.com/lib/pq"
)

var db *sql.DB

func closingError(format string, a ...any) error {
	if db != nil {
		db.Close()
		db = nil
	}
	return fmt.Errorf(format, a...)
}

func initPostgresStores(cf *FreepsStoreConfig) error {
	var err error
	if db, err = sql.Open("postgres", cf.PostgresConnStr); err != nil {
		return closingError("init database connection: %v", err)
	}

	cf.PostgresSchema = utils.StringToIdentifier(cf.PostgresSchema)
	if _, err = db.Exec("create schema if not exists " + cf.PostgresSchema); err != nil {
		return closingError("create schema: %v", err)
	}
	rows, err := db.Query("SELECT table_name FROM information_schema.tables WHERE table_schema = $1", cf.PostgresSchema)
	if err != nil {
		return closingError("query namespaces: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		ns := ""
		if err := rows.Scan(&ns); err != nil {
			return closingError("query namespaces: %v", err)
		}
		store.namespaces[ns] = newPostgresStoreNamespace(cf.PostgresSchema, ns)
	}
	if err := rows.Err(); err != nil {
		return closingError("query namespaces: %v", err)
	}
	cf.ExecutionLogName = utils.StringToIdentifier(cf.ExecutionLogName)
	if _, ok := store.namespaces[cf.ExecutionLogName]; !ok {
		err = createNewNamespace(cf, cf.ExecutionLogName)
		return err
	}

	return nil
}

func createNewNamespace(cf *FreepsStoreConfig, name string) error {
	name = utils.StringToIdentifier(name)
	if _, err := db.Exec(fmt.Sprintf("create table %s.%s (key text primary key, output_type text, content_type text, http_code smallint, value_bytes bytea default NULL, value_plain text default NULL, value_json json default NULL, modification_time timestamp with time zone default current_timestamp, modified_by text);", cf.PostgresSchema, name)); err != nil {
		return fmt.Errorf("create table: %v", err)
	}
	store.namespaces[name] = newPostgresStoreNamespace(cf.PostgresSchema, name)
	return nil
}

func newPostgresStoreNamespace(schema string, name string) *postgresStoreNamespace {
	ns := &postgresStoreNamespace{schema: schema, name: name}
	return ns
}

type postgresStoreNamespace struct {
	schema string
	name   string
}

var _ StoreNamespace = &postgresStoreNamespace{}

func (p *postgresStoreNamespace) CompareAndSwap(key string, expected string, newValue *freepsgraph.OperatorIO, modifiedBy string) *freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) DeleteValue(key string) {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetAllValues() map[string]*freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetKeys() []string {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetValue(key string) *freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) OverwriteValueIfOlder(key string, io *freepsgraph.OperatorIO, maxAge time.Duration, modifiedBy string) *freepsgraph.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) SetValue(key string, io *freepsgraph.OperatorIO, modifiedBy string) error {
	var execErr error
	insertStart := fmt.Sprintf("insert into %s.%s", p.schema, p.name)
	if io.IsEmpty() {
		_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by) values($1,$2,$3,$4,$5)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy)
	} else if io.IsPlain() {
		_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_plain) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, io.GetString())
	} else {
		b, err := io.GetBytes()
		if err != nil {
			return fmt.Errorf("cannot get bytes for insertion in postgres: %v", err)
		}
		if io.IsObject() {
			_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_json) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, b)
		} else {
			_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_bytes) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, b)
		}
	}
	if execErr != nil {
		return fmt.Errorf("error when inserting into postgres: %v", execErr)
	}
	return nil
}
