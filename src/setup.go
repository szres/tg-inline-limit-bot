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
	cmdSetup    string = `^/setup (\d+),\s?(\d+)$`
	cmdBotLimit string = `^/botlimit (\d+),\s?(\d+)$`
)

var cmdWithParamsHandlers = []func(g *GroupStat, c tele.Context) bool{
	onSetup,
	onBotLimit,
}

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
	gid := strconv.FormatInt(c.Chat().ID, 10)
	gkey := findGroup(gid)

	help := "This is inline message limiter."
	help += "\nThe inline messages sent exceeding the specified number within the specified time will be deleted."
	help += "\n\nCommand (admin only):"
	help += "\n/help - display help message"
	help += "\n/heatsink - immediately cooldown for everything"
	help = escape(help)
	help += "\n`/setup <X>,<Y>`" + escape(" - setting user burnout to be triggered by sending X inline messages in Y minutes")
	help += "\n`/botlimit <X>,<Y>`" + escape(" - reply to the inline message to set the limit of the sender bot")

	help += escape("\n\nCurrent setup:\nUser allowed " + strconv.Itoa(inlineStats[gkey].Setup.BurnoutLimit) + " inline messages in " + strconv.Itoa(inlineStats[gkey].Setup.CooldownMinutes) + " minutes.")
	if len(inlineStats[gkey].BotsSetup) > 0 {
		for _, v := range inlineStats[gkey].BotsSetup {
			help += escape("\nBot @" + v.Id + " allowed " + strconv.Itoa(v.BurnoutLimit) + " messages in " + strconv.Itoa(v.CooldownMinutes) + " minutes.")
		}
	}
	return sendSelfDestroyMsg(c.Recipient(), help, 300*time.Second)
}

func onSetup(g *GroupStat, c tele.Context) bool {
	matchs := regexp.MustCompile(`^/setup(?: (\d+),\s?(\d+))?$`).FindStringSubmatch(c.Text())
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
		g.Setup.BurnoutLimit = burnout
		g.Setup.CooldownMinutes = cooldown
		bot.Reply(c.Message(), fmt.Sprintf("Setup update successful\nNow, user burnout is set to be triggered by sending more than `%d` inline messages in `%d` minutes", burnout, cooldown), tele.ModeMarkdownV2)
		return true
	}
	return false
}
func (g *GroupStat) OnChatMsg(c tele.Context) error {
	if hasPrivilege(c) {
		for _, v := range cmdWithParamsHandlers {
			if v(g, c) {
				return nil
			}
		}
	}
	return nil
}
