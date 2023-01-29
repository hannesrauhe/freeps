package mqtt

import (
	log "github.com/sirupsen/logrus"

	"fmt"
	"sync"

	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

// FreepsMqtt provides the functions for MQTT handling
type FreepsMqtt struct {
	impl *FreepsMqttImpl
}

var instantiated *FreepsMqtt
var once sync.Once

// GetInstance returns the process-wide instance of FreepsMqtt, instance needs to be initialized before use
func GetInstance() *FreepsMqtt {
	once.Do(func() {
		instantiated = &FreepsMqtt{}
	})
	return instantiated
}

// Init initilaizes FreepsMQTT based on the config
func (fm *FreepsMqtt) Init(logger log.FieldLogger, cr *utils.ConfigReader, ge *freepsgraph.GraphEngine) error {
	if fm.impl != nil {
		return fmt.Errorf("Freepsmqtt already initialized")
	}
	var err error
	fm.impl, err = newFreepsMqttImpl(logger, cr, ge)
	return err
}

// Shutdown MQTT and cancel all subscriptions
func (fm *FreepsMqtt) Shutdown() {
	if fm.impl == nil {
		return
	}
	fm.impl = nil
}

func (fm *FreepsMqtt) Publish(topic string, msg interface{}, qos int, retain bool) error {
	if fm.impl == nil {
		return fmt.Errorf("Mqtt not initialized")
	}
	return fm.impl.publish(topic, msg, qos, retain)
}

func (fm *FreepsMqtt) SubscribeToTags() error {
	if fm.impl == nil {
		return fmt.Errorf("Mqtt not initialized")
	}
	return fm.impl.starTagSubscriptions()
}

func (fm *FreepsMqtt) GetSubscriptions() ([]string, error) {
	if fm.impl == nil {
		return nil, fmt.Errorf("Mqtt not initialized")
	}
	topics := make([]string, 0, len(fm.impl.topics))
	for t := range fm.impl.topics {
		topics = append(topics, t)
	}
	return topics, nil
}
