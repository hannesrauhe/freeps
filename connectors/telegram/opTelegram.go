//go:build !notelegram

package telegram

import (
	"fmt"
	"net/http"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/base"
	"github.com/hannesrauhe/freeps/freepsgraph"
	"github.com/hannesrauhe/freeps/utils"
)

type ChatState struct {
	Chat             *tgbotapi.Chat
	CallbackResponse *TelegramCallbackResponse
}

type OpTelegram struct {
	GE          *freepsgraph.GraphEngine
	tgc         TelegramConfig
	lastMessage int
	closeChan   chan int
	bot         *tgbotapi.BotAPI
}

var _ base.FreepsOperatorWithConfig = &OpTelegram{}
var _ base.FreepsOperatorWithShutdown = &OpTelegram{}

// GetDefaultConfig returns a copy of the default config
func (m *OpTelegram) GetDefaultConfig() interface{} {
	return &TelegramConfig{Enabled: true, Token: "", AllowedUsers: []string{}, DebugMessages: false, StoreChatNamespace: "_telegram_chats"}
}

// InitCopyOfOperator creates a copy of the operator and initializes it with the given config
func (m *OpTelegram) InitCopyOfOperator(ctx *base.Context, config interface{}, name string) (base.FreepsOperatorWithConfig, error) {
	newM := OpTelegram{GE: m.GE, tgc: *config.(*TelegramConfig), closeChan: make(chan int)}
	if newM.tgc.Token == "" {
		return nil, fmt.Errorf("Telegram token is empty")
	}
	bot, err := tgbotapi.NewBotAPI(newM.tgc.Token)
	if err != nil {
		return nil, err
	}
	bot.Debug = m.tgc.DebugMessages
	newM.bot = bot

	ctx.GetLogger().WithField("component", "telegram").Infof("Authorized on account %s", bot.Self.UserName)
	return &newM, nil
}

// PostArgs are the arguments for the Post function
type PostArgs struct {
	ChatID int64
	Text   *string
}

func (a *PostArgs) ChatIDSuggestions(op base.FreepsOperator) map[string]string {
	m := op.(*OpTelegram)
	return m.getRecentChats()
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
		res, err = m.bot.Send(tphoto)
	} else {
		msg := tgbotapi.NewMessage(args.ChatID, input.GetString())
		res, err = m.bot.Send(msg)
	}
	if err != nil {
		return base.MakeOutputError(http.StatusBadRequest, "Error when sending telegram message: %v", err.Error())
	}
	return base.MakeObjectOutput(res)
}

func (m *OpTelegram) StartListening(ctx *base.Context) {
	go m.mainLoop()
}

func (m *OpTelegram) Shutdown(ctx *base.Context) {
	m.bot.StopReceivingUpdates()
	<-m.closeChan
	m.bot = nil
}
