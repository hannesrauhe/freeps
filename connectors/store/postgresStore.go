//go:build !nopostgress

package freepsstore

import (
	"database/sql"
	"fmt"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"

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

func (s *Store) initPostgresStores() error {
	var err error
	if db, err = sql.Open("postgres", s.config.PostgresConnStr); err != nil {
		return closingError("init database connection: %v", err)
	}

	s.config.PostgresSchema = utils.StringToIdentifier(s.config.PostgresSchema)
	if _, err = db.Exec("create schema if not exists " + s.config.PostgresSchema); err != nil {
		return closingError("create schema: %v", err)
	}
	rows, err := db.Query("select table_name from information_schema.tables where table_schema = $1", s.config.PostgresSchema)
	if err != nil {
		return closingError("query namespaces: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		ns := ""
		if err := rows.Scan(&ns); err != nil {
			return closingError("query namespaces: %v", err)
		}
		store.namespaces[ns] = newPostgresStoreNamespace(s.config.PostgresSchema, ns)
	}
	if err := rows.Err(); err != nil {
		return closingError("query namespaces: %v", err)
	}
	s.config.ExecutionLogName = utils.StringToIdentifier(s.config.ExecutionLogName)
	if s.config.ExecutionLogInPostgres {
		if _, ok := store.namespaces[s.config.ExecutionLogName]; !ok {
			err = s.createPostgresNamespace(s.config.ExecutionLogName)
			return err
		}
	}

	return nil
}

func (s *Store) createPostgresNamespace(name string) error {
	if db == nil {
		return fmt.Errorf("No active postgres connection")
	}
	name = utils.StringToIdentifier(name)
	if _, err := db.Exec(fmt.Sprintf("create table %s.%s (key text primary key, output_type text, content_type text, http_code smallint, value_bytes bytea default NULL, value_plain text default NULL, value_json json default NULL, modification_time timestamp with time zone default current_timestamp, modified_by text);", s.config.PostgresSchema, name)); err != nil {
		return fmt.Errorf("create table: %v", err)
	}
	store.namespaces[name] = newPostgresStoreNamespace(s.config.PostgresSchema, name)
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

func (p *postgresStoreNamespace) query(projection string, filter string, args ...any) (*sql.Rows, error) {
	queryString := fmt.Sprintf("select %v from %v.%v where %v", projection, p.schema, p.name, filter)
	return db.Query(queryString, args...)
}

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
	res := []string{}
	rows, err := p.query("key", "1=1")
	if err != nil {
		return res
	}
	defer rows.Close()
	for rows.Next() {
		key := ""
		if err := rows.Scan(&key); err != nil {
			return res
		}
		res = append(res, key)
	}
	if err := rows.Err(); err != nil {
		return res
	}
	return res
}

func (p *postgresStoreNamespace) GetSearchResultWithMetadata(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]StoreEntry {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetValue(key string) *freepsgraph.OperatorIO {
	rows, err := p.query("http_code, output_type, content_type, value_plain, value_bytes, value_json", "key=$1", key)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		http_code := 0
		var output_type, content_type, value_plain sql.NullString
		var value_bytes, value_json []byte
		if err := rows.Scan(&http_code, &output_type, &content_type, &value_plain, &value_bytes, &value_json); err != nil {
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		}
		switch output_type.String {
		case "empty":
			return freepsgraph.MakeEmptyOutput()
		case "plain":
			return freepsgraph.MakePlainOutput(value_plain.String)
		case "byte":
			return freepsgraph.MakeByteOutputWithContentType(value_bytes, content_type.String)
		case "object":
			return freepsgraph.MakeByteOutputWithContentType(value_bytes, content_type.String)
		default:
			return freepsgraph.MakeOutputError(http.StatusInternalServerError, "getValue: invalid object in db: %v", err)
		}
		logrus.Print(http_code)
	}
	if err := rows.Err(); err != nil {
		return freepsgraph.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
	}
	return freepsgraph.MakeOutputError(http.StatusNotFound, "Key not found")
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
