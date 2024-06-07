package main

import (
	"fmt"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v3"
)

func inlineMessageHandler(c tele.Context) error {
	group := findGroupByContext(c)
	user := group.GetUser(strconv.FormatInt(c.Sender().ID, 10))
	botSetup := group.GetBotSetup(c.Message().Via.Username)
	var resultLog string

	if group.IsUserBurned(user) {
		resultLog = "[BURNED](USER)"
		group.MsgCount("block")
		c.Delete()
		name := fmt.Sprintf("[%s](tg://user?id=%d)", escape(fullName(c.Sender())), c.Sender().ID)
		warning := escape(fmt.Sprintf("your inline message burned out! It may take significant time for resetting. %d minutes left.", user.Cooldown))
		sendSelfDestroyMsg(c.Recipient(), name+", "+warning, gWarningTimeout)
	} else {
		if group.IsBotBurned(c.Message().Via.Username) {
			resultLog = "[BURNED](BOT)"
			group.MsgCount("block")
			c.Delete()
			warning := "Bot @" + botSetup.Id + " burned out! It may take significant time for resetting."
			if !group.BotWarn(c.Message().Via.Username) {
				warning += fmt.Sprintf(" Until %s.", time.Now().Add(time.Minute*time.Duration(botSetup.Cooldown)).Format("15:04"))
				sendMsg(c.Recipient(), escape(warning))
			} else {
				warning += fmt.Sprintf(" %d minutes left.", botSetup.Cooldown)
				sendSelfDestroyMsg(c.Recipient(), escape(warning), gWarningTimeout)
			}
		} else {
			resultLog = "[ALLOWED]"
			group.UserCountAdd(user)
			if botSetup != nil {
				botSetup.CountAdd()
			}
			group.MsgCount("inline")
		}
	}

	details := fmt.Sprintf("Chat %s\nUser @%s:%d/%d", group.Id, user.Id, user.Count, group.Setup.BurnoutLimit)
	if botSetup != nil {
		details += fmt.Sprintf("\nBot @%s:%d/%d", botSetup.Id, botSetup.Count, botSetup.BurnoutLimit)
	}
	msgLog.Info(resultLog, "detail", details)
	return nil
}

func chatMessageHandler(c tele.Context) error {
	group := findGroupByContext(c)
	group.MsgCount("chat")
	for _, fn := range cmdWithParamsHandlers {
		if fn(c) {
			return nil
		}
	}
	return nil
}
