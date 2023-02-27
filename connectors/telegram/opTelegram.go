package telegram

import (
	"net/http"
	"strconv"

	log "github.com/sirupsen/logrus"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type OpTelegram struct {
	bot *tgbotapi.BotAPI
	tgc *TelegramConfig
}

var _ freepsgraph.FreepsOperator = &OpTelegram{}

// GetName returns the name of the operator
func (o *OpTelegram) GetName() string {
	return "telegram"
}

func NewTelegramOp(cr *utils.ConfigReader) *OpTelegram {
	bot, tgc, err := newTgbotFromConfig(cr)
	t := &OpTelegram{bot: bot, tgc: tgc}
	if err != nil {
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}

	return t
}

func (m *OpTelegram) sendIOtoChat(chatid int64, io *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	var err error
	var res tgbotapi.Message
	if len(io.ContentType) > 7 && io.ContentType[0:5] == "image" {
		var byt []byte
		byt, err = io.GetBytes()
		if err != nil {
			freepsgraph.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		tphoto := tgbotapi.NewPhoto(chatid, tgbotapi.FileBytes{Name: "picture." + io.ContentType[6:], Bytes: byt})
		res, err = m.bot.Send(tphoto)
	} else {
		msg := tgbotapi.NewMessage(chatid, io.GetString())
		res, err = m.bot.Send(msg)
	}
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, "Error when sending telegram message: %v", err.Error())
	}
	return freepsgraph.MakeObjectOutput(res)
}

func (m *OpTelegram) Execute(ctx *base.Context, fn string, vars map[string]string, input *freepsgraph.OperatorIO) *freepsgraph.OperatorIO {
	chatid, err := strconv.ParseInt(vars["ChatID"], 10, 64)
	if err != nil {
		return freepsgraph.MakeOutputError(http.StatusBadRequest, err.Error())
	}
	output := input
	text, ok := vars["Text"]
	if ok {
		output = freepsgraph.MakePlainOutput(text)
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

// Shutdown (noOp)
func (o *OpTelegram) Shutdown(ctx *base.Context) {
}
