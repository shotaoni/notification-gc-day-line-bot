package model

import "github.com/line/line-bot-sdk-go/linebot"

type Webhook struct {
	Events []*linebot.Event `json:"events"`
}
