package model

type UserConfig struct {
	UserID           string `dynamo:"UserID,hash"`
	DayOfWeek        string `dynamo:"DayOfWeek,range" index:"index-3,range"`
	Content          string `dynamo:"Content"`
	NotificationTime string `dynamo:"NotificationTime" index:"index-3,hash"`
	InteractiveFlag  int    `dynamo:"InteractiveFlag" localIndex:"index-2,range"`
}
