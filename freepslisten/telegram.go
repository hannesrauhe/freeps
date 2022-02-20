package freepslisten

import (
	"context"
	"fmt"
	"log"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type TelegramConfig struct {
	Token string
}

var DefaultTelegramConfig = TelegramConfig{Token: ""}

type Telegraminator struct {
	Modinator *freepsdo.TemplateMod
	bot       *tgbotapi.BotAPI
}

func (r *Telegraminator) Shutdown(ctx context.Context) {
	r.bot.StopReceivingUpdates()
}

func (r *Telegraminator) optionKeyboard(buttons map[string]string) tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(tgbotapi.NewInlineKeyboardButtonData(bla, "/template/"+bla))
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func (r *Telegraminator) getModKeyboard(mod string) tgbotapi.InlineKeyboardMarkup {
	return r.optionKeyboard()
}

func (r *Telegraminator) MainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			// callback := tgbotapi.NewCallback(update.CallbackQuery.ID, update.CallbackQuery.Data)
			// if _, err := r.bot.Request(callback); err != nil {
			// 	panic(err)
			// }
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "Modes for "+update.CallbackQuery.Data)

			parts := strings.Split(update.CallbackQuery.Data, "/")
			if len(parts) == 1 {
				msg.ReplyMarkup = r.getModKeyboard(parts[0])
			}

			if _, err := r.bot.Send(msg); err != nil {
				panic(err)
			}
			continue
		}
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		if !update.Message.IsCommand() { // ignore any non-command Messages
			msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Hello")
			msg.ReplyMarkup = r.optionKeyboard("template")
			// Send the message.
			if _, err := r.bot.Send(msg); err != nil {
				log.Println(err)
			}
		}

		// Create a new MessageConfig. We don't have text yet,
		// so we leave it empty.
		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "")
		jrw := freepsdo.NewResponseCollector()
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
