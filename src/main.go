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
	gCooldownMinutes int = 240
)

type InlineCooldown struct {
	// Cooldown time left in minutes
	Cooldown int
	// Count of inline messages
	Count int
}

var bot *tele.Bot
var db *scribble.Driver
var inlineCD map[string]InlineCooldown

func burnoutCheck(c tele.Context) error {
	combinedID := strconv.FormatInt(c.Chat().ID, 10) + ":" + strconv.FormatInt(c.Sender().ID, 10)
	value, ok := inlineCD[combinedID]
	if !ok {
		value = InlineCooldown{gCooldownMinutes, 1}
		inlineCD[combinedID] = value
	} else {
		if value.Count >= 4 {
			msg, err := bot.Send(c.Recipient(), fmt.Sprintf("[%s](tg://user?id=%d) %s", escape(fullName(c.Sender())), c.Sender().ID, escape("your inline message burned out! It may take significant time for resetting.")), tele.ModeMarkdownV2)
			if err == nil {
				c.Delete()
				go func() {
					timer := time.NewTimer(15 * time.Second)
					<-timer.C
					bot.Delete(msg)
				}()
			}
			return err
		} else {
			value.Count++
			inlineCD[combinedID] = value
		}
	}
	return nil
}

func inlineCooldownRoutine() {
	interval := time.NewTicker(1 * time.Minute)
	defer interval.Stop()
	for range interval.C {
		keysToRemove := make([]string, 0)
		for k, v := range inlineCD {
			v.Cooldown--
			inlineCD[k] = v
			if v.Cooldown <= 0 {
				keysToRemove = append(keysToRemove, k)
			}
		}
		for _, k := range keysToRemove {
			delete(inlineCD, k)
		}
	}
}

func init() {
	db, _ = scribble.New("../db", nil)
	db.Read("test", "env", &testEnv)
	inlineCD = make(map[string]InlineCooldown)
	db.Read("data", "inline", &inlineCD)
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

	bot.Handle(tele.OnText, func(c tele.Context) error {
		if c.Message().Via != nil {
			return burnoutCheck(c)
		}
		return nil
	})
	bot.Handle(tele.OnPhoto, func(c tele.Context) error {
		if c.Message().Via != nil {
			return burnoutCheck(c)
		}
		return nil
	})

	go bot.Start()

	fmt.Println("bot online!")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, os.Interrupt, syscall.SIGTERM)
	<-sc
	bot.Stop()
	fmt.Println("before shutdown, backup data")
	db.Write("data", "inline", inlineCD)
	<-time.After(time.Second * 1)
	fmt.Println("bot offline!")
}
