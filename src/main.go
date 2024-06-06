package main

import (
	"fmt"
	"math"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	_ "time/tzdata"

	kuma "github.com/Nigh/kuma-push"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/log"
	scribble "github.com/nanobox-io/golang-scribble"
	tele "gopkg.in/telebot.v3"
)

// DOING: Code Refactoring

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
	gRootID                int64
	gKumaPushURL           string
	gToken                 string
	gCooldownMinutesMin    int           = 5
	gCooldownMinutesMax    int           = 1440
	gBurnoutLimitMin       int           = 0
	gBurnoutLimitMax       int           = 13
	gWarningTimeout        time.Duration = 15 * time.Second
	gBotCooldownMinutesMin int           = 1
	gBotCooldownMinutesMax int           = 1440
	gBotBurnoutLimitMin    int           = 1
	gBotBurnoutLimitMax    int           = 1440
)

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
				log.Debug("[DELETE MSG]", "chatId", m.Msg.ChatID, "msgId", m.Msg.MessageID)
			} else {
				msgs2DeleteNew = append(msgs2DeleteNew, m)
			}
		}
		msgs2Delete = msgs2DeleteNew
		db.Write("data", "msg2delete", &msgs2Delete)
	}
}

func oneSecondTimer() {
	interval := time.NewTicker(1 * time.Second)
	defer interval.Stop()
	for range interval.C {
		msgDeleteTimer()
	}
}

var bot *tele.Bot
var db *scribble.Driver

func replySelfDestroyMsg(to *tele.Message, what interface{}, timeout time.Duration) error {
	msg, err := bot.Reply(to, what, tele.ModeMarkdownV2)
	log.Debug("[REPLY MSG]", "to", to.ID, "what", what)
	if err == nil {
		deleteAfter(to, timeout)
		deleteAfter(msg, timeout)
	}
	return err
}
func sendSelfDestroyMsg(to tele.Recipient, what interface{}, timeout time.Duration) error {
	msg, err := bot.Send(to, what, tele.ModeMarkdownV2)
	log.Debug("[SEND MSG]", "to", to.Recipient(), "what", what)
	if err == nil && timeout > 0 {
		deleteAfter(msg, timeout)
	}
	return err
}
func sendMsg(to tele.Recipient, what interface{}) error {
	return sendSelfDestroyMsg(to, what, 0)
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
						timerLog.Info("[COOLDOWN]", "detail", fmt.Sprintf("Chat %s\nUser @%s", group.Id, user.Id))
					}
					inlineStats[gk].Users[uk] = user
				}
			}
			for bk := range group.BotsSetup {
				bs := &group.BotsSetup[bk]
				if bs.Cooldown > 0 {
					if bs.Cooldown > bs.CooldownMinutes {
						bs.Cooldown = bs.CooldownMinutes
					}
					bs.Cooldown--
					if bs.Cooldown <= 0 {
						bs.Count = 0
						bs.Warned = false
						timerLog.Info("[COOLDOWN]", "detail", fmt.Sprintf("Chat %s\nBot @%s", group.Id, bs.Id))
					}
				}
			}
		}

		if time.Now().After(botStat.LastSummarySentTime.Add(24*time.Hour)) ||
			(time.Now().After(botStat.LastSummarySentTime.Add(12*time.Hour)) && time.Now().Hour() >= 23 && time.Now().Minute() >= 30) {
			go func() {
				hours := int(math.Ceil(time.Since(botStat.LastSummarySentTime).Hours()))
				summaryLog.Infof("in %d hours", hours)
				for k, group := range inlineStats {
					summaryLog.Infof("[%s] total:%d inline:%d block:%d", group.Id, group.ChatCount+group.InlineCount, group.InlineCount, group.BlockCount)
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

var msgLog *log.Logger
var timerLog *log.Logger
var summaryLog *log.Logger
var errLog *log.Logger

func init() {
	_, debug := os.LookupEnv("DEBUG")
	if debug {
		log.SetReportCaller(true)
		log.SetLevel(log.DebugLevel)
	}
	log.SetTimeFormat(time.TimeOnly)
	summaryLog = log.WithPrefix("on SUMMARY")
	msgLog = log.WithPrefix("on msg")
	timerLog = log.WithPrefix("on timer")
	errLog = log.NewWithOptions(os.Stderr, log.Options{
		ReportTimestamp: true,
		ReportCaller:    true,
		TimeFormat:      "15:04:05.999999999",
	})
	styles := log.DefaultStyles()
	styles.Levels[log.ErrorLevel] = lipgloss.NewStyle().
		SetString("ERROR").
		Padding(0, 1, 0, 1).
		Background(lipgloss.Color("204")).
		Foreground(lipgloss.Color("0"))
	styles.Levels[log.FatalLevel] = lipgloss.NewStyle().
		SetString("FATAL").
		Padding(0, 1, 0, 1).
		Background(lipgloss.Color("1")).
		Foreground(lipgloss.Color("0"))

	errLog.SetStyles(styles)

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
	var setup string
	for k, v := range inlineStats {
		if v.Setup.BurnoutLimit == 0 && v.Setup.CooldownMinutes == 0 {
			inlineStats[k].Setup = gDefaultSetup
		}
		setup += fmt.Sprintf("%s: %d msg in %d min\n", v.Id, v.Setup.BurnoutLimit, v.Setup.CooldownMinutes)
		for _, bot := range v.BotsSetup {
			setup += fmt.Sprintf("    @%s: %d msg in %d min\n", bot.Id, bot.BurnoutLimit, bot.CooldownMinutes)
		}
	}
	log.Info("Read setup", "Groups", setup)
	if testEnv.Token != "" {
		gToken = testEnv.Token
		gRootID, _ = strconv.ParseInt(testEnv.AdminID, 10, 64)
		gKumaPushURL = testEnv.KumaURL
	} else {
		gToken = os.Getenv("BOT_TOKEN")
		gRootID, _ = strconv.ParseInt(os.Getenv("BOT_ADMIN_ID"), 10, 64)
		gKumaPushURL = os.Getenv("KUMA_PUSH_URL")
	}
	log.Debug("Read ENV", "gToken", gToken, "gRootID", gRootID, "gKumaPushURL", gKumaPushURL)
	k := kuma.New(gKumaPushURL)
	k.Start()
}

func DirectCmdHandler(fn func(c tele.Context) error) func(c tele.Context) error {
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
func PrivilegeMiddleWare(fn tele.HandlerFunc) tele.HandlerFunc {
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
func main() {
	pref := tele.Settings{
		Token:  gToken,
		Poller: &tele.LongPoller{Timeout: 5 * time.Second},
	}
	var err error
	bot, err = tele.NewBot(pref)
	if err != nil {
		errLog.Fatal(err)
		return
	}
	for _, v := range []string{tele.OnText, tele.OnPhoto, tele.OnAnimation, tele.OnDocument, tele.OnSticker, tele.OnVideo, tele.OnVoice} {
		bot.Handle(v, msgHandler)
	}
	bot.Handle(tele.OnAddedToGroup, func(c tele.Context) error {
		return c.Send("My pleasure to join the group! Inline messages will be limited by me.")
	})

	bot.Handle(cmdHelp, onHelp, PrivilegeMiddleWare)
	bot.Handle(cmdHeatsink, onHeatsink, PrivilegeMiddleWare)
	go bot.Start()
	go oneSecondTimer()
	go inlineCooldownRoutine()

	log.Info("online")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	bot.Stop()
	log.Info("backup data")
	err = db.Write("data", "inline", inlineStats)
	if err != nil {
		errLog.Error(err)
	} else {
		log.Info("data backup success")
	}
	<-time.After(time.Second * 1)
	log.Info("offline")
}
