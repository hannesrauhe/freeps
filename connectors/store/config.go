package freepsstore

import "os"

// StoreConfig contains all start-parameters for the store
type StoreConfig struct {
	PostgresConnStr        string // The full connection string to the postgres instance
	PostgresSchema         string // the schema to store namespace-tables in
	ExecutionLogInPostgres bool   // store the execution log in postgres if available
	ExecutionLogName       string // name of the namespace for the execution log
	GraphInfoName          string // name of the namespace for the execution log
	ErrorLogName           string // name of the namespace for the error log
	OperatorInfoName       string // name of the namespace for the operator info log
	MaxErrorLogSize        int    // maximum number of entries in the error log
}

func getDefaultConfig() StoreConfig {
	// get the hostname of this computer
	hostname, err := os.Hostname()
	if err != nil {
		panic("could not get hostname")
	}
	return StoreConfig{PostgresConnStr: "", PostgresSchema: "freeps_" + hostname, ExecutionLogInPostgres: true, ExecutionLogName: "_execution_log", GraphInfoName: "_graph_info", ErrorLogName: "_error_log", OperatorInfoName: "_operator_info", MaxErrorLogSize: 1000}
}
