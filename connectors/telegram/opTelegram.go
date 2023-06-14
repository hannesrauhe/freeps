package telegram

import (
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/utils"
)

type OpTelegram struct {
	tgc TelegramConfig
}

var _ base.FreepsOperatorWithConfig = &OpTelegram{}

// GetConfig returns the config struct of the operator that is filled with the default values
func (m *OpTelegram) GetConfig() interface{} {
	m.tgc = DefaultTelegramConfig
	return &m.tgc
}

// Init is called after the config is read and the operator is created
func (m *OpTelegram) Init(ctx *base.Context) error {
	if m.tgc.Token == "" {
		return fmt.Errorf("Telegram token is empty")
	}
	var err error
	bot, err = tgbotapi.NewBotAPI(m.tgc.Token)
	if err != nil {
		return err
	}
	tgc = &m.tgc
	bot.Debug = m.tgc.DebugMessages
	return nil
}

// PostArgs are the arguments for the Post function
type PostArgs struct {
	ChatID int64
	Text   *string
}

// Post sends a message to a chat
func (m *OpTelegram) Post(ctx *base.Context, input *base.OperatorIO, args PostArgs) *base.OperatorIO {
	var err error
	var res tgbotapi.Message
	if input == nil || input.IsEmpty() {
		if args.Text == nil || *args.Text == "" {
			return base.MakeOutputError(http.StatusBadRequest, "Empty message")
		}
		input = base.MakePlainOutput(*args.Text)
	}

	if utils.StringStartsWith(input.ContentType, "image") {
		var byt []byte
		byt, err = input.GetBytes()
		if err != nil {
			base.MakeOutputError(http.StatusInternalServerError, err.Error())
		}
		tphoto := tgbotapi.NewPhoto(args.ChatID, tgbotapi.FileBytes{Name: "picture." + input.ContentType[6:], Bytes: byt})
		res, err = bot.Send(tphoto)
	} else {
		msg := tgbotapi.NewMessage(args.ChatID, input.GetString())
		res, err = bot.Send(msg)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when sending telegram message: %v", err.Error())
	}
	return base.MakeObjectOutput(res)
}
