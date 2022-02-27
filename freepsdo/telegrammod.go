package freepsdo

import (
	"encoding/json"
	"log"
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

type TelegramMod struct {
	bot *tgbotapi.BotAPI
	tgc *TelegramConfig
}

var _ Mod = &TelegramMod{}

func NewTelegramBot(cr *utils.ConfigReader) *TelegramMod {
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
	t := &TelegramMod{bot: bot, tgc: &tgc}
	if err != nil {
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}
	bot.Debug = tgc.DebugMessages

	return t
}

func (m *TelegramMod) DoWithJSON(function string, jsonStr []byte, jrw *ResponseCollector) {
	var vars map[string]string
	json.Unmarshal(jsonStr, &vars)

	chatid, _ := strconv.ParseInt(vars["ChatID"], 10, 64)
	msg := tgbotapi.NewMessage(chatid, vars["Text"])
	m.bot.Send(msg)
}

func (m *TelegramMod) GetFunctions() []string {
	return []string{"Post"}
}

func (m *TelegramMod) GetPossibleArgs(fn string) []string {
	return []string{"ChatID", "Text"}
}

func (m *TelegramMod) GetArgSuggestions(fn string, arg string, otherArgs map[string]interface{}) map[string]string {
	return map[string]string{}
}
