package freepslisten

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"

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

func (r *Telegraminator) newBase64Button(name string, ta *freepsdo.TemplateAction) tgbotapi.InlineKeyboardButton {
	byt, err := json.Marshal(ta)
	if err != nil {
		panic(err)
	}
	return tgbotapi.NewInlineKeyboardButtonData(name, base64.StdEncoding.EncodeToString(byt))
}

func (r *Telegraminator) getModButtons() []tgbotapi.InlineKeyboardButton {
	keys := make([]tgbotapi.InlineKeyboardButton, 0, len(r.Modinator.Mods))
	for k := range r.Modinator.Mods {
		ta := freepsdo.TemplateAction{Mod: k}
		keys = append(keys, r.newBase64Button(k, &ta))
	}
	return keys
}

func (r *Telegraminator) getFnButtons(ta *freepsdo.TemplateAction) []tgbotapi.InlineKeyboardButton {
	fn := r.Modinator.Mods[ta.Mod].GetFunctions()
	keys := make([]tgbotapi.InlineKeyboardButton, 0, len(fn))
	for _, k := range fn {
		ta.Fn = k
		keys = append(keys, r.newBase64Button(k, ta))
	}
	return keys
}

func (r *Telegraminator) optionKeyboard(buttons []tgbotapi.InlineKeyboardButton) tgbotapi.InlineKeyboardMarkup {
	row := tgbotapi.NewInlineKeyboardRow(buttons...)
	return tgbotapi.NewInlineKeyboardMarkup(row)
}

func (r *Telegraminator) getModKeyboard() tgbotapi.InlineKeyboardMarkup {
	return r.optionKeyboard(r.getModButtons())
}

func (r *Telegraminator) getFnKeyboard(ta *freepsdo.TemplateAction) tgbotapi.InlineKeyboardMarkup {
	return r.optionKeyboard(r.getFnButtons(ta))
}

func (r *Telegraminator) sendStartMessage(msg *tgbotapi.MessageConfig) {
	msg.ReplyMarkup = r.getModKeyboard()
	// Send the message.
	if _, err := r.bot.Send(*msg); err != nil {
		log.Println(err)
	}
}

func (r *Telegraminator) MainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			msg := tgbotapi.NewMessage(update.CallbackQuery.Message.Chat.ID, "")
			ta := freepsdo.TemplateAction{}
			byt, err := base64.StdEncoding.DecodeString(update.CallbackQuery.Data)
			if err != nil {
				msg.Text = err.Error()
				r.sendStartMessage(&msg)
				continue
			}
			err = json.Unmarshal(byt, &ta)
			if err != nil {
				msg.Text = err.Error()
				r.sendStartMessage(&msg)
				continue
			}

			if len(ta.Fn) == 0 {
				msg.Text = "Pick a function"
				msg.ReplyMarkup = r.getFnKeyboard(&ta)
			} else {
				jrw := freepsdo.NewResponseCollector()
				r.Modinator.ExecuteTemplateAction(&ta, jrw)
				status, otype, byt := jrw.GetFinalResponse()
				if otype == "image/jpeg" {
					msg.Text = "Here is a picture for you"
					m := tgbotapi.NewPhoto(update.CallbackQuery.Message.Chat.ID, tgbotapi.FileBytes{Name: "raspistill.jpg", Bytes: byt})
					if _, err := r.bot.Send(m); err != nil {
						log.Println(err)
					}
				} else {
					msg.Text = fmt.Sprintf("%v: %q", status, byt)
				}
			}
			if _, err := r.bot.Send(msg); err != nil {
				log.Println(err)
			}
			continue
		}
		if update.Message == nil { // ignore any non-Message updates
			continue
		}

		msg := tgbotapi.NewMessage(update.Message.Chat.ID, "Hello")
		r.sendStartMessage(&msg)
		continue

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
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}
	bot.Debug = true

	log.Printf("Authorized on account %s", bot.Self.UserName)
	bot.GetMyCommands()

	go t.MainLoop()
	return t
}
