package freepsgraph

import (
	"net/http"
	"regexp"

	"github.com/hannesrauhe/freeps/base"
)

// OpRegexp is a collection of regexp operations
type OpRegexp struct {
}

var _ base.FreepsOperator = &OpRegexp{}

// RegexpArgs are the arguments for the Regexp function
type RegexpArgs struct {
	Regexp string
}

// FindStringIndex returns the first match of the given regexp
func (m *OpRegexp) FindStringIndex(ctx *base.Context, input *base.OperatorIO, args RegexpArgs) *base.OperatorIO {
	re, err := regexp.Compile(args.Regexp)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid regexp: %v", err)
	}
	loc := re.FindStringIndex(input.GetString())
	if loc == nil {
		return base.MakeOutputError(http.StatusExpectationFailed, "No match")
	}
	return base.MakePlainOutput(input.GetString()[loc[0]:loc[1]])
}

// FindStringSubmatchIndex returns the first match of the given regexp
func (m *OpRegexp) FindStringSubmatchIndex(ctx *base.Context, input *base.OperatorIO, args RegexpArgs) *base.OperatorIO {
	re, err := regexp.Compile(args.Regexp)
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Invalid regexp: %v", err)
	}
	loc := re.FindStringSubmatchIndex(input.GetString())
	if loc == nil {
		return base.MakeOutputError(http.StatusExpectationFailed, "No match")
	}
	return base.MakePlainOutput(input.GetString()[loc[2]:loc[3]])
}
