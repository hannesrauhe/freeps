//go:build noexec || windows

package freepsexec

import (
	"fmt"

	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

func AddExecOperators(cr *utils.ConfigReader, flowEngine *freepsflow.FlowEngine) error {
	return fmt.Errorf("Not compiled")
}
