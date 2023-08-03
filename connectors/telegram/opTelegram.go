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

// GetDefaultConfig returns a copy of the default config
func (m *OpTelegram) GetDefaultConfig() interface{} {
	return &TelegramConfig{Token: ""}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (m *OpTelegram) InitCopyOfOperator(config interface{}, ctx *base.Context) (base.FreepsOperatorWithConfig, error) {
	if bot != nil {
		return nil, fmt.Errorf("Only one instance of telegram is allowed")
	}

	newM := OpTelegram{tgc: *config.(*TelegramConfig)}
	if newM.tgc.Token == "" {
		return nil, fmt.Errorf("Telegram token is empty")
	}
	var err error
	bot, err = tgbotapi.NewBotAPI(newM.tgc.Token)
	if err != nil {
		return nil, err
	}
	tgc = &newM.tgc
	bot.Debug = m.tgc.DebugMessages
	return &newM, nil
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
