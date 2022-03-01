package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/guregu/dynamo"
	"github.com/joho/godotenv"
	"github.com/line/line-bot-sdk-go/linebot"
)

var configMessage = "ゴミ捨て日の設定をするよ。 曜日を選択してね!"

var wdays = [...]string{"日", "月", "火", "水", "木", "金", "土"}

type UserConfig struct {
	UserID           string `dynamo:"UserID,hash"`
	DayOfWeek        string `dynamo:"DayOfWeek,range"`
	Content          string `dynamo:"Content"`
	NotificationTime string `dynamo:"NotificationTime"`
	InteractiveFlag  int    `dynamo:"InteractiveFlag" localIndex:"index-2,range"`
}

const AWS_REGION = "ap-northeast-1"
const DYNAMO_ENDPOINT = "http://localhost:8000"

func main() {
	err := godotenv.Load(".env")

	if err != nil {
		log.Fatalf("Error loading env: %v", err)
	}

	sess, err := session.NewSession(&aws.Config{
		Region:      aws.String(AWS_REGION),
		Endpoint:    aws.String(DYNAMO_ENDPOINT),
		Credentials: credentials.NewStaticCredentials("hoge", "huga", "hogehuga"),
	})
	if err != nil {
		log.Fatal(err)
	}
	db := dynamo.New(sess)

	db.Table("UserConfig").DeleteTable().Run()

	err = db.CreateTable("UserConfig", UserConfig{}).Run()
	if err != nil {
		log.Print(err)
	}

	table := db.Table("UserConfig")

	bot, err := linebot.New(os.Getenv("LINEBOT_SECRET_TOKEN"), os.Getenv("LINEBOT_CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Fatal(err)
	}

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
					sendReplyMessage(bot, event, message, table)
				}
			} else if event.Type == linebot.EventTypePostback {
				if event.Postback.Data == "time" {
					createTime(bot, event, table)
				} else {
					createUserConfig(bot, event, table)
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

func createTime(bot *linebot.Client, event *linebot.Event, table dynamo.Table) {
	var user UserConfig
	err := table.Get("UserID", event.Source.UserID).Range("InteractiveFlag", dynamo.Equal, 2).Index("index-2").One(&user)
	if err != nil {
		log.Fatal(err)
	}

	err = table.Update("UserID", event.Source.UserID).Range("DayOfWeek", user.DayOfWeek).Set("NotificationTime", event.Postback.Params.Time).Set("InteractiveFlag", 0).Value(&user)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("了解-!%s曜日に\n\n%sを%sに通知するよ!\n\n変更したかったらまた再度設定してね-!", user.DayOfWeek, user.Content, event.Postback.Params.Time))).Do(); err != nil {
		log.Print(err)
	}
}

func createUserConfig(bot *linebot.Client, event *linebot.Event, table dynamo.Table) {
	var user UserConfig

	err := table.Put(&UserConfig{UserID: event.Source.UserID, DayOfWeek: event.Postback.Data, InteractiveFlag: 1}).Run()
	if err != nil {
		log.Fatal(err)
	}

	err = table.Get("UserID", event.Source.UserID).Range("DayOfWeek", dynamo.Equal, event.Postback.Data).One(&user)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s曜日は何ごみを捨てる日にする?\n\nメッセージで教えてね!", event.Postback.Data))).Do(); err != nil {
		log.Print(err)
	}

}

func sendTimeConfig(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, user UserConfig) {
	time := linebot.NewButtonsTemplate(
		"",
		"通知時間を選択してね!",
		"00:00 ~ 23:59",
		linebot.NewDatetimePickerAction("Time", "time", "time", "", "23:59", "00:00"),
	)
	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s曜日に%sを通知するよ!\n\n何時に通知して欲しいか選んでね!", user.DayOfWeek, message.Text)), linebot.NewTemplateMessage("時間設定", time)).Do(); err != nil {
		log.Print(err)
	}
}

func updateDayOfWeek(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, user UserConfig, table dynamo.Table) {
	err := table.Update("UserID", event.Source.UserID).Range("DayOfWeek", user.DayOfWeek).Set("Content", message.Text).Set("InteractiveFlag", 2).Value(&user)
	if err != nil {
		log.Fatal(err)
	}
	sendTimeConfig(bot, event, message, user)
}

func sendReplyMessage(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, table dynamo.Table) {
	var user UserConfig
	err := table.Get("UserID", event.Source.UserID).Range("InteractiveFlag", dynamo.Equal, 1).Index("index-2").One(&user)
	if user.UserID != "" {
		updateDayOfWeek(bot, event, message, user, table)
	}
	if err != nil {
		log.Print(err)
	}

	if message.Text == "設定" {
		bt, bt2 := makeButtonTemplate()
		if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(configMessage), linebot.NewTemplateMessage("曜日ボタン", bt), linebot.NewTemplateMessage("曜日ボタン", bt2)).Do(); err != nil {
			log.Print(err)
		}
	}
}

func makeButtonTemplate() (*linebot.ButtonsTemplate, *linebot.ButtonsTemplate) {
	pas := []*linebot.PostbackAction{}
	for _, wday := range wdays {
		pas = append(pas, linebot.NewPostbackAction(wday, wday, "", ""))
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

	return bt, bt2
}
