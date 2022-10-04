package freepsgraph

import (
	"log"
	"net/http"
	"os/exec"
	"strconv"
)

type OpRaspistill struct {
}

var _ FreepsOperator = &OpRaspistill{}

func CaptureRaspiStill(width, height int, cameraParams map[string]string) (bytes []byte, err error) {
	args := []string{
		"-w", strconv.Itoa(width),
		"-h", strconv.Itoa(height),
		"-o", "-", // output to stdout
	}
	for k, v := range cameraParams {
		args = append(args, k)
		if v != "" {
			args = append(args, v)
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
	b, err := CaptureRaspiStill(1600, 1200, map[string]string{"--quality": "90", "--brightness": "50"})

	if err != nil {
		return MakeOutputError(http.StatusInternalServerError, "Error executing raspistill: %v", err.Error())
	}

	return MakeByteOutputWithContentType(b, "image/jpeg")
}

func (m *OpRaspistill) GetFunctions() []string {
	ret := []string{"do"}
	return ret
}

func (m *OpRaspistill) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *OpRaspistill) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	ret := map[string]string{}
	return ret
}
