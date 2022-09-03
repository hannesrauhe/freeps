package freepsgraph

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/utils"
)

type TelegramConfig struct {
	Token         string
	AllowedUsers  []string
	DebugMessages bool
}

var DefaultTelegramConfig = TelegramConfig{Token: ""}

type OpTelegram struct {
	bot *tgbotapi.BotAPI
	tgc *TelegramConfig
}

var _ FreepsOperator = &OpTelegram{}

func NewTelegramBot(cr *utils.ConfigReader) *OpTelegram {
	tgc := DefaultTelegramConfig
	err := cr.ReadSectionWithDefaults("telegrambot", &tgc)
	if err != nil {
		log.Fatal(err)
	}
	cr.WriteBackConfigIfChanged()
	if err != nil {
		log.Print(err)
	}

	if tgc.Token == "" {
		return nil
	}

	bot, err := tgbotapi.NewBotAPI(tgc.Token)
	t := &OpTelegram{bot: bot, tgc: &tgc}
	if err != nil {
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}
	bot.Debug = tgc.DebugMessages

	return t
}

func (m *OpTelegram) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	chatid, err := strconv.ParseInt(vars["ChatID"], 10, 64)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}
	text := vars["Text"]
	if text == "" {
		jsonStr, err := json.MarshalIndent(vars, "", "  ")
		if err != nil {
			return MakeOutputError(http.StatusBadRequest, err.Error())
		}
		text = string(jsonStr)
	}
	msg := tgbotapi.NewMessage(chatid, text)
	res, err := m.bot.Send(msg)

	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}
	return MakeObjectOutput(res)
}

func (m *OpTelegram) GetFunctions() []string {
	return []string{"Post"}
}

func (m *OpTelegram) GetPossibleArgs(fn string) []string {
	return []string{"ChatID", "Text"}
}

func (m *OpTelegram) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	return map[string]string{}
}
