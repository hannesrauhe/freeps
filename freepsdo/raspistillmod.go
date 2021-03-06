package freepsdo

import (
	"fmt"
	"log"
	"net/http"
	"os/exec"
	"strconv"
)

type RaspistillMod struct {
}

var _ Mod = &RaspistillMod{}

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

func (m *RaspistillMod) DoWithJSON(fn string, jsonStr []byte, jrw *ResponseCollector) {
	b, err := CaptureRaspiStill(1600, 1200, map[string]interface{}{"--quality": 90, "--brightness": 50})

	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, "Error executing raspistill: %v", err.Error())
		return
	}

	jrw.WriteResponseWithCodeAndType(200, ResponseTypeJPEG, b)
}

func (m *RaspistillMod) GetFunctions() []string {
	ret := []string{"do"}
	return ret
}

func (m *RaspistillMod) GetPossibleArgs(fn string) []string {
	ret := []string{}
	return ret
}

func (m *RaspistillMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}
