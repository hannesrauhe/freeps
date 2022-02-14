package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"log"
)

type internalContext struct {
	TemplateAction
	StatusCode      int                `json:",omitempty"`
	Output          interface{}        `json:",omitempty"`
	ChildrenContext []*internalContext `json:",omitempty"`
}

type ResponseCollector struct {
	writer      http.ResponseWriter
	children    []*ResponseCollector
	root        *ResponseCollector
	context     *internalContext
	prettyPrint bool
}

func NewJsonResponseWriter(w http.ResponseWriter) *ResponseCollector {
	return &ResponseCollector{writer: w}
}

func NewJsonResponseWriterPrintDirectly() *ResponseCollector {
	return &ResponseCollector{}
}

func (j *ResponseCollector) SetContext(ta *TemplateAction) {
	if j.context != nil {
		log.Print("Context is already set")
		return
	}
	j.context = &internalContext{TemplateAction: *ta}
}

func (j *ResponseCollector) SetPrettyPrint(p bool) {
	j.prettyPrint = p
}

func (j *ResponseCollector) GetHttpResponseWriter() http.ResponseWriter {
	root := j.root
	if root == nil {
		root = j
	}
	return j.writer
}

func (j *ResponseCollector) Clone() *ResponseCollector {
	if j.context == nil {
		log.Print("Context is not yet set")
		return nil
	}
	if j.context.ChildrenContext != nil {
		log.Print("Collector already finished")
		return nil
	}

	if j.children == nil {
		j.children = []*ResponseCollector{}
	}
	root := j.root
	if root == nil {
		root = j
	}
	c := &ResponseCollector{root: root}
	j.children = append(j.children, c)
	return c
}

func (j *ResponseCollector) WriteSuccess() {
	if j.context == nil {
		log.Print("Context is not yet set")
		return
	}
	if j.context.StatusCode != 0 {
		return
	}
	j.context.StatusCode = 200
	j.finishIfAllFinished()
}

func (j *ResponseCollector) WriteMessageWithCode(statusCode int, response interface{}) {
	if j.context == nil {
		log.Print("Context is not yet set")
		return
	}
	if j.context.StatusCode != 0 {
		log.Print("Response is already sent")
		return
	}
	if statusCode == 0 {
		log.Print("Setting status code to 0 is not allowed, using 200 instead")
		statusCode = 200
	}
	j.context.StatusCode = statusCode
	j.context.Output = response
	j.finishIfAllFinished()
}

func (j *ResponseCollector) WriteMessageWithCodef(statusCode int, format string, a ...interface{}) {
	j.WriteMessageWithCode(statusCode, fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteError(statusCode int, format string, a ...interface{}) {
	j.WriteMessageWithCode(statusCode, fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteSuccessMessage(response interface{}) {
	j.WriteMessageWithCode(200, response)
}

func (j *ResponseCollector) WriteSuccessf(format string, a ...interface{}) {
	j.WriteSuccessMessage(fmt.Sprintf(format, a...))
}

// GetOutput returns the output collected by this collector or the output of the first child that produced any output
// returns an error, if the children have failed or did not finish yet
func (j *ResponseCollector) GetOutput() (interface{}, error) {
	if !j.areChildrenFinished() {
		return nil, fmt.Errorf("Children haven't finished processing")
	}
	if j.IsStatusFailed() {
		return nil, fmt.Errorf("Status is failed")
	}
	if j.context.Output != nil {
		return j.context.Output, nil
	}
	if j.children != nil {
		for _, rc := range j.children {
			o, err := rc.GetOutput()
			if o != nil {
				return o, err
			}
		}
	}
	return nil, nil
}

// GetMarshalledOutput runs GetOutput and returnes the json-encoded Output
func (j *ResponseCollector) GetMarshalledOutput() ([]byte, error) {
	i, err := j.GetOutput()
	if err != nil {
		return nil, err
	}
	if i == nil {
		return []byte{}, nil
	}
	return json.Marshal(i)
}

func (j *ResponseCollector) IsStatusFailed() bool {
	return j.context.StatusCode >= 300
}

func (j *ResponseCollector) marshal(a interface{}) []byte {
	var m []byte
	if j.prettyPrint {
		m, _ = json.MarshalIndent(a, "", "  ")
	} else {
		m, _ = json.Marshal(a)
	}
	return m
}

func (j *ResponseCollector) writeFinalResponse() {
	if j.writer == nil {
		os.Stdout.Write(j.marshal(j.context))
		fmt.Println()
	} else {
		var m []byte
		m = j.marshal(j.context)
		j.writer.WriteHeader(j.context.StatusCode)
		j.writer.Write(m)
	}
}

func (j *ResponseCollector) areChildrenFinished() bool {
	if j.context == nil {
		return false
	}
	if j.context.StatusCode == 0 {
		return false
	}
	if j.children == nil {
		return true
	}
	for _, c := range j.children {
		if !c.areChildrenFinished() {
			return false
		}
	}
	return true
}

func (j *ResponseCollector) getChildrenResponse() bool {
	if j.children == nil {
		return true
	}
	if !j.areChildrenFinished() {
		return false
	}
	if j.context.ChildrenContext != nil {
		return true
	}
	j.context.ChildrenContext = make([]*internalContext, len(j.children))
	for k, c := range j.children {
		c.getChildrenResponse()
		j.context.ChildrenContext[k] = c.context
		if c.IsStatusFailed() {
			j.context.StatusCode = http.StatusFailedDependency
		} else if j.context.Output == nil {
			// collect the output of the first successful child - it's like the throne hierarchy in the British Royal family...
			j.context.Output, _ = c.GetOutput()
		}
	}
	return true
}

func (j *ResponseCollector) finishIfAllFinished() {
	if j.root != nil {
		j.root.finishIfAllFinished()
		return
	}
	// I'm root

	if !j.getChildrenResponse() {
		return
	}

	// done, make sure all references are deleted
	j.children = nil

	j.writeFinalResponse()
}
