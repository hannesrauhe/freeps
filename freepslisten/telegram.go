package freepslisten

import (
	"context"
	"fmt"
	"log"
	"os"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type Telegraminator struct {
	Modinator *freepsdo.TemplateMod
	bot       *tgbotapi.BotAPI
}

func (r *Telegraminator) Shutdown(ctx context.Context) {
	r.bot.StopReceivingUpdates()
}

func (r *Telegraminator) MainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			continue
		}

		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		jrw := freepsdo.NewJsonResponseWriterPrintDirectly()
		r.Modinator.ExecuteModWithJson(update.Message.Command(), update.Message.CommandArguments(), []byte("{}"), jrw)
		m, err := jrw.GetMarshalledOutput()
		if err != nil {
			log.Printf("Error on Telegram: %v", err)
			msg.Text = fmt.Sprintf("Error: %v", err)
		} else {
			msg.Text = fmt.Sprintf("%q", m)
		}
		if _, err := r.bot.Send(msg); err != nil {
			log.Println(err)
		}
	}
}

func NewTelegramBot(cr *utils.ConfigReader, doer *freepsdo.TemplateMod, cancel context.CancelFunc) *Telegraminator {
	bot, err := tgbotapi.NewBotAPI(os.Getenv("TELEGRAM_APITOKEN"))
	t := &Telegraminator{Modinator: doer, bot: bot}
	if err != nil {
		log.Printf("Error on Telegram: %v", err)
		return t
	}
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)
	bot.GetMyCommands()

	go t.MainLoop()
	return t
}
