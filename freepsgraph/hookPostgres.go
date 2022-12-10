package freepsgraph

import (
	"database/sql"

	"github.com/hannesrauhe/freeps/utils"

	_ "github.com/lib/pq"
)

type FreepsPostgressConfig struct {
	ConnStr string // The full connection string to the postgres instance
}

type HookPostgres struct {
	db *sql.DB
}

var defaultConfig = FreepsPostgressConfig{ConnStr: "host=host port=5432 user=user password=pass dbname=db sslmode=require"}

var _ FreepsHook = &HookPostgres{}

// NewPostgressHook creates a new Postgress Hook
func NewPostgressHook(cr *utils.ConfigReader) (*HookPostgres, error) {
	phc := defaultConfig
	err := cr.ReadSectionWithDefaults("postgress", &phc)

	db, err := sql.Open("postgres", phc.ConnStr)
	if err != nil {
		return nil, err
	}
	return &HookPostgres{db: db}, nil
}

// OnExecute gets called when freepsgraph starts executing a Graph
func (h *HookPostgres) OnExecute(ctx *utils.Context, graphName string, mainArgs map[string]string, mainInput *OperatorIO) error {
	stmt, err := h.db.Prepare(`INSERT INTO "system".graph_execution_log	(graph_name, uuid) VALUES($1, $2);`)
	if err != nil {
		return err
	}
	_, err = stmt.Exec(graphName, ctx.GetID())
	if err != nil {
		return err
	}
	err = stmt.Close()
	if err != nil {
		return err
	}
	return nil
}

// Shutdown gets called on graceful shutdown
func (h *HookPostgres) Shutdown() error {
	return h.db.Close()
}
