package freepsstore

// StoreConfig contains all start-parameters for the store
type StoreConfig struct {
	PostgresConnStr        string // The full connection string to the postgres instance
	PostgresSchema         string // the schema to store namespace-tables in
	ExecutionLogInPostgres bool   // store the execution log in postgres if available
	ExecutionLogName       string // name of the namespace for the execution log
	GraphInfoName          string // name of the namespace for the execution log
	ErrorLogName           string // name of the namespace for the error log
	MaxErrorLogSize        int    // maximum number of entries in the error log
}

var defaultConfig = StoreConfig{PostgresConnStr: "", PostgresSchema: "freepsstore", ExecutionLogInPostgres: true, ExecutionLogName: "execution_log", GraphInfoName: "_graph_info", ErrorLogName: "_error_log", MaxErrorLogSize: 1000}
