package main

import (
	"fmt"
	"strconv"
	"time"

	"github.com/charmbracelet/log"
	tele "gopkg.in/telebot.v3"
)

type User struct {
	Id string `json:"id"`
	// Cooldown time left in minutes
	Cooldown int `json:"cooldown"`
	// Count of valid inline messages
	Count int `json:"count"`
}
type Stat struct {
	InlineCount int
	ChatCount   int
	BlockCount  int
}
type GroupSetup struct {
	CooldownMinutes int
	BurnoutLimit    int
}

type BotSetup struct {
	GroupSetup
	User
	Warned bool
}

var gDefaultSetup = GroupSetup{
	CooldownMinutes: 240,
	BurnoutLimit:    4,
}

type GroupStat struct {
	Id          string     `json:"gid"`
	InlineCount int        `json:"inlinecount"`
	ChatCount   int        `json:"chatcount"`
	BlockCount  int        `json:"blockcount"`
	Setup       GroupSetup `json:"setup"`
	Users       []User     `json:"users"`
	BotsSetup   []BotSetup `json:"botsetup"`
}

var inlineStats []GroupStat

func newGroup(gid string) GroupStat {
	return GroupStat{
		Id:          gid,
		InlineCount: 0,
		ChatCount:   0,
		BlockCount:  0,
		Setup:       gDefaultSetup,
		Users:       make([]User, 0),
	}
}
func findGroup(gid string) int {
	for k, v := range inlineStats {
		if v.Id == gid {
			return k
		}
	}
	inlineStats = append(inlineStats, newGroup(gid))
	return len(inlineStats) - 1
}

func (g *GroupStat) NewUser(id string) {
	g.Users = append(g.Users, User{Id: id, Count: 0, Cooldown: g.Setup.CooldownMinutes})
	log.Debug(fmt.Sprintf("NewUser: %s, %d", id, g.Setup.CooldownMinutes))
}

func (g *GroupStat) FindBotSetup(name string) *BotSetup {
	for i, v := range g.BotsSetup {
		if v.Id == name {
			return &g.BotsSetup[i]
		}
	}
	return nil
}
func (g *GroupStat) NewBotSetup(name string, cooldown int, burnout int) int {
	g.BotsSetup = append(g.BotsSetup, BotSetup{User: User{Id: name, Count: 0, Cooldown: 0}, GroupSetup: GroupSetup{CooldownMinutes: cooldown, BurnoutLimit: burnout}})
	return len(g.BotsSetup) - 1
}
func (g *GroupStat) RemoveBotSetup(name string) {
	for i, v := range g.BotsSetup {
		if v.Id == name {
			g.BotsSetup = append(g.BotsSetup[:i], g.BotsSetup[i+1:]...)
			return
		}
	}
}
func (g *GroupStat) FindUser(id string) int {
	for k, v := range g.Users {
		if v.Id == id {
			return k
		}
	}
	g.NewUser(id)
	return len(g.Users) - 1
}

func (g *GroupStat) StatOnMsgType(t string) {
	switch t {
	case "inline":
		g.InlineCount++
	case "chat":
		g.ChatCount++
	case "block":
		g.BlockCount++
	}
}

func (g *GroupStat) Heatsink() {
	g.Users = make([]User, 0)
	for i := range g.BotsSetup {
		g.BotsSetup[i].Count = 0
		g.BotsSetup[i].Warned = false
	}
}
func (g *GroupStat) StatReset() {
	g.InlineCount = 0
	g.ChatCount = 0
	g.BlockCount = 0
}
func (g *GroupStat) StatOnMsg(c tele.Context) error {
	uid := strconv.FormatInt(c.Sender().ID, 10)
	uk := g.FindUser(uid)
	if c.Message().Unixtime < time.Now().Unix()-60 {
		// ignore old messages
		return nil
	}
	if c.Message().Via != nil {
		var result string
		var botsetup *BotSetup
		if g.Users[uk].Count >= g.Setup.BurnoutLimit {
			result = "[BURNED](USER)"
			g.StatOnMsgType("block")
			c.Delete()
			name := fmt.Sprintf("[%s](tg://user?id=%d)", escape(fullName(c.Sender())), c.Sender().ID)
			warning := escape(fmt.Sprintf("your inline message burned out! It may take significant time for resetting. %d minutes left.", g.Users[uk].Cooldown))
			sendSelfDestroyMsg(c.Recipient(), name+", "+warning, gWarningTimeout)
		} else {
			botsetup = g.FindBotSetup(c.Message().Via.Username)
			if botsetup != nil && botsetup.Count >= botsetup.BurnoutLimit {
				result = "[BURNED](BOT)"
				g.StatOnMsgType("block")
				c.Delete()
				name := botsetup.Id
				warning := " burned out! It may take significant time for resetting."
				if !botsetup.Warned {
					warning += fmt.Sprintf(" Until %s.", time.Now().Add(time.Minute*time.Duration(botsetup.Cooldown)).Format("15:04"))
					sendMsg(c.Recipient(), escape("Bot @"+name+warning))
					botsetup.Warned = true
				} else {
					warning += fmt.Sprintf(" %d minutes left.", botsetup.Cooldown)
					sendSelfDestroyMsg(c.Recipient(), escape("Bot @"+name+warning), gWarningTimeout)
				}
			} else {
				g.Users[uk].Count++
				if g.Users[uk].Count == 1 {
					g.Users[uk].Cooldown = g.Setup.CooldownMinutes
				}
				result = "[ALLOWED]"
				if botsetup != nil {
					botsetup.Count++
					if botsetup.Count == 1 {
						botsetup.Cooldown = botsetup.CooldownMinutes
					}
				}
				g.StatOnMsgType("inline")
			}
		}
		details := fmt.Sprintf("Chat %s\nUser @%s:%d/%d", g.Id, g.Users[uk].Id, g.Users[uk].Count, g.Setup.BurnoutLimit)
		if botsetup != nil {
			details += fmt.Sprintf("\nBot @%s:%d/%d", botsetup.Id, botsetup.Count, botsetup.BurnoutLimit)
		}
		msgLog.Info(result, "detail", details)
	} else {
		g.StatOnMsgType("chat")
		g.OnChatMsg(c)
	}
	return nil
}
