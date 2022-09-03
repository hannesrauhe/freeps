package freepsgraph

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
)

type OpRaspistill struct {
}

var _ FreepsOperator = &OpRaspistill{}

func CaptureRaspiStill(width, height int, cameraParams map[string]interface{}) (bytes []byte, err error) {
	args := []string{
		"-w", strconv.Itoa(width),
		"-h", strconv.Itoa(height),
		"-o", "-", // output to stdout
	}
	for k, v := range cameraParams {
		args = append(args, k)
		if v != nil {
			args = append(args, fmt.Sprintf("%v", v))
		}
	}

	byt, err := exec.Command("/usr/bin/raspistill", args...).CombinedOutput()
	if err != nil {
		log.Printf("*** Error running raspistillbin: %v\n", err)
		return []byte{}, err
	}
	return byt, nil
}

func (m *OpRaspistill) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	b, err := CaptureRaspiStill(1600, 1200, map[string]interface{}{"--quality": 90, "--brightness": 50})

	if err != nil {
		return MakeOutputError(http.StatusInternalServerError, "Error executing raspistill: %v", err.Error())
	}

	return MakeByteOutput(b)
}

func (m *OpRaspistill) GetFunctions() []string {
	ret := []string{"do"}
	return ret
}

func (m *OpRaspistill) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *OpRaspistill) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}
