package freepsdo

import (
	"encoding/json"
	"fmt"

	"log"
)

type internalContext struct {
	TemplateAction
	StatusCode      int                `json:",omitempty"`
	Output          interface{}        `json:",omitempty"`
	OutputType      string             `json:",omitempty"`
	ChildrenContext []*internalContext `json:",omitempty"`
}

type ResponseCollector struct {
	children    []*ResponseCollector
	root        *ResponseCollector
	context     *internalContext
	prettyPrint bool
}

func NewResponseCollector() *ResponseCollector {
	return &ResponseCollector{}
}

func (j *ResponseCollector) SetContext(ta *TemplateAction) {
	if j.context != nil {
		log.Print("Context is already set")
		return
	}
	j.context = &internalContext{TemplateAction: *ta}
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

func (j *ResponseCollector) WriteResponseWithCodeAndType(statusCode int, outputType string, response interface{}) {
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
	if response != nil {
		j.context.Output = response
		j.context.OutputType = outputType
	}
}

func (j *ResponseCollector) WriteMessageWithCode(statusCode int, response interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, "application/json", response)
}

func (j *ResponseCollector) WriteSuccess() {
	j.WriteMessageWithCode(200, nil)
}

func (j *ResponseCollector) WriteMessageWithCodef(statusCode int, format string, a ...interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, "text/plain", fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteError(statusCode int, format string, a ...interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, "text/plain", fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteSuccessMessage(response interface{}) {
	j.WriteMessageWithCode(200, response)
}

func (j *ResponseCollector) WriteSuccessf(format string, a ...interface{}) {
	j.WriteMessageWithCode(200, fmt.Sprintf(format, a...))
}

// GetOutput returns the output collected by this collector or the output of the first child that produced any output
// returns an error, if the children have failed or did not finish yet
func (j *ResponseCollector) GetOutput() (interface{}, string, error) {
	if !j.isSubtreeFinished() {
		return nil, "", fmt.Errorf("Children haven't finished processing")
	}
	if j.IsStatusFailed() {
		return j.context.Output, j.context.OutputType, fmt.Errorf("Status is failed")
	}
	if j.context.Output != nil {
		return j.context.Output, j.context.OutputType, nil
	}
	if j.children != nil {
		for _, rc := range j.children {
			o, t, err := rc.GetOutput()
			if o != nil {
				return o, t, err
			}
		}
	}
	return nil, "", nil
}

// GetMarshalledOutput runs GetOutput and returnes the json-encoded Output
func (j *ResponseCollector) GetMarshalledOutput() ([]byte, error) {
	i, _, err := j.GetOutput() // ignore the outputType for now
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

func (j *ResponseCollector) GetResponseTree() []byte {
	j.collectandFinalizeSubtreeResponse()
	b, _ := json.Marshal(j.context)
	return b
}

func (j *ResponseCollector) GetFinalResponse() (int, string, []byte) {
	j.collectandFinalizeSubtreeResponse()
	o, t, _ := j.GetOutput()
	var b []byte
	var err error
	switch t := o.(type) {
	case string:
		b = []byte(t)
	case []byte:
		b = t
	default:
		b, err = json.Marshal(o)
	}
	if err != nil {
		panic(err)
	}
	return j.context.StatusCode, t, b
}

func (j *ResponseCollector) isSubtreeFinished() bool {
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
		if !c.isSubtreeFinished() {
			return false
		}
	}
	return true
}

func (j *ResponseCollector) collectandFinalizeSubtreeResponse() bool {
	if j.children == nil {
		return true
	}
	if !j.isSubtreeFinished() {
		return false
	}
	if j.context.ChildrenContext != nil {
		return true
	}
	j.context.ChildrenContext = make([]*internalContext, len(j.children))
	for k, c := range j.children {
		c.collectandFinalizeSubtreeResponse()
		j.context.ChildrenContext[k] = c.context
		if c.IsStatusFailed() {
			j.context.StatusCode = 424 // http.StatusFailedDependency
		} else if j.context.Output == nil {
			var err error
			// collect the output of the first successful child - it's like the throne hierarchy in the British Royal family...
			j.context.Output, j.context.OutputType, err = c.GetOutput()
			if err != nil {
				panic(err)
			}
		}
	}
	j.children = nil
	return true
}
