package freepsgraph

import (
	"testing"

	"gotest.tools/v3/assert"
)

func TestConn(t *testing.T) {
	t.Skip("Needs local db")
	vars := map[string]string{"host": "localhost", "port": "5432", "user": "postgres", "password": "test", "dbname": "freeps"}
	input := MakePlainOutput("test_value")

	s := NewPostgresOp()
	out := s.Execute("query", vars, input)
	assert.Assert(t, out != nil)
	assert.Assert(t, !out.IsError(), "Unexpected error when trying to connect to postgres", out)
}
