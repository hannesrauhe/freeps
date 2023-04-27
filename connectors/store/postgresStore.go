//go:build !nopostgress

package freepsstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
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

func (s *Store) initPostgresStores() error {
	var err error
	if db, err = sql.Open("postgres", s.config.PostgresConnStr); err != nil {
		return closingError("init database connection: %v", err)
	}

	s.config.PostgresSchema = utils.StringToIdentifier(s.config.PostgresSchema)
	if _, err = db.Exec("create schema if not exists " + s.config.PostgresSchema); err != nil {
		return closingError("create schema: %v", err)
	}

	s.config.ExecutionLogName = utils.StringToIdentifier(s.config.ExecutionLogName)

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
		if ns == s.config.ExecutionLogName && !s.config.ExecutionLogInPostgres {
			// skip the execution log namespace if it was disabled by the user
			continue
		}
		store.namespaces[ns] = newPostgresStoreNamespace(s.config.PostgresSchema, ns)
	}
	if err := rows.Err(); err != nil {
		return closingError("query namespaces: %v", err)
	}

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
	if _, err := db.Exec(fmt.Sprintf("create table %s.%s (key text primary key, output_type text not null, content_type text not null, http_code smallint not null, value_bytes bytea default NULL, value_plain text default NULL, value_json json default NULL, modification_time timestamp with time zone default current_timestamp not null, modified_by text not null);", s.config.PostgresSchema, name)); err != nil {
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
	if filter == "" {
		filter = "1=1"
	}
	// TODO(HR): remove hard-coded limit of 100 rows
	queryString := fmt.Sprintf("select %v from %v.%v where %v limit %d", projection, p.schema, p.name, filter, 100)
	return db.Query(queryString, args...)
}

func (p *postgresStoreNamespace) entryToOutput(output *base.OperatorIO, valuePlain sql.NullString, valueBytes []byte, valueJSON []byte) {
	switch output.OutputType {
	case base.Empty:
		output.Output = nil
	case base.PlainText:
		if !valuePlain.Valid {
			*output = *base.MakeOutputError(http.StatusInternalServerError, "getValue: invalid object in db: plain value is NULL")
		}
		output.Output = valuePlain.String
	case base.Byte:
		output.Output = valueBytes
	case base.Object:
		output.Output = map[string]interface{}{}
		json.Unmarshal(valueJSON, &output.Output)
	default:
		*output = *base.MakeOutputError(http.StatusInternalServerError, "getValue: invalid object in db: OutputType unkown")
	}
}

func (p *postgresStoreNamespace) CompareAndSwap(key string, expected string, newValue *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

func (p *postgresStoreNamespace) DeleteOlder(maxAge time.Duration) int {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) DeleteValue(key string) {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetAllFiltered(keyPattern string, valuePattern string, modifiedByPattern string, minAge time.Duration, maxAge time.Duration) map[string]*base.OperatorIO {
	result := map[string]*base.OperatorIO{}
	filter := ""
	filterParts := []any{}
	if keyPattern != "" {
		filter = "key LIKE $1"
		filterParts = append(filterParts, keyPattern)
	}
	if modifiedByPattern != "" {
		if filter != "" {
			filter += " AND "
		}
		filter += fmt.Sprintf("modified_by LIKE $%d", len(filterParts)+1)
		filterParts = append(filterParts, modifiedByPattern)
	}
	if minAge != 0 {
		if filter != "" {
			filter += " AND "
		}
		filter += fmt.Sprintf("modification_time < now() - interval '%v'", minAge.String())
	}
	if maxAge != 0 {
		if filter != "" {
			filter += " AND "
		}
		filter += fmt.Sprintf("modification_time > now() - interval '%v'", maxAge.String())
	}
	// TODO(HR): meh
	// if valuePattern != "" {
	// 	if len(filterParts) > 0 {
	// 		filter += " AND "
	// 	}
	// 	filter += fmt.Sprintf("value_plain LIKE $%d or value_bytes as string LIKE $%d or value_json as string LIKE $%d", len(filterParts)+1, len(filterParts)+1, len(filterParts)+1)
	// 	filterParts = append(filterParts, valuePattern)
	// }

	rows, err := p.query("key, http_code, output_type, content_type, value_plain, value_bytes, value_json", filter, filterParts...)
	if err != nil {
		result["error"] = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		return result
	}
	defer rows.Close()
	for rows.Next() {
		key := ""
		output := base.OperatorIO{}
		var valuePlain sql.NullString
		var valueBytes, valueJSON []byte
		if err := rows.Scan(&key, &output.HTTPCode, &output.OutputType, &output.ContentType, &valuePlain, &valueBytes, &valueJSON); err != nil {
			result["error"] = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
			return result
		}
		p.entryToOutput(&output, valuePlain, valueBytes, valueJSON)
		result[key] = &output
	}
	if err := rows.Err(); err != nil {
		result["error"] = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
	}
	return result
}

func (p *postgresStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetKeys() []string {
	res := []string{}
	rows, err := p.query("key", "")
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

func (p *postgresStoreNamespace) GetValue(key string) *base.OperatorIO {
	rows, err := p.query("http_code, output_type, content_type, value_plain, value_bytes, value_json", "key=$1", key)
	if err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
	}
	defer rows.Close()
	for rows.Next() {
		output := base.OperatorIO{}
		var valuePlain sql.NullString
		var valueBytes, valueJSON []byte
		if err := rows.Scan(&output.HTTPCode, &output.OutputType, &output.ContentType, &valuePlain, &valueBytes, &valueJSON); err != nil {
			return base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		}
		p.entryToOutput(&output, valuePlain, valueBytes, valueJSON)
		return &output
	}
	if err := rows.Err(); err != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
	}
	return base.MakeOutputError(http.StatusNotFound, "Key not found")
}

func (p *postgresStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

func (p *postgresStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

func (p *postgresStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) error {
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
