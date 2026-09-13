package freepsflow_test

import (
	"testing"
	"time"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsflow"
	log "github.com/sirupsen/logrus"
)

/*
Regression tests for the shutdown deadlock: FlowEngine.Shutdown must not hold the
internal operator lock while calling Shutdown on the operators. Operators may block
in Shutdown until one of their background goroutines finishes, and those goroutines
may call back into the engine (e.g. GetOperator). Since the lock is not reentrant,
holding it during Shutdown leads to a permanent deadlock (observed with the telegram
operator, which waits for its update loop to finish).
*/

// reentrantOp calls back into the flow engine from its Shutdown function.
type reentrantOp struct {
	ge *freepsflow.FlowEngine
}

func (*reentrantOp) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return input
}
func (*reentrantOp) GetFunctions() []string             { return []string{} }
func (*reentrantOp) GetPossibleArgs(fn string) []string { return []string{} }
func (*reentrantOp) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	return map[string]string{}
}
func (*reentrantOp) GetName() string                  { return "reentrant" }
func (*reentrantOp) GetHook() interface{}             { return nil }
func (*reentrantOp) StartListening(ctx *base.Context) {}

func (o *reentrantOp) Shutdown(ctx *base.Context) {
	// This would deadlock if FlowEngine.Shutdown still held the (non-reentrant)
	// operator lock while calling this function.
	o.ge.GetOperator("reentrant")
}

// blockedListenerOp mimics the telegram operator: Shutdown waits for a background
// goroutine (started via StartListening) to finish, and that goroutine calls back
// into the flow engine before signaling completion.
type blockedListenerOp struct {
	ge            *freepsflow.FlowEngine
	shutdownStart chan struct{} // closed by Shutdown to synchronize the listener goroutine
	closeChan     chan int      // unbuffered completion signal, like in OpTelegram
	done          chan struct{} // closed by the listener goroutine after signaling completion
}

func (*blockedListenerOp) Execute(ctx *base.Context, fn string, fa base.FunctionArguments, input *base.OperatorIO) *base.OperatorIO {
	return input
}
func (*blockedListenerOp) GetFunctions() []string             { return []string{} }
func (*blockedListenerOp) GetPossibleArgs(fn string) []string { return []string{} }
func (*blockedListenerOp) GetArgSuggestions(fn string, arg string, otherArgs base.FunctionArguments) map[string]string {
	return map[string]string{}
}
func (*blockedListenerOp) GetName() string      { return "blockedlistener" }
func (*blockedListenerOp) GetHook() interface{} { return nil }

func (o *blockedListenerOp) StartListening(ctx *base.Context) {
	go func() {
		<-o.shutdownStart // make sure Shutdown is already running (and held the lock before the fix)
		o.ge.GetOperators()
		o.closeChan <- 1
		close(o.done)
	}()
}

func (o *blockedListenerOp) Shutdown(ctx *base.Context) {
	close(o.shutdownStart)
	<-o.closeChan
}

func runShutdownWithDeadline(t *testing.T, ge *freepsflow.FlowEngine) {
	t.Helper()
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "shutdown test")
	done := make(chan struct{})
	go func() {
		ge.Shutdown(ctx)
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("FlowEngine.Shutdown did not return within 5 seconds - deadlock?")
	}
}

func TestShutdownWithEngineReentrantOperator(t *testing.T) {
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	ge := freepsflow.NewFlowEngine(ctx, nil, func() {})
	op := &reentrantOp{ge: ge}
	ge.AddOperator(op)

	runShutdownWithDeadline(t, ge)
}

func TestShutdownWithBlockedListenerGoroutine(t *testing.T) {
	ctx := base.NewBaseContextWithReason(log.StandardLogger(), "")
	ge := freepsflow.NewFlowEngine(ctx, nil, func() {})
	op := &blockedListenerOp{ge: ge, shutdownStart: make(chan struct{}), closeChan: make(chan int), done: make(chan struct{})}
	ge.AddOperator(op)
	ge.StartListening(ctx)

	runShutdownWithDeadline(t, ge)

	// after shutdown, the listener goroutine must have finished signaling completion
	select {
	case <-op.done:
	case <-time.After(time.Second):
		t.Fatal("listener goroutine did not complete shutdown")
	}
}
