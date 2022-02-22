package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/linebot"
)

var dayOfWeekData = "action=dayOfWeek&day=%s"

var configMessage = "ゴミ捨て日の設定をするよ。 曜日を選択してね!"

var wdays = [...]string{"日", "月", "火", "水", "木", "金", "土"}

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading env: %v", err)
	}

	bot, err := linebot.New(os.Getenv("LINEBOT_SECRET_TOKEN"), os.Getenv("LINEBOT_CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Fatal(err)
	}

	log.Print(bot)

	// Setup HTTP Server for receiving requests from LINE platform
	http.HandleFunc("/callback", func(w http.ResponseWriter, req *http.Request) {
		events, err := bot.ParseRequest(req)
		if err != nil {
			if err == linebot.ErrInvalidSignature {
				w.WriteHeader(400)
			} else {
				w.WriteHeader(500)
			}
			return
		}
		for _, event := range events {
			if event.Type == linebot.EventTypeMessage {
				switch message := event.Message.(type) {
				case *linebot.TextMessage:
					sendReplyMessage(bot, event, message)
				case *linebot.StickerMessage:
					replyMessage := fmt.Sprintf(
						"sticker id is %s, stickerResourceType is %s", message.StickerID, message.StickerResourceType)
					if _, err = bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(replyMessage)).Do(); err != nil {
						log.Print(err)
					}
				}
			}
		}
	})
	// This is just sample code.
	// For actual use, you must support HTTPS by using `ListenAndServeTLS`, a reverse proxy or something else.
	if err := http.ListenAndServe(":"+os.Getenv("PORT"), nil); err != nil {
		log.Fatal(err)
	}
}

func sendReplyMessage(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage) {
	pas := []*linebot.PostbackAction{}
	for _, wday := range wdays {
		data := fmt.Sprintf(dayOfWeekData, wday)
		pas = append(pas, linebot.NewPostbackAction(wday, data, "", ""))
	}

	var actions = make([]linebot.TemplateAction, 0)
	for _, pa := range pas {
		actions = append(actions, pa)
	}

	// actionは4つまで
	bt := linebot.NewButtonsTemplate(
		"",
		"曜日を選択してね!",
		"月~木",
		actions[:4]...,
	)

	bt2 := linebot.NewButtonsTemplate(
		"",
		"曜日を選択してね!",
		"金~日",
		actions[4:]...,
	)
	if message.Text == "設定" {
		if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(configMessage), linebot.NewTemplateMessage("曜日ボタン", bt), linebot.NewTemplateMessage("曜日ボタン", bt2)).Do(); err != nil {
			log.Print(err)
		}
	}
}
