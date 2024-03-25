//go:build noexec || windows

package freepsexecutable

import (
	"fmt"

	"github.com/hannesrauhe/freeps/base"
)

type OpExecutable struct {
}

var _ base.FreepsOperatorWithConfig = &OpExecutable{}

// GetDefaultConfig returns a copy of the default config
func (bt *OpExecutable) GetDefaultConfig(fullName string) interface{} {
	return &ExecutableConfig{Enabled: false}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (bt *OpExecutable) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	return nil, fmt.Errorf("OpExecutable support not compiled in")
}
