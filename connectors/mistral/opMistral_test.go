package opmistral

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path"
	"testing"

	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
	"github.com/sirupsen/logrus"
	"gotest.tools/v3/assert"
)

func TestOpMistral(t *testing.T) {
	ctx := base.NewBaseContextWithReason(logrus.StandardLogger(), "")
	tdir := t.TempDir()
	cr, err := utils.NewConfigReader(logrus.StandardLogger(), path.Join(tdir, "test_config.json"))
	assert.NilError(t, err)

	// the [mistral] section is generated on startup, with defaults
	ops := base.MakeFreepsOperators(&OpMistral{}, cr, ctx)
	assert.Equal(t, len(ops), 1)
	b, err := cr.GetSectionBytes("mistral")
	assert.NilError(t, err)
	var conf MistralConfig
	assert.NilError(t, json.Unmarshal(b, &conf))
	assert.Equal(t, conf.BaseURL, "https://api.mistral.ai/v1")
	assert.Equal(t, conf.Model, "voxtral-mini-latest")
	assert.Equal(t, conf.Language, "de")

	// without a key, Transcribe refuses
	res := ops[0].Execute(ctx, "Transcribe", base.MakeEmptyFunctionArguments(), base.MakePlainOutput("RIFF..."))
	assert.Assert(t, res.IsError())

	// with a key, Transcribe posts the audio to the transcriptions endpoint
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, r.URL.Path, "/v1/audio/transcriptions")
		assert.Equal(t, r.Header.Get("Authorization"), "Bearer secret123")
		assert.NilError(t, r.ParseMultipartForm(1<<20))
		assert.Equal(t, r.FormValue("model"), "voxtral-mini-latest")
		assert.Equal(t, r.FormValue("language"), "de")
		file, _, err := r.FormFile("file")
		assert.NilError(t, err)
		defer file.Close()
		audio, err := io.ReadAll(file)
		assert.NilError(t, err)
		assert.Equal(t, string(audio), "RIFF...")
		json.NewEncoder(w).Encode(map[string]string{"text": "mach das licht an"})
	}))
	defer srv.Close()

	cr.SetConfigFileContent(string(mustWriteSection(t, b, MistralConfig{APIKey: "secret123", BaseURL: srv.URL + "/v1", Model: "voxtral-mini-latest", Language: "de"})))
	ops = base.MakeFreepsOperators(&OpMistral{}, cr, ctx)
	res = ops[0].Execute(ctx, "Transcribe", base.MakeEmptyFunctionArguments(), base.MakePlainOutput("RIFF..."))
	assert.Assert(t, !res.IsError())
	got, err := res.GetBytes()
	assert.NilError(t, err)
	assert.Equal(t, string(got), "mach das licht an")
}

func mustWriteSection(t *testing.T, configBytes []byte, conf MistralConfig) []byte {
	newb, err := utils.WriteSection(configBytes, "mistral", conf, false)
	assert.NilError(t, err)
	return newb
}
