package freepsdo

import (
	"encoding/json"
	"fmt"

	log "github.com/sirupsen/logrus"
)

type Response struct {
	StatusCode int          `json:",omitempty"`
	Output     interface{}  `json:",omitempty"`
	OutputType ResponseType `json:",omitempty"`
}

type internalContext struct {
	TemplateAction
	Creator           string             `json:",omitempty"`
	Response          *Response          `json:",omitempty"`
	ChildrenContext   []*internalContext `json:",omitempty"`
	CollectedResponse *Response          `json:",omitempty"`
}

type ResponseCollector struct {
	children   []*ResponseCollector
	root       *ResponseCollector
	context    *internalContext
	creator    string
	outputMode OutputModeT
}

func NewResponseCollector(creator string) *ResponseCollector {
	return &ResponseCollector{creator: creator}
}

func (j *ResponseCollector) SetContext(ta *TemplateAction) {
	if j.context != nil {
		log.Print("Context is already set")
		return
	}
	j.context = &internalContext{TemplateAction: *ta, Creator: j.creator}
}

func (j *ResponseCollector) SetOutputMode(outputMode OutputModeT) {
	j.outputMode = outputMode
}

func (j *ResponseCollector) Clone() *ResponseCollector {
	if j.context == nil {
		log.Print("Context is not yet set")
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

func (j *ResponseCollector) WriteResponseWithCodeAndType(statusCode int, outputType ResponseType, response interface{}) {
	if j.context == nil {
		log.Print("Context is not yet set")
		return
	}
	if j.context.Response != nil {
		log.Print("Response is already sent")
		return
	}
	if statusCode == 0 {
		log.Print("Setting status code to 0 is not allowed, using 200 instead")
		statusCode = 200
	}
	j.context.Response = &Response{Output: response, StatusCode: statusCode, OutputType: outputType}
}

func (j *ResponseCollector) WriteMessageWithCode(statusCode int, response interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, ResponseTypeJSON, response)
}

func (j *ResponseCollector) WriteSuccess() {
	j.WriteResponseWithCodeAndType(200, ResponseTypeNone, nil)
}

func (j *ResponseCollector) WriteMessageWithCodef(statusCode int, format string, a ...interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, ResponseTypePlainText, fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteError(statusCode int, format string, a ...interface{}) {
	j.WriteResponseWithCodeAndType(statusCode, ResponseTypePlainText, fmt.Sprintf(format, a...))
}

func (j *ResponseCollector) WriteSuccessMessage(response interface{}) {
	j.WriteMessageWithCode(200, response)
}

func (j *ResponseCollector) WriteSuccessf(format string, a ...interface{}) {
	j.WriteResponseWithCodeAndType(200, ResponseTypePlainText, fmt.Sprintf(format, a...))
}

func responseIsEmpty(r *Response) bool {
	return r == nil || r.Output == nil
}

// GetOutput collects the output of the collector and all it's children
func (j *ResponseCollector) GetOutput() (*Response, error) {
	if !j.isSubtreeFinished() {
		return nil, fmt.Errorf("Children haven't finished processing")
	}
	if j.context.ChildrenContext == nil {
		j.context.ChildrenContext = make([]*internalContext, len(j.children))
	}

	var err error
	err = nil
	switch j.outputMode {
	// keep this logic for backward compatibiliy; will hopefully get thrown out at some point
	case OutputModeFirstNonEmpty:
		if j.context.CollectedResponse == nil {
			j.context.CollectedResponse = &Response{StatusCode: 200}
		}
		if !responseIsEmpty(j.context.Response) {
			j.context.CollectedResponse.Output = j.context.Response.Output
			j.context.CollectedResponse.OutputType = j.context.Response.OutputType
		}
		for _, c := range j.children {
			cr, cErr := c.GetOutput()
			if cErr != nil {
				err = cErr
			}
			j.context.ChildrenContext = append(j.context.ChildrenContext, c.context)
			if responseIsEmpty(j.context.CollectedResponse) && !responseIsEmpty(cr) {
				j.context.CollectedResponse.Output = cr.Output
				j.context.CollectedResponse.OutputType = cr.OutputType
			}
			if c.IsStatusFailed() {
				j.context.CollectedResponse.StatusCode = 424
			}
		}
		j.children = nil
	}

	return j.context.CollectedResponse, err
}

// GetMarshalledOutput runs GetOutput and returnes the json-encoded Output or an error if the operation failed
func (j *ResponseCollector) GetMarshalledOutput() ([]byte, error) {
	r, err := j.GetOutput()
	if err != nil {
		return nil, err
	}
	if j.IsStatusFailed() {
		return nil, fmt.Errorf("Status Code: %v", j.GetStatusCode())
	}
	if r.Output == nil {
		return []byte{}, nil
	}
	outputObject := r.Output
	if r.OutputType == ResponseTypePlainText {
		switch r.Output.(type) {
		case string:
			outputObject = map[string]string{"output": outputObject.(string)}
		default:
			return nil, fmt.Errorf("Output is not plain text as expected")
		}
	}
	return json.Marshal(outputObject)
}

func (j *ResponseCollector) GetFinalResponse(pretty bool) (int, ResponseType, []byte) {
	r, err := j.GetOutput()
	if err != nil {
		return 500, ResponseTypePlainText, []byte(err.Error())
	}
	var b []byte
	switch t := r.Output.(type) {
	case string:
		b = []byte(t)
	case []byte:
		b = t
	default:
		if !pretty {
			b, _ = json.Marshal(r.Output)
		} else {
			b, _ = json.MarshalIndent(r.Output, "", "  ")
		}
	}
	return r.StatusCode, r.OutputType, b
}

func (j *ResponseCollector) IsStatusFailed() bool {
	return j.GetStatusCode() >= 300
}

func (j *ResponseCollector) GetStatusCode() int {
	if j.context == nil {
		return 0
	}
	if j.context.Response == nil {
		return 0
	}
	return j.context.Response.StatusCode
}

func (j *ResponseCollector) IsRoot() bool {
	return j.root == nil
}

func (j *ResponseCollector) GetCreator() string {
	if j.root == nil {
		return j.creator
	}
	return j.root.creator
}

func (j *ResponseCollector) GetResponseTree() []byte {
	j.GetOutput()
	b, err := json.MarshalIndent(j.context, "", "  ")
	if err != nil {
		log.Printf("Error when marshalling response tree: %v", err)
	}
	return b
}

func (j *ResponseCollector) isSubtreeFinished() bool {
	if j.context == nil {
		return false
	}
	if j.context.Response == nil {
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
