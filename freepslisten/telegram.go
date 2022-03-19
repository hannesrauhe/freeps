package freepslisten

import (
	"context"
	"encoding/json"
	"fmt"
	"log"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/hannesrauhe/freeps/freepsdo"
	"github.com/hannesrauhe/freeps/utils"
)

type Telegraminator struct {
	Modinator   *freepsdo.TemplateMod
	bot         *tgbotapi.BotAPI
	tgc         *freepsdo.TelegramConfig
	lastMessage int
	chatState   map[int64]TelegramCallbackResponse
}

type TelegramCallbackResponse struct {
	T string `json:",omitempty"` // TemplateActionID
	F bool   `json:",omitempty"` // Finished ?
	P int    `json:",omitempty"` // processed Args
	C string `json:",omitempty"` // last choice
	K bool   `json:",omitempty"` // request to type value instead of choosing
}

func (r *Telegraminator) Shutdown(ctx context.Context) {
	r.bot.StopReceivingUpdates()
}

type ButtonWrapper struct {
	Button tgbotapi.InlineKeyboardButton
	Choice string
}

func (r *Telegraminator) newJSONButton(name string, tcr *TelegramCallbackResponse) *ButtonWrapper {
	if len(name) > 15 {
		name = name[0:15]
	}
	byt, err := json.Marshal(tcr)
	if err != nil {
		panic(err)
	}
	s := string(byt)
	return &ButtonWrapper{Button: tgbotapi.NewInlineKeyboardButtonData(name, s), Choice: tcr.C}
}

func (r *Telegraminator) getReplyKeyboard() tgbotapi.ReplyKeyboardMarkup {
	rows := [][]tgbotapi.KeyboardButton{}
	row := []tgbotapi.KeyboardButton{}
	counter := 0
	for k := range r.Modinator.Mods {
		row = append(row, tgbotapi.NewKeyboardButton(k))
		counter++
		if counter != 0 && counter%3 == 0 {
			rows = append(rows, row)
			row = []tgbotapi.KeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, row)
	}

	return tgbotapi.NewOneTimeReplyKeyboard(rows...)
}

func (r *Telegraminator) getModButtons() []*ButtonWrapper {
	keys := make([]*ButtonWrapper, 0, len(r.Modinator.Mods))
	for k := range r.Modinator.Mods {
		tcr := TelegramCallbackResponse{F: false, P: -1, C: k}
		keys = append(keys, r.newJSONButton(k, &tcr))
	}
	return keys
}

func (r *Telegraminator) getFnButtons(tcr *TelegramCallbackResponse) []*ButtonWrapper {
	ta := r.Modinator.GetTemporaryTemplateAction(tcr.T)
	fn := r.Modinator.Mods[ta.Mod].GetFunctions()
	keys := make([]*ButtonWrapper, 0, len(fn))
	tcr.K = true
	keys = append(keys, r.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for _, k := range fn {
		tcr.C = k
		keys = append(keys, r.newJSONButton(k, tcr))
	}
	return keys
}

func (r *Telegraminator) getArgsButtons(arg string, tcr *TelegramCallbackResponse) []*ButtonWrapper {
	ta := r.Modinator.GetTemporaryTemplateAction(tcr.T)
	argOptions := r.Modinator.Mods[ta.Mod].GetArgSuggestions(ta.Fn, arg, ta.Args)
	keys := make([]*ButtonWrapper, 0, len(argOptions)+2)
	tcr.F = true
	keys = append(keys, r.newJSONButton("<Execute>", tcr))
	tcr.F = false
	keys = append(keys, r.newJSONButton("<Skip "+arg+">", tcr))
	tcr.K = true
	keys = append(keys, r.newJSONButton("<CUSTOM>", tcr))
	tcr.K = false
	for k, v := range argOptions {
		tcr.C = v
		keys = append(keys, r.newJSONButton(k, tcr))
	}
	return keys
}

func (r *Telegraminator) multiChoiceKeyboard(buttons []*ButtonWrapper) (tgbotapi.InlineKeyboardMarkup, string) {
	rows := [][]tgbotapi.InlineKeyboardButton{}
	row := []tgbotapi.InlineKeyboardButton{}
	counter := 0
	addVals := ""
	for _, b := range buttons {
		if len(*b.Button.CallbackData) > 60 {
			addVals += " " + b.Choice
			continue
		}
		row = append(row, b.Button)
		counter++
		if counter != 0 && counter%3 == 0 {
			rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
			row = []tgbotapi.InlineKeyboardButton{}
		}
	}
	if len(row) > 0 {
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(row...))
	}
	return tgbotapi.NewInlineKeyboardMarkup(rows...), addVals
}

func (r *Telegraminator) getModKeyboard() (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getModButtons())
}

func (r *Telegraminator) getFnKeyboard(tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getFnButtons(tcr))
}

func (r *Telegraminator) getArgsKeyboard(arg string, tcr *TelegramCallbackResponse) (tgbotapi.InlineKeyboardMarkup, string) {
	return r.multiChoiceKeyboard(r.getArgsButtons(arg, tcr))
}

func (r *Telegraminator) sendMessage(msg *tgbotapi.MessageConfig) {
	if r.lastMessage > 0 {
		d := tgbotapi.NewDeleteMessage(msg.ChatID, r.lastMessage)
		r.bot.Send(d)
		r.lastMessage = 0
	}
	m, err := r.bot.Send(*msg)
	if err != nil {
		log.Println(err)
		return
	}
	r.lastMessage = m.MessageID
}

func (r *Telegraminator) sendStartMessage(msg *tgbotapi.MessageConfig) {
	r.Modinator.RemoveTemporaryTemplate(fmt.Sprint(msg.ChatID))
	msg.ReplyMarkup, _ = r.getModKeyboard()
	r.sendMessage(msg)
}

func (r *Telegraminator) Respond(chat *tgbotapi.Chat, callbackData string, inputText string) {
	msg := tgbotapi.NewMessage(chat.ID, "Hello "+chat.FirstName+".")
	allowed := false
	for _, v := range r.tgc.AllowedUsers {
		if v == chat.UserName {
			allowed = true
			break
		}
	}
	if !allowed {
		msg.Text += " I'm not allowed to talk to you."
		if _, err := r.bot.Send(msg); err != nil {
			log.Println(err)
		}
		return
	}

	tcr, ok := r.chatState[chat.ID]
	if !ok {
		tcr = TelegramCallbackResponse{}
		if callbackData != "" {
			// a button on the InlineKeyboard was pressed
			byt := []byte(callbackData)
			err := json.Unmarshal(byt, &tcr)
			if err != nil {
				msg.Text = err.Error()
				r.sendStartMessage(&msg)
				return
			}
		} else {
			// inputText contains the mod to use
			r.Modinator.RemoveTemporaryTemplate(fmt.Sprint(msg.ChatID))
			tcr.P = -1
			tcr.C = inputText
		}
	} else {
		// the user was asked to provide input
		tcr.C = inputText
		delete(r.chatState, chat.ID)
	}
	tcr.T = fmt.Sprint(chat.ID)
	tpl := r.Modinator.GetTemporaryTemplateAction(tcr.T)

	if len(tpl.Mod) == 0 {
		tpl.Mod = tcr.C
		if _, ok := r.Modinator.Mods[tpl.Mod]; !ok {
			msg.Text += " Please pick a Mod"
			r.sendStartMessage(&msg)
			return
		}
		msg.Text = "Pick a function for " + tpl.Mod
		msg.ReplyMarkup, _ = r.getFnKeyboard(&tcr)
	} else if len(tpl.Fn) == 0 {
		if tcr.K {
			msg.Text = "Type a function for " + tpl.Mod
			tcr.K = false
			r.chatState[chat.ID] = tcr
		} else {
			tpl.Fn = tcr.C
		}
	}

	// sanity check
	if _, ok := r.Modinator.Mods[tpl.Mod]; !ok {
		msg.Text += " Something went wrong. Please pick a Mod"
		r.sendStartMessage(&msg)
		return
	}

	if len(tpl.Fn) > 0 && !tcr.F {
		args := r.Modinator.Mods[tpl.Mod].GetPossibleArgs(tpl.Fn)
		if tcr.P == 0 {
			tpl.Args = map[string]interface{}{}
		}
		if tcr.K {
			msg.Text = fmt.Sprintf("Type a Value for %s (%s/%s)", args[tcr.P], tpl.Mod, tpl.Fn)
			tcr.K = false
			r.chatState[chat.ID] = tcr
		} else {
			if tcr.P >= 0 {
				tpl.Args[args[tcr.P]] = tcr.C
			}
			tcr.C = ""
			tcr.P++
			if tcr.P >= len(args) {
				tcr.F = true
			} else {
				addVals := ""
				msg.Text = fmt.Sprintf("Pick a Value for %s (%s/%s)", args[tcr.P], tpl.Mod, tpl.Fn)
				msg.ReplyMarkup, addVals = r.getArgsKeyboard(args[tcr.P], &tcr)
				if len(addVals) > 0 {
					// do not use SendMessage, because that message gets deleted.... yeah, I need to clean this up
					r.bot.Send(tgbotapi.NewMessage(chat.ID, "More values:"+addVals+"."))
				}
			}
		}
	}

	if tcr.F {
		jrw := freepsdo.NewResponseCollector(fmt.Sprintf("telegram: %v", chat.FirstName))
		r.Modinator.ExecuteTemplateAction(tpl, jrw)
		status, otype, byt := jrw.GetFinalResponse(true)
		if otype == "image/jpeg" {
			msg.Text = "Here is a picture for you"
			m := tgbotapi.NewPhoto(chat.ID, tgbotapi.FileBytes{Name: "raspistill.jpg", Bytes: byt})
			if _, err := r.bot.Send(m); err != nil {
				log.Println(err)
			}
		} else {
			msg.Text = fmt.Sprintf("%v: %q", status, byt)
		}
		r.Modinator.RemoveTemporaryTemplate(fmt.Sprint(msg.ChatID))
		msg.ReplyMarkup = r.getReplyKeyboard()
	}
	r.sendMessage(&msg)
}

func (r *Telegraminator) MainLoop() {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := r.bot.GetUpdatesChan(u)

	for update := range updates {
		if update.CallbackQuery != nil {
			r.Respond(update.CallbackQuery.Message.Chat, update.CallbackQuery.Data, "")
			continue
		}
		if update.Message == nil { // ignore any non-Message updates
			continue
		}
		r.Respond(update.Message.Chat, "", update.Message.Text)
	}
}

func NewTelegramBot(cr *utils.ConfigReader, doer *freepsdo.TemplateMod, cancel context.CancelFunc) *Telegraminator {
	tgc := freepsdo.DefaultTelegramConfig
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
	t := &Telegraminator{Modinator: doer, bot: bot, tgc: &tgc, chatState: make(map[int64]TelegramCallbackResponse)}
	if err != nil {
		log.Printf("Error on Telegram registration: %v", err)
		return t
	}
	bot.Debug = tgc.DebugMessages

	log.Printf("Authorized on account %s", bot.Self.UserName)

	go t.MainLoop()
	return t
}
