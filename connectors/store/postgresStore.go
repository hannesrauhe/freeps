//go:build !nopostgress

package freepsstore

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"math"
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
	qlog   StoreNamespace
	schema string
	name   string
}

var _ StoreNamespace = &postgresStoreNamespace{}

func (p *postgresStoreNamespace) query(limit int, projection string, filter string, args ...any) (*sql.Rows, error) {
	if filter == "" {
		filter = "1=1"
	}
	queryString := fmt.Sprintf("select %v from %v.%v where %v order by modification_time desc limit %d", projection, p.schema, p.name, filter, limit)

	// if p.qlog == nil {
	// 	p.qlog = store.GetNamespace("_postgres_query_log")
	// }
	// if p.qlog != nil {
	// 	p.qlog.SetValue(time.Now().Format("2006/01/02 15:04:05.00000"), base.MakePlainOutput("query: %v", queryString), "postgresStoreNamespace.query")
	// }
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
	r := p.GetSearchResultWithMetadata(keyPattern, valuePattern, modifiedByPattern, minAge, maxAge)
	for k, v := range r {
		result[k] = v.data
	}
	return result
}

func (p *postgresStoreNamespace) GetAllValues(limit int) map[string]*base.OperatorIO {
	panic("not implemented") // TODO: Implement
}

func (p *postgresStoreNamespace) GetKeys() []string {
	res := []string{}
	rows, err := p.query(100, "key", "")
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
	result := map[string]StoreEntry{}
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
	if maxAge != time.Duration(math.MaxInt64) {
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

	rows, err := p.query(100, "key, http_code, output_type, content_type, value_plain, value_bytes, value_json, modified_by, modification_time", filter, filterParts...)
	if err != nil {
		e := StoreEntry{
			timestamp: time.Now(),
			data:      base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err),
		}
		result["error"] = e
		return result
	}
	defer rows.Close()
	for rows.Next() {
		key := ""
		e := StoreEntry{
			data: &base.OperatorIO{},
		}
		var valuePlain sql.NullString
		var valueBytes, valueJSON []byte
		if err := rows.Scan(&key, &e.data.HTTPCode, &e.data.OutputType, &e.data.ContentType, &valuePlain, &valueBytes, &valueJSON, &e.modifiedBy, &e.timestamp); err != nil {
			result["error"] = StoreEntry{
				timestamp: time.Now(),
				data:      base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err),
			}
			return result
		}
		p.entryToOutput(e.data, valuePlain, valueBytes, valueJSON)
		result[key] = e
	}
	if err := rows.Err(); err != nil {
		result["error"] = StoreEntry{
			timestamp: time.Now(),
			data:      base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err),
		}
	}
	return result
}

func (p *postgresStoreNamespace) GetValue(key string) StoreEntry {
	e := StoreEntry{
		timestamp: time.Now(),
		data:      &base.OperatorIO{},
	}
	rows, err := p.query(1, "key, http_code, output_type, content_type, value_plain, value_bytes, value_json, modified_by, modification_time", "key=$1", key)
	if err != nil {
		e.data = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		return e
	}
	defer rows.Close()
	for rows.Next() {
		output := base.OperatorIO{}
		var valuePlain sql.NullString
		var valueBytes, valueJSON []byte
		if err := rows.Scan(&output.HTTPCode, &output.OutputType, &output.ContentType, &valuePlain, &valueBytes, &valueJSON, &e.modifiedBy, &e.timestamp); err != nil {
			e.data = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		}
		p.entryToOutput(e.data, valuePlain, valueBytes, valueJSON)
		return e
	}
	if err := rows.Err(); err != nil {
		e.data = base.MakeOutputError(http.StatusInternalServerError, "getValue: %v", err)
		return e
	}
	return NotFoundEntry
}

func (p *postgresStoreNamespace) GetValueBeforeExpiration(key string, maxAge time.Duration) StoreEntry {
	return StoreEntry{
		timestamp: time.Now(),
		data:      base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet"),
	}
}

func (p *postgresStoreNamespace) OverwriteValueIfOlder(key string, io *base.OperatorIO, maxAge time.Duration, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

func (p *postgresStoreNamespace) SetValue(key string, io *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	var execErr error
	insertStart := fmt.Sprintf("insert into %s.%s", p.schema, p.name)
	if io.IsEmpty() {
		_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by) values($1,$2,$3,$4,$5)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy)
	} else if io.IsPlain() {
		_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_plain) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, io.GetString())
	} else {
		b, err := io.GetBytes()
		if err != nil {
			base.MakeOutputError(http.StatusInternalServerError, "cannot get bytes for insertion in postgres: %v", err)
		}
		if io.IsObject() {
			_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_json) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, b)
		} else {
			_, execErr = db.Exec(insertStart+"(key, output_type, content_type, http_code, modified_by, value_bytes) values($1,$2,$3,$4,$5,$6)", key, io.OutputType, io.ContentType, io.HTTPCode, modifiedBy, b)
		}
	}
	if execErr != nil {
		return base.MakeOutputError(http.StatusInternalServerError, "error when inserting into postgres: %v", execErr)
	}
	return io
}

func (p *postgresStoreNamespace) SetAll(valueMap map[string]interface{}, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

func (p *postgresStoreNamespace) UpdateTransaction(key string, fn func(*base.OperatorIO) *base.OperatorIO, modifiedBy string) *base.OperatorIO {
	return base.MakeOutputError(http.StatusNotImplemented, "postgres support not fully implemented yet")
}

// Len returns the number of entries in the namespace
func (p *postgresStoreNamespace) Len() int {
	var count int
	err := db.QueryRow("select count(*) from " + p.schema + "." + p.name).Scan(&count)
	if err != nil {
		return -1
	}
	return count
}
