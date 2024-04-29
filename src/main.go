package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	kuma "github.com/Nigh/kuma-push"
	scribble "github.com/nanobox-io/golang-scribble"
	tele "gopkg.in/telebot.v3"
)

type testenv struct {
	Token   string `json:"token"`
	AdminID string `json:"adminid"`
	KumaURL string `json:"kumaurl"`
}

var testEnv testenv

var (
	gTimeFormat      string = "2006-01-02 15:04"
	gRootID          int64
	gKumaPushURL     string
	gToken           string
	gCooldownMinutes int           = 240
	gBurnoutLimit    int           = 4
	gWarningTimeout  time.Duration = 15 * time.Second
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
	reserve int
}

type GroupStat struct {
	Id          string     `json:"gid"`
	InlineCount int        `json:"inlinecount"`
	ChatCount   int        `json:"chatcount"`
	BlockCount  int        `json:"blockcount"`
	Setup       GroupSetup `json:"setup"`
	Users       []User     `json:"users"`
}

func (g *GroupStat) NewUser(id string) {
	g.Users = append(g.Users, User{Id: id, Count: 0, Cooldown: gCooldownMinutes})
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

func (g *GroupStat) StatReset() {
	g.InlineCount = 0
	g.ChatCount = 0
	g.BlockCount = 0
}
func (g *GroupStat) StatOnMsg(c tele.Context) error {
	uid := strconv.FormatInt(c.Sender().ID, 10)
	uk := g.FindUser(uid)
	if c.Message().Via != nil {
		fmt.Printf("[%s:%s][MSG] Type:inline From:@%s\n", g.Id, uid, c.Message().Via.Username)
		g.Users[uk].Count++
		if g.Users[uk].Count == 1 {
			g.Users[uk].Cooldown = gCooldownMinutes
		}
		if g.Users[uk].Count > gBurnoutLimit {
			fmt.Println("\t\t--BLOCKED--")
			g.StatOnMsgType("block")
			c.Delete()
			go sendSelfDestroyMsg(c.Recipient(), fmt.Sprintf("[%s](tg://user?id=%d)", escape(fullName(c.Sender())), c.Sender().ID)+
				escape(fmt.Sprintf(", your inline message burned out! It may take significant time for resetting. %d minutes left.", g.Users[uk].Cooldown)), gWarningTimeout)
		} else {
			fmt.Println("\t\t--ALLOWED--")
			g.StatOnMsgType("inline")
		}
		// msgPrint(c)
	} else {
		g.StatOnMsgType("chat")
	}
	return nil
}

var bot *tele.Bot
var db *scribble.Driver
var inlineStats []GroupStat

func newGroup(gid string) GroupStat {
	return GroupStat{
		Id:          gid,
		InlineCount: 0,
		ChatCount:   0,
		BlockCount:  0,
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

func sendSelfDestroyMsg(to tele.Recipient, what interface{}, timeout time.Duration) error {
	msg, err := bot.Send(to, what, tele.ModeMarkdownV2)
	if err == nil {
		go func() {
			timer := time.NewTimer(timeout)
			<-timer.C
			bot.Delete(msg)
		}()
	}
	return err
}

func msgPrint(c tele.Context) {
	gid := strconv.FormatInt(c.Chat().ID, 10)
	uid := strconv.FormatInt(c.Sender().ID, 10)
	gk := findGroup(gid)
	for _, u := range inlineStats[gk].Users {
		if u.Id == uid {
			fmt.Printf("GROUP %s USER %s - n:%d cd:%d\n", gid, uid, u.Count, u.Cooldown)
			return
		}
	}
	fmt.Println("[ERROR] USER NOT FOUND")
}

func msgHandler(c tele.Context) error {
	gid := strconv.FormatInt(c.Chat().ID, 10)
	gkey := findGroup(gid)
	return inlineStats[gkey].StatOnMsg(c)
}

func inlineCooldownRoutine() {
	interval := time.NewTicker(1 * time.Minute)
	defer interval.Stop()
	for range interval.C {
		for gk, group := range inlineStats {
			for uk, user := range group.Users {
				if user.Cooldown > 0 {
					user.Cooldown--
					if user.Cooldown <= 0 {
						user.Count = 0
					}
					inlineStats[gk].Users[uk] = user
				}
			}
		}
		if time.Now().Hour() == 23 && time.Now().Minute() == 55 {
			go func() {
				for k, group := range inlineStats {
					if group.InlineCount > 0 {
						gid, _ := strconv.ParseInt(group.Id, 10, 64)
						sendSelfDestroyMsg(tele.ChatID(gid), escape(fmt.Sprintf("In the past 24 hours, there are total %d msgs handled by this bot.\nIn the handled msgs, there are:\n%d inline msgs sent\n%d inline msgs been blocked", group.InlineCount+group.ChatCount, group.InlineCount, group.BlockCount)), 60*time.Second)
					}
					inlineStats[k].StatReset()
					time.Sleep(time.Millisecond * 200)
				}
			}()
		}
	}
}

func init() {
	db, _ = scribble.New("../db", nil)
	db.Read("test", "env", &testEnv)
	inlineStats = make([]GroupStat, 0)
	db.Read("data", "inline", &inlineStats)
	go inlineCooldownRoutine()
	if testEnv.Token != "" {
		gToken = testEnv.Token
		gRootID, _ = strconv.ParseInt(testEnv.AdminID, 10, 64)
		gKumaPushURL = testEnv.KumaURL
	} else {
		gToken = os.Getenv("BOT_TOKEN")
		gRootID, _ = strconv.ParseInt(os.Getenv("BOT_ADMIN_ID"), 10, 64)
		gKumaPushURL = os.Getenv("KUMA_PUSH_URL")
	}
	fmt.Printf("gToken:%s\ngRootID:%d\ngKumaPushURL:%s\n", gToken, gRootID, gKumaPushURL)
	k := kuma.New(gKumaPushURL)
	k.Start()
}

func escape(s string) string {
	var escapeList string = `_*[]()~` + "`" + `>#+-=|{}.!`
	prefix := ""
	result := ""
	for _, c := range s {
		char := string(c)
		if strings.ContainsRune(escapeList, rune(c)) && prefix != `\` {
			result += `\` + char
		} else {
			result += char
		}
		prefix = char
	}
	return result
}
func fullName(u *tele.User) string {
	if u.FirstName == "" || u.LastName == "" {
		return u.FirstName + u.LastName
	}
	return u.FirstName + " " + u.LastName
}

func main() {
	pref := tele.Settings{
		Token:  gToken,
		Poller: &tele.LongPoller{Timeout: 5 * time.Second},
	}
	var err error
	bot, err = tele.NewBot(pref)
	if err != nil {
		log.Fatal(err)
		return
	}

	for _, v := range []string{tele.OnText, tele.OnPhoto, tele.OnAnimation, tele.OnDocument, tele.OnSticker, tele.OnVideo, tele.OnVoice} {
		bot.Handle(v, msgHandler)
	}
	bot.Handle(tele.OnAddedToGroup, func(c tele.Context) error {
		c.Send("My pleasure to join the group! Inline messages will be limited by me.")
		return nil
	})

	go bot.Start()

	fmt.Println("bot online!")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	bot.Stop()
	fmt.Println("before shutdown, backup data")
	db.Write("data", "inline", inlineStats)
	<-time.After(time.Second * 1)
	fmt.Println("bot offline!")
}
