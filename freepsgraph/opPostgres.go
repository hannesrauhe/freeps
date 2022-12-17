package freepsgraph

import (
	"database/sql"
	"fmt"
	"net/http"

	"github.com/hannesrauhe/freeps/utils"
	_ "github.com/lib/pq"
)

// OpPostgres describes the postgres Op
type OpPostgres struct {
}

var _ FreepsOperator = &OpPostgres{}

// GetName returns the name of the operator
func (o *OpPostgres) GetName() string {
	return "postgres"
}

// NewPostgresOp creates a new Postgres Operator
func NewPostgresOp() *OpPostgres {
	return &OpPostgres{}
}

// Execute sends a query to the db specified by vars
func (o *OpPostgres) Execute(ctx *utils.Context, function string, vars map[string]string, mainInput *OperatorIO) *OperatorIO {
	psqlconn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=require", vars["host"], vars["port"], vars["user"], vars["password"], vars["dbname"])
	db, err := sql.Open("postgres", psqlconn)
	if err != nil {
		MakeOutputError(http.StatusBadRequest, err.Error())
	}

	defer db.Close()

	err = db.Ping()
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}

	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return MakeEmptyOutput()
}

// GetFunctions returns all availabble functions
func (o *OpPostgres) GetFunctions() []string {
	return []string{"query"}
}

// GetPossibleArgs returns required and optional args
func (o *OpPostgres) GetPossibleArgs(fn string) []string {
	return []string{"host", "port", "user", "password", "dbname"}
}

// GetArgSuggestions return suggestions for arguments based on other argumens
func (o *OpPostgres) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}

// Shutdown (noOp)
func (o *OpPostgres) Shutdown(ctx *utils.Context) {
}
