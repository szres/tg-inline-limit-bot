package main

import (
	"time"

	"github.com/charmbracelet/log"
	tele "gopkg.in/telebot.v3"
)

type MsgWithTimeout struct {
	Msg  tele.StoredMessage `json:"msg"`
	Time time.Time          `json:"time"`
}

var msgs2Delete []MsgWithTimeout

func msgInit() {
	msgs2Delete = make([]MsgWithTimeout, 0)
	db.Read("data", "msg2delete", &msgs2Delete)
	go oneSecondTimer()
}

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
