package automation

import (
	"github.com/hannesrauhe/freeps/base"
)

type RuleTrigger struct {
}

var _ base.FreepsTrigger = &RuleTrigger{}

func (bt *RuleTrigger) GetName() string {
	return "Rule"
}

func (bt *RuleTrigger) GetDescription() string {
	return "Triggers when a rule gets deleted or created via the Automation operator"
}

func (bt *RuleTrigger) GetSuggestions() []string {
	return []string{"created", "edited", "deleted"}
}

func (oa *OpAutomation) GetTriggers() []base.FreepsTrigger {
	return []base.FreepsTrigger{&RuleTrigger{}}
}
