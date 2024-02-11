package freepsstore

import (
	"os"

	"github.com/hannesrauhe/freeps/utils"
)

// StoreConfig contains all start-parameters for the store
type StoreConfig struct {
	Namespaces       map[string]StoreNamespaceConfig
	PostgresConnStr  string // The full connection string to the postgres instance
	ExecutionLogName string // name of the namespace for the execution log
	GraphInfoName    string // name of the namespace for the execution log
	ErrorLogName     string // name of the namespace for the error log
	OperatorInfoName string // name of the namespace for the operator info log
	MaxErrorLogSize  int    // maximum number of entries in the error log
}

// StoreNamespaceConfig contains the configuration for a single namespace
type StoreNamespaceConfig struct {
	NamespaceType string

	/* files */
	Directory string `json:",omitempty"` // existing directory to store files in; use temp dir if empty

	/* postgres */
	SchemaName string `json:",omitempty"`
	TableName  string `json:",omitempty"`
}

func getDefaultNamespaces() map[string]StoreNamespaceConfig {
	namespaces := make(map[string]StoreNamespaceConfig)
	namespaces["_files"] = StoreNamespaceConfig{
		NamespaceType: "files",
	}

	// get the hostname of this computer
	hostname, err := os.Hostname()
	if err != nil {
		panic("could not get hostname")
	}
	namespaces["_execution_log"] = StoreNamespaceConfig{
		NamespaceType: "postgres",
		SchemaName:    "freeps_" + utils.StringToIdentifier(hostname),
		TableName:     "_execution_log",
	}
	namespaces["_error_log"] = StoreNamespaceConfig{
		NamespaceType: "log",
	}
	return namespaces
}
