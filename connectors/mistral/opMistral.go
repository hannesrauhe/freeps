package opmistral

import (
	"bytes"
	"encoding/json"
	"io"
	"mime/multipart"
	"net/http"
	"time"

	"github.com/hannesrauhe/freeps/base"
)

// MistralConfig is the configuration of the Mistral operator.
type MistralConfig struct {
	APIKey   string
	BaseURL  string
	Model    string
	Language string
}

// OpMistral talks to the Mistral API. The key lives in the freeps config, so
// LAN clients can read it via the config operator instead of typing it in.
type OpMistral struct {
	conf MistralConfig
}

var _ base.FreepsOperatorWithConfig = &OpMistral{}

func (o *OpMistral) GetDefaultConfig() interface{} {
	return &MistralConfig{
		BaseURL:  "https://api.mistral.ai/v1",
		Model:    "voxtral-mini-latest",
		Language: "de",
	}
}

func (o *OpMistral) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	return &OpMistral{conf: *config.(*MistralConfig)}, nil
}

type TranscribeArgs struct {
	Model    *string
	Language *string
}

var _ base.FreepsFunctionParametersWithInit = &TranscribeArgs{}

func (a *TranscribeArgs) Init(ctx *base.Context, op base.FreepsOperator, fn string) {
	c := op.(*OpMistral).conf
	a.Model = &c.Model
	a.Language = &c.Language
}

// Transcribe turns spoken audio given as main input into text.
func (o *OpMistral) Transcribe(ctx *base.Context, mainInput *base.OperatorIO, args TranscribeArgs) *base.OperatorIO {
	if o.conf.APIKey == "" {
		return base.MakeOutputError(http.StatusBadRequest, "no Mistral key in configuration")
	}
	audio, err := mainInput.GetBytes()
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "%v", err)
	}
	if len(audio) == 0 {
		return base.MakeOutputError(http.StatusBadRequest, "no audio data in input")
	}

	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	if _, err := part.Write(audio); err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	w.WriteField("model", *args.Model)
	if *args.Language != "" {
		w.WriteField("language", *args.Language)
	}
	if err := w.Close(); err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}

	req, err := http.NewRequest(http.MethodPost, o.conf.BaseURL+"/audio/transcriptions", body)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+o.conf.APIKey)

	client := &http.Client{Timeout: 90 * time.Second}
	res, err := client.Do(req)
	if err != nil {
		return base.MakeOutputError(http.StatusBadGateway, "Mistral not reachable: %v", err)
	}
	defer res.Body.Close()
	response, err := io.ReadAll(res.Body)
	if err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	if res.StatusCode != http.StatusOK {
		return base.MakeOutputError(res.StatusCode, "Mistral HTTP %v: %v", res.StatusCode, excerpt(string(response)))
	}

	var parsed struct{ Text string }
	if err := json.Unmarshal(response, &parsed); err != nil {
		return base.MakeInternalServerErrorOutput(err)
	}
	return base.MakePlainOutput(parsed.Text)
}

// excerpt shortens a server error body for log/response use.
func excerpt(s string) string {
	if len(s) <= 200 {
		return s
	}
	return s[:200] + "…"
}
