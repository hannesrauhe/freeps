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
