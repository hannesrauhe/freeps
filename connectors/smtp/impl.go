package smtp

import (
	"bytes"
	"fmt"
	"io"
	"net/mail"

	"github.com/emersion/go-smtp"
	"github.com/hannesrauhe/freeps/base"
	freepsstore "github.com/hannesrauhe/freeps/connectors/store"
	"github.com/hannesrauhe/freeps/freepsflow"
)

// MailHandler implements smtp.Backend to handle incoming emails
type MailHandler struct {
	GE  *freepsflow.FlowEngine
	ctx *base.Context
}

func (b *MailHandler) NewSession(c *smtp.Conn) (smtp.Session, error) {
	return &Session{GE: b.GE, ctx: b.ctx}, nil
}

// Session represents a mail session
type Session struct {
	ctx  *base.Context
	GE   *freepsflow.FlowEngine
	from string
	to   []string
	data bytes.Buffer
}

func (s *Session) Mail(from string, opts *smtp.MailOptions) error {
	s.from = from
	return nil
}

func (s *Session) Rcpt(to string, opts *smtp.RcptOptions) error {
	s.to = append(s.to, to)
	return nil
}

func (s *Session) Data(r io.Reader) error {
	_, err := io.Copy(&s.data, r)
	if err != nil {
		return err
	}

	msg, err := mail.ReadMessage(bytes.NewReader(s.data.Bytes()))
	if err != nil {
		return err
	}

	subject := msg.Header.Get("Subject")
	body, _ := io.ReadAll(msg.Body)
	input := base.MakeByteOutput([]byte(body))
	ctx := base.CreateContextWithField(s.ctx, "component", "smtp", fmt.Sprintf("mail from %s", s.from))

	tags := []string{"smtp", "sender:" + s.from}
	args := base.NewFunctionArguments(map[string]string{"from": s.from, "subject": subject})
	args.Set("to", s.to)

	// independent of recipients, trigger flows for sender
	out := s.GE.ExecuteFlowByTags(ctx, tags, args, input)
	_ = out // TODO: handle output

	for _, recipient := range s.to {
		tags := []string{"smtp", "to:" + recipient}
		freepsstore.GetGlobalStore().GetNamespaceNoError("_smtp").SetValue(recipient, input, ctx)
		out := s.GE.ExecuteFlowByTags(ctx, tags, args, input)
		_ = out // TODO: handle output
	}
	return nil
}

func (s *Session) Reset()        {}
func (s *Session) Logout() error { return nil }
