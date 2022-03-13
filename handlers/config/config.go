package main

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/guregu/dynamo"
	"github.com/line/line-bot-sdk-go/linebot"
	"github.com/shotaoni/notification-gc-day-line-bot/bot"
	"github.com/shotaoni/notification-gc-day-line-bot/db"
	"github.com/shotaoni/notification-gc-day-line-bot/model"
	"github.com/shotaoni/notification-gc-day-line-bot/utils"
)

func handler(request events.APIGatewayProxyRequest) (events.APIGatewayProxyResponse, error) {
	// TODO 消す
	log.Print("Header", request.Headers)
	log.Print("Body", request.Body)

	if !validateSignature(os.Getenv("LINEBOT_SECRET_TOKEN"), request.Headers["x-line-signature"], []byte(request.Body)) {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       fmt.Sprintf(`{"message":"%s"}`+"\n", linebot.ErrInvalidSignature.Error()),
		}, nil
	}

	webhook := model.Webhook{}

	if err := json.Unmarshal([]byte(request.Body), &webhook); err != nil {
		return events.APIGatewayProxyResponse{
			StatusCode: http.StatusBadRequest,
			Body:       fmt.Sprintf(`{"message":"%s"}`+"\n", http.StatusText(http.StatusBadRequest)),
		}, nil
	}

	table, err := db.ConnectTable("UserConfig")
	if err != nil {
		log.Fatal(err)
	}

	bot, err := bot.NewLineBotClient()

	if err != nil {
		log.Fatal(err)
	}

	for _, event := range webhook.Events {
		if event.Type == linebot.EventTypeMessage {
			switch message := event.Message.(type) {
			case *linebot.TextMessage:
				sendReplyMessage(bot, event, message, *table)
			}
		} else if event.Type == linebot.EventTypePostback {
			replyMessageByPostBack(bot, event, *table)
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func replyMessageByPostBack(bot *linebot.Client, event *linebot.Event, table dynamo.Table) {
	dataMap := makeDataMap(event.Postback.Data)
	log.Print(dataMap)
	if dataMap["action"] == "deleteUserConfig" {
		deleteUserConfig(bot, event, table, dataMap["dayOfWeek"])
	} else if dataMap["action"] == "createUserConfig" {
		createUserConfig(bot, event, table, dataMap["dayOfWeek"])
	} else if dataMap["action"] == "createTime" {
		createTime(bot, event, table)
	}
}

func deleteUserConfig(bot *linebot.Client, event *linebot.Event, table dynamo.Table, dayOfWeek string) {
	err := table.Delete("UserID", event.Source.UserID).Range("DayOfWeek", dayOfWeek).Run()
	if err != nil {
		log.Fatal(err)
	}

	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s曜日の通知を削除したよ!\n", dayOfWeek))).Do(); err != nil {
		log.Print(err)
	}
}

func makeDataMap(data string) map[string]string {
	dataMap := make(map[string]string)

	arr := strings.Split(data, "&")

	for _, data := range arr {
		splitedData := strings.Split(data, "=")
		dataMap[splitedData[0]] = splitedData[1]
	}
	return dataMap
}

func validateSignature(channelSecret string, signature string, body []byte) bool {
	decoded, err := base64.StdEncoding.DecodeString(signature)
	if err != nil {
		return false
	}

	hash := hmac.New(sha256.New, []byte(channelSecret))
	_, err = hash.Write(body)
	if err != nil {
		return false
	}

	return hmac.Equal(decoded, hash.Sum(nil))
}

func resetInteractiveFlag(userID string, users []model.UserConfig, table dynamo.Table) {
	// 複数ボタン押下対応
	for _, u := range users {
		err := table.Update("UserID", userID).Range("DayOfWeek", u.DayOfWeek).Set("InteractiveFlag", 0).Value(&u)
		if err != nil {
			log.Print(err)
		}
	}
}

func sendReplyMessage(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, table dynamo.Table) {
	users := []model.UserConfig{}

	err := table.Get("UserID", event.Source.UserID).Range("InteractiveFlag", dynamo.Equal, 1).Index("index-2").All(&users)

	if (len(users)) > 0 {
		resetInteractiveFlag(event.Source.UserID, users, table)
		updateDayOfWeek(bot, event, message, users[len(users)-1], table)
		return
	}
	if err != nil {
		log.Print(err)
	}

	switch message.Text {
	case "設定":
		bt, bt2 := makeButtonTemplate()
		if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("ゴミ捨て日の設定をするよ。 曜日を選択してね!"), linebot.NewTemplateMessage("曜日ボタン", bt), linebot.NewTemplateMessage("曜日ボタン", bt2)).Do(); err != nil {
			log.Print(err)
		}
		return
	case "削除":
		bt, bt2 := makeDeleteButtonTemplate(event.Source.UserID, table)
		switch {
		case bt == nil && bt2 == nil:
			if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("登録しているデータはないみたい!登録するときは\"設定\"と入力してね!")).Do(); err != nil {
				log.Print(err)
			}
			return
		case bt2 == nil:
			if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("ゴミ捨て日の通知削除をするよ。 削除したい曜日を選択してね!"), linebot.NewTemplateMessage("曜日ボタン", bt)).Do(); err != nil {
				log.Print(err)
			}
			return
		default:
			if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage("ゴミ捨て日の通知削除をするよ。 削除したい曜日を選択してね!"), linebot.NewTemplateMessage("曜日ボタン", bt), linebot.NewTemplateMessage("曜日ボタン", bt2)).Do(); err != nil {
				log.Print(err)
			}
		}
	default:
		if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s??ちょっと理解ができない言葉みたい...\n\n通知設定をしたい時は\"設定\"と入力、通知の削除をしたい時は\"削除\"と入力してね!", message.Text))).Do(); err != nil {
			log.Print(err)
		}
	}
}

func createTime(bot *linebot.Client, event *linebot.Event, table dynamo.Table) {
	user := model.UserConfig{}

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

func createUserConfig(bot *linebot.Client, event *linebot.Event, table dynamo.Table, dayOfWeek string) {
	user := model.UserConfig{}

	err := table.Put(model.UserConfig{UserID: event.Source.UserID, DayOfWeek: dayOfWeek, InteractiveFlag: 1}).Run()
	if err != nil {
		log.Fatal(err)
	}

	err = table.Get("UserID", event.Source.UserID).Range("DayOfWeek", dynamo.Equal, dayOfWeek).One(&user)
	if err != nil {
		log.Fatal(err)
	}

	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s曜日は何ごみを捨てる日にする?\n\nメッセージで教えてね!", dayOfWeek))).Do(); err != nil {
		log.Print(err)
	}

}

func sendTimeConfig(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, user model.UserConfig) {
	time := linebot.NewButtonsTemplate(
		"",
		"通知時間を選択してね!",
		"00:00 ~ 23:59",
		linebot.NewDatetimePickerAction("Time", "action=createTime", "time", "", "23:59", "00:00"),
	)
	if _, err := bot.ReplyMessage(event.ReplyToken, linebot.NewTextMessage(fmt.Sprintf("%s曜日に%sを通知するよ!\n\n何時に通知して欲しいか選んでね!", user.DayOfWeek, message.Text)), linebot.NewTemplateMessage("時間設定", time)).Do(); err != nil {
		log.Print(err)
	}
}

func updateDayOfWeek(bot *linebot.Client, event *linebot.Event, message *linebot.TextMessage, user model.UserConfig, table dynamo.Table) {
	err := table.Update("UserID", event.Source.UserID).Range("DayOfWeek", user.DayOfWeek).Set("Content", message.Text).Set("InteractiveFlag", 2).Value(&user)
	if err != nil {
		log.Fatal(err)
	}
	sendTimeConfig(bot, event, message, user)
}

func makeDeleteButtonTemplate(userID string, table dynamo.Table) (*linebot.ButtonsTemplate, *linebot.ButtonsTemplate) {
	userConfigs := []model.UserConfig{}

	err := table.Get("UserID", userID).All(&userConfigs)
	if err != nil {
		log.Fatal(err)
		return nil, nil
	}

	pas := []*linebot.PostbackAction{}

	for _, u := range userConfigs {
		pas = append(pas, linebot.NewPostbackAction(u.DayOfWeek, fmt.Sprintf("action=deleteUserConfig&dayOfWeek=%s", u.DayOfWeek), "", ""))
	}

	var actions = make([]linebot.TemplateAction, 0)
	for _, pa := range pas {
		actions = append(actions, pa)
	}

	var bt *linebot.ButtonsTemplate
	var bt2 *linebot.ButtonsTemplate

	if len(actions) > 0 {
		// actionは4つまで
		i := 0
		if len(actions) > 4 {
			i = 4
		} else {
			i = len(actions)
		}

		bt = linebot.NewButtonsTemplate(
			"",
			"曜日を選択してね!",
			"曜日選択",
			actions[:i]...,
		)
	} else {
		bt = nil
	}

	if len(actions) >= 5 {
		bt2 = linebot.NewButtonsTemplate(
			"",
			"曜日を選択してね!",
			"曜日選択",
			actions[4:]...,
		)
	} else {
		bt2 = nil
	}
	return bt, bt2

}

func makeButtonTemplate() (*linebot.ButtonsTemplate, *linebot.ButtonsTemplate) {
	pas := []*linebot.PostbackAction{}
	for _, wday := range utils.Wdays {
		pas = append(pas, linebot.NewPostbackAction(wday, fmt.Sprintf("action=createUserConfig&dayOfWeek=%s", wday), "", ""))
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

func main() {
	lambda.Start(handler)
}
