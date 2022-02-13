package freepsdo

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"

	"log"
)

type internalResponse struct {
	TemplateAction
	StatusCode       int                 `json:",omitempty"`
	Output           interface{}         `json:",omitempty"`
	ChildrenResponse []*internalResponse `json:",omitempty"`
}

type ResponseCollector struct {
	writer      http.ResponseWriter
	children    []*ResponseCollector
	root        *ResponseCollector
	response    *internalResponse
	prettyPrint bool
}

func NewJsonResponseWriter(w http.ResponseWriter) *ResponseCollector {
	return &ResponseCollector{writer: w}
}

func NewJsonResponseWriterPrintDirectly() *ResponseCollector {
	return &ResponseCollector{}
}

func (j *ResponseCollector) SetContext(ta *TemplateAction) {
	if j.response != nil {
		log.Print("Context is already set")
		return
	}
	j.response = &internalResponse{TemplateAction: *ta}
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
	if j.response == nil {
		log.Print("Context is not yet set")
		return nil
	}
	if j.response.StatusCode != 0 {
		log.Print("Response already sent")
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
	if j.response == nil {
		log.Print("Context is not yet set")
		return
	}
	if j.response.StatusCode != 0 {
		return
	}
	j.response.StatusCode = 200
	j.finishIfAllFinished()
}

func (j *ResponseCollector) WriteMessageWithCode(statusCode int, response interface{}) {
	if j.response == nil {
		log.Print("Context is not yet set")
		return
	}
	if j.response.StatusCode != 0 {
		log.Print("Response is already sent")
		return
	}
	if statusCode == 0 {
		log.Print("Setting status code to 0 is not allowed, using 200 instead")
		statusCode = 200
	}
	j.response.StatusCode = statusCode
	j.response.Output = response
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
		os.Stdout.Write(j.marshal(j.response))
		fmt.Println()
	} else {
		var m []byte
		m = j.marshal(j.response)
		j.writer.WriteHeader(j.response.StatusCode)
		j.writer.Write(m)
	}
}

func (j *ResponseCollector) areChildrenFinished() bool {
	if j.response == nil {
		return false
	}
	if j.response.StatusCode == 0 {
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
	if j.response.ChildrenResponse != nil {
		return true
	}
	j.response.ChildrenResponse = make([]*internalResponse, len(j.children))
	for k, c := range j.children {
		c.getChildrenResponse()
		j.response.ChildrenResponse[k] = c.response
		if c.response.StatusCode >= 400 {
			j.response.StatusCode = http.StatusFailedDependency
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
