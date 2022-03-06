package bot

import (
	"log"
	"os"

	"github.com/line/line-bot-sdk-go/linebot"
)

func NewLineBotClient() (*linebot.Client, error) {
	bot, err := linebot.New(os.Getenv("LINEBOT_SECRET_TOKEN"), os.Getenv("LINEBOT_CHANNEL_ACCESS_TOKEN"))

	if err != nil {
		log.Fatal(err)
	}

	return bot, nil

}
