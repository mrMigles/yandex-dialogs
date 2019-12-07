package voice_mail

import (
	"log"
	"math/rand"
	"time"
)

type DatingBot struct {
	mailService MailService
}

func NewDatingBot(service MailService) DatingBot {
	rand.Seed(time.Now().Unix())
	return DatingBot{mailService: service}
}

func (m DatingBot) CheckMails() {
	log.Print("Run Dating cron")
	sentMessages := map[int]map[int]struct{}{}
	messages := m.mailService.GetMessagesForUser(&User{Number: 7070})
	for _, message := range messages {
		for i := 1; i <= len(messages); i++ {
			toMessage := messages[rand.Intn(len(messages))]
			if message.From != toMessage.From {
				if from, ok := sentMessages[message.From]; ok {
					if _, ok := from[toMessage.From]; ok {
						continue
					}
				}
				message.To = toMessage.From
				err := m.mailService.SendMessage(&message)
				if err != nil {
					log.Printf("Error with loop %v", err)
					continue
				}
				if _, ok := sentMessages[message.From]; !ok {
					sentMessages[message.From] = map[int]struct{}{}
				}
				sentMessages[message.From][message.To] = struct{}{}
			}
		}
	}
}

func (m DatingBot) GetCron() string {
	return "0 */8 * * *"
}
