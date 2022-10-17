package freepsgraph

import (
	"net/http"
	"os/exec"

	log "github.com/sirupsen/logrus"
)

type OpRaspistill struct {
}

var _ FreepsOperator = &OpRaspistill{}

func CaptureRaspiStill(cameraParams map[string]string) (bytes []byte, err error) {
	defaultArgs := map[string]string{
		"-w":           "1600",
		"-h":           "1200",
		"-o":           "-",
		"-e":           "jpg",
		"--quality":    "90",
		"--brightness": "50"}

	for k, v := range cameraParams {
		defaultArgs[k] = v
	}

	args := []string{}
	for k, v := range defaultArgs {
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
	b, err := CaptureRaspiStill(vars)

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
	ret := []string{"-rot", "-ss"}
	return ret
}

func (m *OpRaspistill) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	ret := map[string]string{
		"-rot": "0,90,180,270",
		"-ss":  "10,100,1000,10000",
	}
	return ret
}
