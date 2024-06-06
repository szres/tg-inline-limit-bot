package main

import (
	"strings"

	tele "gopkg.in/telebot.v3"
)

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
