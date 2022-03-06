package main

import (
	"fmt"
	"log"
	"time"

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
	table, err := db.ConnectTable("UserConfig")
	if err != nil {
		log.Fatal(err)
	}

	bot, err := bot.NewLineBotClient()

	if err != nil {
		log.Fatal(err)
	}

	jst, err := time.LoadLocation("Asia/Tokyo")
	if err != nil {
		log.Print(err)
	}
	t := time.Now().In(jst)

	users := []model.UserConfig{}

	err = table.Get("NotificationTime", t.Format("15:04")).Range("DayOfWeek", dynamo.Equal, utils.Wdays[t.Weekday()]).Index("index-3").All(&users)

	if err != nil {
		log.Print(err)
	}

	for _, u := range users {
		_, err := bot.PushMessage(u.UserID, linebot.NewTextMessage(fmt.Sprintf("今日は%sだよ!忘れずに捨ててねー!", u.Content))).Do()
		if err != nil {
			log.Fatal(err)
		}
	}

	return events.APIGatewayProxyResponse{
		StatusCode: 200,
	}, nil
}

func main() {
	lambda.Start(handler)
}
