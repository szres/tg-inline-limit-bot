package main

import (
	"fmt"
	"regexp"
	"strconv"
	"time"

	tele "gopkg.in/telebot.v3"
)

const (
	cmdHelp     string = "/help"
	cmdHeatsink string = "/heatsink"
	cmdSetup    string = `^/setup(?: (\d+),\s?(\d+))?$`
	cmdBotLimit string = `^/botlimit(?: (\d+),\s?(\d+))?$`
)

var cmdWithParamsHandlers = []func(c tele.Context) bool{
	onSetup,
	onBotLimit,
}

// type cmdType int

// const (
// 	Normal cmdType = iota
// 	Regexp
// )

// type CMDs struct {
// 	Type    cmdType
// 	Content string
// 	Handler func(c tele.Context) bool
// }

func onSetupHelp(c tele.Context) error {
	reply := "Usage: `/setup <X>,<Y>`"
	reply += "\nExample: `/setup 4,240`"
	reply += fmt.Sprintf("\n\nThe valid X value is from %d to %d, and the valid Y value is from %d to %d", gBurnoutLimitMin, gBurnoutLimitMax, gCooldownMinutesMin, gCooldownMinutesMax)
	return replySelfDestroyMsg(c.Message(), reply, 60*time.Second)
}

func onBotLimitHelp(c tele.Context) error {
	reply := "Usage: REPLY to the inline message `/botlimit <X>,<Y>`"
	reply += escape("\nExample: relpy to message {User via @InlineBot} with") + " `/botlimit 4,240`"
	reply += fmt.Sprintf("\n\nThe valid X value is from %d to %d, and the valid Y value is from %d to %d", gBotBurnoutLimitMin, gBotBurnoutLimitMax, gBotCooldownMinutesMin, gBotCooldownMinutesMax)
	reply += escape("\nSet the X and Y value to 0 would remove the limit of the bot.")
	return replySelfDestroyMsg(c.Message(), reply, 60*time.Second)
}

func onHelp(c tele.Context) error {
	group := findGroupByContext(c)

	help := "This is inline message limiter."
	help += "\nThe inline messages sent exceeding the specified number within the specified time will be deleted."
	help += "\n\nCommand (admin only):"
	help += "\n/help - display help message"
	help += "\n/heatsink - immediately cooldown for everything"
	help = escape(help)
	help += "\n`/setup <X>,<Y>`" + escape(" - setting user burnout to be triggered by sending X inline messages in Y minutes")
	help += "\n`/botlimit <X>,<Y>`" + escape(" - reply to the inline message to set the limit of the sender bot")

	help += escape("\n\nCurrent setup:\nUser allowed " + strconv.Itoa(group.Setup.BurnoutLimit) + " inline messages in " + strconv.Itoa(group.Setup.CooldownMinutes) + " minutes.")
	if len(group.BotsSetup) > 0 {
		for _, v := range group.BotsSetup {
			help += escape("\nBot @" + v.Id + " allowed " + strconv.Itoa(v.BurnoutLimit) + " messages in " + strconv.Itoa(v.CooldownMinutes) + " minutes.")
		}
	}
	return sendSelfDestroyMsg(c.Recipient(), help, 300*time.Second)
}

func onHeatsink(c tele.Context) error {
	findGroupByContext(c).Heatsink()
	_, err := bot.Reply(c.Message(), escape("Everyone's burnout count has been reset."), tele.ModeMarkdownV2)
	errLog.Error("Reply to message", "err", err)
	return err
}

func onSetup(c tele.Context) bool {
	group := findGroupByContext(c)
	matchs := regexp.MustCompile(cmdSetup).FindStringSubmatch(c.Text())
	if len(matchs) > 0 {
		if len(matchs[1]) == 0 || len(matchs[2]) == 0 {
			onSetupHelp(c)
			return true
		}

		burnout, err1 := strconv.Atoi(matchs[1])
		cooldown, err2 := strconv.Atoi(matchs[2])
		if err1 != nil || err2 != nil ||
			burnout < gBurnoutLimitMin || burnout > gBurnoutLimitMax ||
			cooldown < gCooldownMinutesMin || cooldown > gCooldownMinutesMax {
			reply := escape(fmt.Sprintf("Invalid value.\n\nThe valid X value is from %d to %d, and the valid Y value is from %d to %d", gBurnoutLimitMin, gBurnoutLimitMax, gCooldownMinutesMin, gCooldownMinutesMax))
			bot.Reply(c.Message(), reply, tele.ModeMarkdownV2)
			return true
		}
		group.Setup.BurnoutLimit = burnout
		group.Setup.CooldownMinutes = cooldown
		bot.Reply(c.Message(), fmt.Sprintf("Setup update successful\nNow, user burnout is set to be triggered by sending more than `%d` inline messages in `%d` minutes", burnout, cooldown), tele.ModeMarkdownV2)
		return true
	}
	return false
}

func onBotLimit(c tele.Context) bool {
	group := findGroupByContext(c)
	matchs := regexp.MustCompile(cmdBotLimit).FindStringSubmatch(c.Text())
	if len(matchs) > 0 {
		if len(matchs[1]) == 0 || len(matchs[2]) == 0 || c.Message().ReplyTo == nil || c.Message().ReplyTo.Via == nil {
			onBotLimitHelp(c)
			return true
		} else {
			botName := c.Message().ReplyTo.Via.Username
			burnout, err1 := strconv.Atoi(matchs[1])
			cooldown, err2 := strconv.Atoi(matchs[2])
			if err1 != nil || err2 != nil || ((burnout != 0 && cooldown != 0) &&
				(burnout < gBotBurnoutLimitMin || burnout > gBotBurnoutLimitMax ||
					cooldown < gBotCooldownMinutesMin || cooldown > gBotCooldownMinutesMax)) {
				reply := escape(fmt.Sprintf("Invalid value.\n\nThe valid X value is from %d to %d, and the valid Y value is from %d to %d", gBotBurnoutLimitMin, gBotBurnoutLimitMax, gBotCooldownMinutesMin, gBotCooldownMinutesMax))
				bot.Reply(c.Message(), reply, tele.ModeMarkdownV2)
				return true
			}
			if burnout == 0 && cooldown == 0 {
				group.RemoveBotSetup(botName)
				bot.Reply(c.Message(), "Remove bot limit successful", tele.ModeMarkdownV2)
			} else {
				bs := group.GetBotSetup(botName)
				if bs == nil {
					group.NewBotSetup(botName, cooldown, burnout)
				} else {
					bs.CooldownMinutes = cooldown
					bs.BurnoutLimit = burnout
				}
				bot.Reply(c.Message(), escape(fmt.Sprintf("Setup successful\nBot @%s's limit is set to %d messages in %d minutes", botName, burnout, cooldown)), tele.ModeMarkdownV2)
			}
			return true
		}
	}
	return false
}
