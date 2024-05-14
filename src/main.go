package main

import (
	"fmt"
	"log"
	"math"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	_ "time/tzdata"

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

type BotStat struct {
	LastSummarySentTime time.Time
}

var botStat BotStat

var (
	gTimeFormat         string = "2006-01-02 15:04:05"
	gRootID             int64
	gKumaPushURL        string
	gToken              string
	gCooldownMinutesMin int           = 5
	gCooldownMinutesMax int           = 1440
	gBurnoutLimitMin    int           = 0
	gBurnoutLimitMax    int           = 13
	gWarningTimeout     time.Duration = 15 * time.Second
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
}

func (g *GroupStat) NewUser(id string) {
	g.Users = append(g.Users, User{Id: id, Count: 0, Cooldown: g.Setup.CooldownMinutes})
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
		fmt.Printf("[%s][%s:%s][MSG] Type:inline From:@%s\n", time.Now().Format(gTimeFormat), g.Id, uid, c.Message().Via.Username)
		g.Users[uk].Count++
		if g.Users[uk].Count == 1 {
			g.Users[uk].Cooldown = g.Setup.CooldownMinutes
		}
		if g.Users[uk].Count > g.Setup.BurnoutLimit {
			fmt.Println("\t\t--BLOCKED--")
			g.StatOnMsgType("block")
			c.Delete()
			name := fmt.Sprintf("[%s](tg://user?id=%d)", escape(fullName(c.Sender())), c.Sender().ID)
			warning := escape(fmt.Sprintf("your inline message burned out! It may take significant time for resetting. %d minutes left.", g.Users[uk].Cooldown))
			sendSelfDestroyMsg(c.Recipient(), name+", "+warning, gWarningTimeout)
		} else {
			fmt.Println("\t\t--ALLOWED--")
			g.StatOnMsgType("inline")
		}
	} else {
		g.StatOnMsgType("chat")
		g.OnChatMsg(c)
	}
	return nil
}

type SetupPending struct {
	Gid     string
	Uid     string
	Timeout int
}

var setupPendings []SetupPending

func (g *GroupStat) OnChatMsg(c tele.Context) error {
	newSetupPendings := make([]SetupPending, 0)
	for _, v := range setupPendings {
		if v.Gid == g.Id && v.Uid == strconv.FormatInt(c.Sender().ID, 10) {
			matchs := regexp.MustCompile(`^(\d+),\s?(\d+)$`).FindStringSubmatch(c.Text())
			if len(matchs) > 0 {
				burnout, err1 := strconv.Atoi(matchs[1])
				cooldown, err2 := strconv.Atoi(matchs[2])
				if err1 != nil || err2 != nil ||
					burnout < gBurnoutLimitMin || burnout > gBurnoutLimitMax ||
					cooldown < gCooldownMinutesMin || cooldown > gCooldownMinutesMax {
					bot.Reply(c.Message(), escape("Invalid value."), tele.ModeMarkdownV2)
					continue
				}
				g.Setup.BurnoutLimit = burnout
				g.Setup.CooldownMinutes = cooldown
				bot.Reply(c.Message(), fmt.Sprintf("Setup update successful\nNow, burnout is set to be triggered by sending more than `%d` inline messages in `%d` minutes", burnout, cooldown), tele.ModeMarkdownV2)
			}
		} else {
			newSetupPendings = append(newSetupPendings, v)
		}
	}
	setupPendings = newSetupPendings
	return nil
}

type MsgWithTimeout struct {
	Msg  tele.StoredMessage `json:"msg"`
	Time time.Time          `json:"time"`
}

var msgs2Delete []MsgWithTimeout

func deleteAfter(msg tele.Editable, timeout time.Duration) {
	m, c := msg.MessageSig()
	msgStore := tele.StoredMessage{MessageID: m, ChatID: c}
	msgs2Delete = append(msgs2Delete, MsgWithTimeout{Msg: msgStore, Time: time.Now().Add(timeout)})
	db.Write("data", "msg2delete", &msgs2Delete)
}

func msgDeleteTimer() {
	isMsg2Delete := false
	for _, msg := range msgs2Delete {
		if msg.Time.Before(time.Now()) {
			isMsg2Delete = true
			break
		}
	}
	if isMsg2Delete {
		msgs2DeleteNew := make([]MsgWithTimeout, 0)
		for _, m := range msgs2Delete {
			if m.Time.Before(time.Now()) {
				bot.Delete(m.Msg)
			} else {
				msgs2DeleteNew = append(msgs2DeleteNew, m)
			}
		}
		msgs2Delete = msgs2DeleteNew
		db.Write("data", "msg2delete", &msgs2Delete)
	}
}
func setupPendingTimer() {
	setupPendingsNew := make([]SetupPending, 0)
	for _, v := range setupPendings {
		v.Timeout--
		if v.Timeout > 0 {
			setupPendingsNew = append(setupPendingsNew, v)
		}
	}
	setupPendings = setupPendingsNew
}

func oneSecondTimer() {
	interval := time.NewTicker(1 * time.Second)
	defer interval.Stop()
	for range interval.C {
		msgDeleteTimer()
		setupPendingTimer()
	}
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

func replySelfDestroyMsg(to *tele.Message, what interface{}, timeout time.Duration) error {
	msg, err := bot.Reply(to, what, tele.ModeMarkdownV2)
	if err == nil {
		deleteAfter(to, timeout)
		deleteAfter(msg, timeout)
	}
	return err
}
func sendSelfDestroyMsg(to tele.Recipient, what interface{}, timeout time.Duration) error {
	msg, err := bot.Send(to, what, tele.ModeMarkdownV2)
	if err == nil {
		deleteAfter(msg, timeout)
	}
	return err
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
					// in case of setup changed
					if user.Cooldown > group.Setup.CooldownMinutes {
						user.Cooldown = group.Setup.CooldownMinutes
					}
					user.Cooldown--
					if user.Cooldown <= 0 {
						user.Count = 0
					}
					inlineStats[gk].Users[uk] = user
				}
			}
		}

		if time.Now().After(botStat.LastSummarySentTime.Add(24*time.Hour)) ||
			(time.Now().After(botStat.LastSummarySentTime.Add(12*time.Hour)) && time.Now().Hour() >= 23 && time.Now().Minute() >= 30) {
			go func() {
				hours := int(math.Ceil(time.Since(botStat.LastSummarySentTime).Hours()))
				fmt.Printf("--- %dH SUMMARY ----------------\n", hours)
				for k, group := range inlineStats {
					fmt.Printf("[%s] total:%d inline:%d block:%d\n", group.Id, group.ChatCount+group.InlineCount, group.InlineCount, group.BlockCount)
					if group.InlineCount > 0 {
						gid, _ := strconv.ParseInt(group.Id, 10, 64)
						sendSelfDestroyMsg(tele.ChatID(gid), fmt.Sprintf("In the past `%d` hours, there are `%d` msgs handled by this bot\\.\nIn the `%d` inline msgs, there are:\n`%d` allowed\n`%d` blocked", hours, group.InlineCount+group.BlockCount+group.ChatCount, group.InlineCount+group.BlockCount, group.InlineCount, group.BlockCount), 6*time.Hour)
					}
					inlineStats[k].StatReset()
					time.Sleep(time.Millisecond * 200)
				}
				botStat.LastSummarySentTime = time.Now()
				db.Write("data", "bot", &botStat)
			}()
		}
	}
}

func init() {
	db, _ = scribble.New("../db", nil)
	db.Read("test", "env", &testEnv)
	inlineStats = make([]GroupStat, 0)
	db.Read("data", "inline", &inlineStats)
	msgs2Delete = make([]MsgWithTimeout, 0)
	db.Read("data", "msg2delete", &msgs2Delete)
	db.Read("data", "bot", &botStat)
	if botStat.LastSummarySentTime.IsZero() {
		botStat.LastSummarySentTime = time.Now().Add(-12 * time.Hour)
		db.Write("data", "bot", &botStat)
	}
	for k, v := range inlineStats {
		if v.Setup.BurnoutLimit == 0 && v.Setup.CooldownMinutes == 0 {
			inlineStats[k].Setup = gDefaultSetup
		}
	}
	setupPendings = make([]SetupPending, 0)
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

func hasPrivilege(c tele.Context) bool {
	member, err := bot.ChatMemberOf(c.Chat(), c.Sender())
	if err != nil {
		return false
	}
	if member.Role != tele.Creator && member.Role != tele.Administrator {
		return false
	}
	return true
}

const (
	Help     string = "/help"
	Heatsink string = "/heatsink"
	Setup    string = "/setup"
)

func CmdHandler(fn func(c tele.Context) error) func(c tele.Context) error {
	return func(c tele.Context) error {
		if c.Chat().Type == tele.ChatPrivate {
			return c.Send("Command is only valid in a group.")
		}
		if !hasPrivilege(c) {
			return replySelfDestroyMsg(c.Message(), escape("Only admins can use this command!"), 15*time.Second)
		}
		return fn(c)
	}
}

func onHelp(c tele.Context) error {
	gid := strconv.FormatInt(c.Chat().ID, 10)
	gkey := findGroup(gid)

	help := "This is inline message limiter."
	help += "\nThe inline messages sent exceeding the specified number within the specified time will be deleted."
	help += "\n\nCommand (admin only):"
	help += "\n/help - display this message"
	help += "\n/heatsink - immediately reset all burnout count"
	help += "\n/setup - setting burnout to be triggered by sending X inline messages in Y minutes"
	help += "\n\nCurrent setup: send more than " + strconv.Itoa(inlineStats[gkey].Setup.BurnoutLimit) + " inline messages in " + strconv.Itoa(inlineStats[gkey].Setup.CooldownMinutes) + " minutes would burnout."
	return sendSelfDestroyMsg(c.Recipient(), escape(help), 300*time.Second)
}
func onHeatsink(c tele.Context) error {
	gid := strconv.FormatInt(c.Chat().ID, 10)
	gkey := findGroup(gid)
	inlineStats[gkey].Heatsink()
	_, err := bot.Reply(c.Message(), escape("Everyone's burnout count has been reset."), tele.ModeMarkdownV2)
	return err
}
func onSetup(c tele.Context) error {
	gid := strconv.FormatInt(c.Chat().ID, 10)
	uid := strconv.FormatInt(c.Sender().ID, 10)
	for k, v := range setupPendings {
		if v.Gid == gid && v.Uid == uid {
			setupPendings[k].Timeout = 0
		}
	}
	setupPendings = append(setupPendings, SetupPending{Gid: gid, Uid: uid, Timeout: 100})
	reply := "Set burnout to be triggered by sending `X` inline messages in `Y` minutes in the following format:`X,Y`"
	reply += "\nExample: `4,240`"
	reply += fmt.Sprintf("\n\nThe valid X value is from %d to %d, and the valid Y value is from %d to %d", gBurnoutLimitMin, gBurnoutLimitMax, gCooldownMinutesMin, gCooldownMinutesMax)
	return replySelfDestroyMsg(c.Message(), reply, 100*time.Second)
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
		return c.Send("My pleasure to join the group! Inline messages will be limited by me.")
	})
	bot.Handle(Help, CmdHandler(onHelp))
	bot.Handle(Heatsink, CmdHandler(onHeatsink))
	bot.Handle(Setup, CmdHandler(onSetup))
	go bot.Start()
	go oneSecondTimer()
	go inlineCooldownRoutine()

	fmt.Printf("[%s] bot online!\n", time.Now().Format(gTimeFormat))
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	bot.Stop()
	fmt.Println("before shutdown, backup data")
	db.Write("data", "inline", inlineStats)
	<-time.After(time.Second * 1)
	fmt.Println("bot offline!")
}
