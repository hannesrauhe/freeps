//go:build noexec || windows

package freepsexec

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

func AddExecOperators(cr *utils.ConfigReader, graphEngine *freepsgraph.GraphEngine) error {
	return fmt.Errorf("Not compiled")
}
