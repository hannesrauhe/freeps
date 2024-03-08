package base

import (
	"bytes"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
)

// OperationLogEntry is an entry description of a Graph Operation
type OperationLogEntry struct {
	GraphName         string
	OpDesc            string
	OpName            string
	InputFrom         string
	Arguments         map[string]string
	StartTime         time.Time
	ExecutionDuration time.Duration
	HTTPResponseCode  int
	NestingLevel      int
}

// OperationLogNoTime is a OperationLog without time structs - for serialization and readability
type OperationLogNoTime struct {
	GraphName         string
	OpDesc            string
	OpName            string
	InputFrom         string
	StartTime         string
	ExecutionDuration string
	HTTPResponseCode  int
	NestingLevel      int
}

func (o OperationLogEntry) toNoTime() OperationLogNoTime {
	return OperationLogNoTime{
		GraphName:         o.GraphName,
		OpDesc:            o.OpDesc,
		OpName:            o.OpName,
		InputFrom:         o.InputFrom,
		StartTime:         o.StartTime.Format(time.RFC1123),
		ExecutionDuration: o.ExecutionDuration.String(),
		HTTPResponseCode:  o.HTTPResponseCode,
		NestingLevel:      o.NestingLevel,
	}
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (o OperationLogEntry) MarshalJSON() ([]byte, error) {
	return json.Marshal(o.toNoTime())
}

// Context keeps the runtime data of a graph execution tree
type Context struct {
	UUID         uuid.UUID
	logger       log.FieldLogger
	Created      time.Time
	Finished     time.Time
	Operations   []OperationLogEntry
	CurrentLevel int
}

// ContextNoTime is a Context without time structs - for serialization and readability
type ContextNoTime struct {
	UUID         uuid.UUID
	Created      string
	Finished     string
	Operations   []OperationLogNoTime
	CurrentLevel int
}

// MarshalJSON provides a custom marshaller with better readable time formats
func (c Context) MarshalJSON() ([]byte, error) {
	cnt := []OperationLogNoTime{}
	for _, o := range c.Operations {
		cnt = append(cnt, o.toNoTime())
	}
	readable := ContextNoTime{
		UUID:         c.UUID,
		Created:      c.Created.Format(time.RFC1123),
		Finished:     c.Finished.Format(time.RFC1123),
		Operations:   cnt,
		CurrentLevel: c.CurrentLevel,
	}

	return json.Marshal(readable)
}

// NewContext creates a Context with a given logger
func NewContext(logger log.FieldLogger) *Context {
	u := uuid.New()
	return &Context{UUID: u, logger: logger.WithField("uuid", u.String()), Created: time.Now(), Operations: make([]OperationLogEntry, 0)}
}

// GetID returns the string represantation of the ID for this execution tree
func (c *Context) GetID() string {
	return c.UUID.String()
}

// GetLogger returns a Logger with the proper fields added to identify the context
func (c *Context) GetLogger() log.FieldLogger {
	return c.logger
}

func (c *Context) IncreaseNesting() {
	c.CurrentLevel++
}

func (c *Context) DecreaseNesting() {
	c.CurrentLevel--
	c.Finished = time.Now()
}

func (c *Context) IsRootContext() bool {
	return c.CurrentLevel == 0
}

// RecordOperationStart records a new entry in the execution log of this context and returns the index
func (c *Context) RecordOperationStart(graphName string, opDesc string, opName string, inputFrom string, arguments map[string]string) int {
	op := OperationLogEntry{GraphName: graphName,
		OpDesc:       opDesc,
		StartTime:    time.Now(),
		OpName:       opName,
		InputFrom:    inputFrom,
		NestingLevel: c.CurrentLevel,
		Arguments:    arguments,
	}
	c.Operations = append(c.Operations, op)
	return len(c.Operations) - 1
}

// GetOperation returns the OperationLog for the given index
func (c *Context) GetOperation(opIndex int) OperationLogEntry {
	return c.Operations[opIndex]
}

// RecordOperationFinish marks the operation at opIndex finished with code responseCode
func (c *Context) RecordOperationFinish(opIndex int, responseCode int) {
	c.Operations[opIndex].HTTPResponseCode = responseCode
	c.Operations[opIndex].ExecutionDuration = time.Now().Sub(c.Operations[opIndex].StartTime)
}

// digraph G {
// 	fontname="Helvetica,Arial,sans-serif"
// 	node [fontname="Helvetica,Arial,sans-serif"]
// 	edge [fontname="Helvetica,Arial,sans-serif"]

// 	subgraph cluster_0 {
// 		style=filled;
// 		color=lightgrey;
// 		node [style=filled,color=white];
// 		a0 -> a1 -> a2 -> a3;
// 		label = "process #1";
// 	}

// 	subgraph cluster_1 {
// 		node [style=filled];
// 		b0 -> b1 -> b2 -> b3;
// 		label = "process #2";
// 		color=blue
// 	}
// 	start -> a0;
// 	start -> b0;
// 	a1 -> b3;
// 	b2 -> a3;
// 	a3 -> a0;
// 	a3 -> end;
// 	b3 -> end;

//		start [shape=Mdiamond];
//		end [shape=Msquare];
//	}
//
// ToDot returns an execution graph in dot-Notation
func (c *ContextNoTime) ToDot() string {
	var w bytes.Buffer
	wrt := func(st string) { w.WriteString(st) }
	wrtnl := func(st string) { wrt(st + "\n") }
	wrtf := func(format string, a ...any) { w.WriteString(fmt.Sprintf(format, a...)) }
	wrtfnl := func(format string, a ...any) { wrtf(format+"\n", a...) }
	wrtnl("digraph G {")

	inputNode := "input"
	wrtfnl(" \"%s\" [shape=Mdiamond, label=input]", inputNode)

	maxNestingLevel := 0
	wrtf(" \"%s\"", inputNode)
	for i, n := range c.Operations {
		wrtf(" -> %d", i)
		if n.NestingLevel > maxNestingLevel {
			maxNestingLevel = n.NestingLevel
		}
	}
	wrtnl("[style=\"dotted\"]")

	inputMap := map[string]int{}

	opened := 0
	lastNestingLevel := 1
	printNode := func(n OperationLogNoTime, nodeID int) {
		label := n.OpName
		if label[0] == '#' {
			label = n.OpDesc
		}
		wrtfnl("%d [label=\"%s\"]", nodeID, label)
		if n.InputFrom != "" && n.InputFrom != "_" {
			wrtfnl("%d -> %d", inputMap[fmt.Sprintf("%d.%v", n.NestingLevel, n.InputFrom)], nodeID)
		}
	}
	for i, n := range c.Operations {
		inputMap[fmt.Sprintf("%d.%v", n.NestingLevel, n.OpName)] = i
		if n.NestingLevel < lastNestingLevel {
			if opened > 0 {
				opened--
				wrtnl("}")
			}
			printNode(n, i)
			// prevCaller = fmt.Sprintf("%d.%s", n.NestingLevel, n.OpName)
		} else if lastNestingLevel == n.NestingLevel {
			printNode(n, i)
		} else if n.NestingLevel > lastNestingLevel {
			opened++
			wrtfnl("subgraph cluster_%d {", i)
			wrtfnl("label = \"%v (%d)\"", n.GraphName, i)
			inputMap[fmt.Sprintf("%d._", n.NestingLevel)] = i - 1
			printNode(n, i)
		}
		lastNestingLevel = n.NestingLevel
	}
	for i := 0; i < opened; i++ {
		wrtnl(" }")
	}

	wrtnl("}")

	return w.String()
}
