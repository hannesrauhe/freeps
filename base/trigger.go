package base

// FreepsTrigger provides the interface for all triggers used to start automatic actions
type FreepsTrigger interface {
	// GetTriggerName returns the name of the trigger
	GetName() string
	// GetTriggerType returns the type of the trigger
	GetDescription() string
	// GetSuggestions returns a list of possible trigger values
	GetSuggestions() []string
}

type FreepsTriggerImpl struct {
	name        string
	desc        string
	suggestions []string
}

func NewFreepTrigger(name string, desc string, suggestions []string) *FreepsTriggerImpl {
	return &FreepsTriggerImpl{name: name, desc: desc, suggestions: suggestions}
}

func (t *FreepsTriggerImpl) GetName() string {
	return t.name
}

func (t *FreepsTriggerImpl) GetDescription() string {
	return t.desc
}

func (t *FreepsTriggerImpl) GetSuggestions() []string {
	return t.suggestions
}
