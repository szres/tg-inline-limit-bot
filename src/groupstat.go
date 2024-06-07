package main

import (
	"fmt"

	"github.com/charmbracelet/log"
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

var groups []GroupStat

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
func findGroupByGid(gid string) int {
	for k, v := range groups {
		if v.Id == gid {
			return k
		}
	}
	groups = append(groups, newGroup(gid))
	return len(groups) - 1
}

func (g *GroupStat) NewUser(id string) {
	g.Users = append(g.Users, User{Id: id, Count: 0, Cooldown: g.Setup.CooldownMinutes})
	log.Debug(fmt.Sprintf("NewUser: %s, %d", id, g.Setup.CooldownMinutes))
}
func (g *GroupStat) GetUser(id string) *User {
	return &g.Users[g.FindUser(id)]
}
func (g *GroupStat) GetBotSetup(name string) *BotSetup {
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

func (g *GroupStat) MsgCount(t string) {
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

func (g *GroupStat) UserCountAdd(u *User) {
	u.Count++
	if u.Count == 1 {
		u.Cooldown = g.Setup.CooldownMinutes
	}
}
func (g *GroupStat) IsUserBurned(u *User) bool {
	return u.Count >= g.Setup.BurnoutLimit
}

func (b *BotSetup) CountAdd() {
	b.Count++
	if b.Count == 1 {
		b.Cooldown = b.CooldownMinutes
	}
}

func (g *GroupStat) IsBotBurned(name string) bool {
	bk := g.GetBotSetup(name)
	if bk == nil {
		return false
	}
	return bk.Count >= bk.BurnoutLimit
}

func (g *GroupStat) BotWarn(name string) bool {
	bk := g.GetBotSetup(name)
	if bk == nil {
		return true
	}
	if bk.Warned {
		return true
	}
	bk.Warned = true
	return false
}
