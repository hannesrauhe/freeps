package freepsdo

import (
	"encoding/json"
	"log"
	"net/http"
	"os/exec"

	"github.com/hannesrauhe/freeps/utils"
)

type MuttMod struct {
	config *MuttModConfig
}

type MuttModConfig struct {
	DefaultSubject string
	DefaultBody    string
	DefaultRecv    string
}

var MuttModDefaultConfig = MuttModConfig{DefaultSubject: "default subject", DefaultBody: "default Body", DefaultRecv: ""}

var _ Mod = &MuttMod{}

type MuttParameters struct {
	Subject    string
	Attachment string
	Body       string
	Recv       string
}

func NewMuttMod(cr *utils.ConfigReader) *MuttMod {
	conf := MuttModDefaultConfig
	err := cr.ReadSectionWithDefaults("muttmod", &conf)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}
	return &MuttMod{}
}

func (m *MuttMod) DoWithJSON(function string, inputBytes []byte, jrw *ResponseCollector) {
	params := MuttParameters{Subject: m.config.DefaultSubject, Body: m.config.DefaultBody, Recv: m.config.DefaultRecv}

	if function == "send" {
		err := json.Unmarshal(inputBytes, &params)

		if err != nil {
			log.Printf("%q", inputBytes)
			jrw.WriteError(http.StatusBadRequest, "%v", err.Error())
			return
		}
	}

	if params.Recv == "" {
		jrw.WriteError(http.StatusBadRequest, "Need a valid receiver mail address")
		return
	}

	// "New document: $FILE_TO_UPLOAD" | mutt -s "New document: $FILE_TO_UPLOAD" "${MAIL_RECV_ADDR}" -a "$FILE_TO_UPLOAD"
	args := []string{"-s", params.Subject, params.Recv}
	cmd := exec.Command("mutt", args...)
	p, err := cmd.StdinPipe()
	err = cmd.Start()
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, err.Error())
	}

	if function == "sendasbody" {
		_, err = p.Write(inputBytes)
	} else if function == "send" {
		_, err = p.Write([]byte(params.Body))
	}

	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, err.Error())
	}
	err = p.Close()
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, err.Error())
	}
	err = cmd.Wait()
	if err != nil {
		jrw.WriteError(http.StatusInternalServerError, err.Error())
	}
	jrw.WriteSuccessMessage(params)
}

func (m *MuttMod) GetFunctions() []string {
	return []string{"send", "sendasbody"}
}

func (m *MuttMod) GetPossibleArgs(fn string) []string {
	ret := []string{"subject", "body", "recv", "attachment"}
	return ret
}

func (m *MuttMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	ret := map[string]string{}
	return ret
}
