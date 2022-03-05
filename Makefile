include .env
.PHONY: build clean deploy
build:
	env GOOS=linux go build -ldflags="-s -w" -o bin/main main.go
clean:
	rm -rf ./bin
deploy: clean build
	@sls deploy --verbose \
	--linebotSecretToken ${LINEBOT_SECRET_TOKEN} \
	--linebotChannelAccessToken ${LINEBOT_CHANNEL_ACCESS_TOKEN}