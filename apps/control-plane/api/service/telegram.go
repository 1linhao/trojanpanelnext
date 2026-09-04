package service

import (
	"errors"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
	"github.com/sirupsen/logrus"
	"trojan-panel/model/constant"
)

var bot = new(tgbotapi.BotAPI)

func NewTelegramBotApi() (*tgbotapi.BotAPI, error) {
	var err error
	// 从数据库中查询api token
	apiToken := ""
	bot, err = tgbotapi.NewBotAPI(apiToken)
	if err != nil {
		logrus.Errorf("new bot api err: %v", err)
		return nil, errors.New(constant.TelegramBotApiError)
	}
	logrus.Infof("Authorized on account %s", bot.Self.UserName)
	return bot, nil
}

func GetUpdatesChan() tgbotapi.UpdatesChannel {
	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60
	return bot.GetUpdatesChan(u)
}
