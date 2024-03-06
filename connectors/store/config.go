package freepsstore

import (
	"os"

	"github.com/hannesrauhe/freeps/utils"
)

var fileNamespace = "_files"
var debugNamespace = "_debug"
var executionLogNamespace = "_execution_log"
var errorNamespace = "_error_log"

// StoreConfig contains all start-parameters for the store
type StoreConfig struct {
	Namespaces      map[string]StoreNamespaceConfig
	PostgresConnStr string // The full connection string to the postgres instance
	MaxErrorLogSize int    // maximum number of entries in the error log
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
	namespaces[fileNamespace] = StoreNamespaceConfig{
		NamespaceType: "files",
	}

	// get the hostname of this computer
	hostname, err := os.Hostname()
	if err != nil {
		panic("could not get hostname")
	}
	namespaces[executionLogNamespace] = StoreNamespaceConfig{
		NamespaceType: "postgres",
		SchemaName:    "freeps_" + utils.StringToIdentifier(hostname),
		TableName:     "_execution_log",
	}
	namespaces[errorNamespace] = StoreNamespaceConfig{
		NamespaceType: "log",
	}
	namespaces[debugNamespace] = StoreNamespaceConfig{
		NamespaceType: "memory",
	}
	return namespaces
}
