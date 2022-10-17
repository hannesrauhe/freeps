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
	switch arg {
	case "-rot":
		return map[string]string{"0": "0", "90": "90", "180": "180", "270": "270"}
	case "-ss":
		return map[string]string{"1s": "1000", "2s": "2000", "3s": "3000", "4s": "4000", "5s": "5000"}
	}
	return map[string]string{}
}
