package smtp

import (
	"fmt"
	"net"

	"github.com/emersion/go-smtp"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	"github.com/hannesrauhe/freeps/utils"
)

// OpSMTP implements the FreepsOperator interface to trigger actions via OpSMTP
type OpSMTP struct {
	GE     *freepsflow.FlowEngine
	CR     *utils.ConfigReader
	config SMTPConfig
	ctx    *base.Context
}

var _ base.FreepsOperatorWithConfig = &OpSMTP{}
var _ base.FreepsOperatorWithShutdown = &OpSMTP{}

// GetDefaultConfig returns a copy of the default config
func (sm *OpSMTP) GetDefaultConfig() interface{} {
	newConfig := DefaultConfig
	return &newConfig
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (sm *OpSMTP) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	smc := *config.(*SMTPConfig)

	neSMTP := OpSMTP{config: smc, GE: sm.GE, ctx: ctx}

	return &neSMTP, nil
}

// Shutdown the smtp server
func (sm *OpSMTP) Shutdown(ctx *base.Context) {
}

// StartListening starts the smtp server to listen for incoming emails
func (sm *OpSMTP) StartListening(ctx *base.Context) {
	be := &MailHandler{GE: sm.GE, ctx: ctx}
	s := smtp.NewServer(be)

	s.Addr = fmt.Sprintf(":%d", sm.config.Port)
	s.Domain = "localhost"
	s.AllowInsecureAuth = true

	listener, err := net.Listen("tcp", s.Addr)
	if err != nil {
		ctx.GetLogger().Errorf("Failed to start SMTP listener on port %d: %v", sm.config.Port, err)
		return
	}
	go func() {
		ctx.GetLogger().Infof("SMTP server listening on %s", s.Addr)
		if err := s.Serve(listener); err != nil {
			ctx.GetLogger().Errorf("SMTP server error: %v", err)
		}
	}()
}
