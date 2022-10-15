package freepsgraph

import (
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

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

func (m *OpTelegram) sendIOtoChat(chatid int64, io *OperatorIO) *OperatorIO {
	var err error
	var res tgbotapi.Message
	if len(io.ContentType) > 7 && io.ContentType[0:5] == "image" {
		var byt []byte
		byt, err = io.GetBytes()
		if err != nil {
			MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		tphoto := tgbotapi.NewPhoto(chatid, tgbotapi.FileBytes{Name: "picture." + io.ContentType[6:], Bytes: byt})
		res, err = m.bot.Send(tphoto)
	} else {
		msg := tgbotapi.NewMessage(chatid, io.GetString())
		res, err = m.bot.Send(msg)
	}
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, "Error when sending telegram message: %v", err.Error())
	}
	return MakeObjectOutput(res)
}

func (m *OpTelegram) Execute(fn string, vars map[string]string, input *OperatorIO) *OperatorIO {
	chatid, err := strconv.ParseInt(vars["ChatID"], 10, 64)
	if err != nil {
		return MakeOutputError(http.StatusBadRequest, err.Error())
	}
	output := input
	text, ok := vars["Text"]
	if ok {
		output = MakePlainOutput(text)
	}
	return m.sendIOtoChat(chatid, output)
}

func (m *OpTelegram) GetFunctions() []string {
	return []string{"Post"}
}

func (m *OpTelegram) GetPossibleArgs(fn string) []string {
	return []string{"ChatID", "Text"}
}

func (m *OpTelegram) GetArgSuggestions(fn string, arg string, otherArgs map[string]string) map[string]string {
	return map[string]string{}
}
