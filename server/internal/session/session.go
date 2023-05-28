package session

import (
	"log"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/models"
)

const SessionCapacity = 2

type Session struct {
	users     []*models.User
	sessionID int64
}

func NewSession(users []*models.User, sessionID int64) *Session {
	return &Session{
		users:     users,
		sessionID: sessionID,
	}
}

func (s *Session) Run() {
	log.Printf("running session %d\n", s.sessionID)

	sessionNotification := &proto.Notifications{
		Notification: &proto.Notifications_EnterSession{
			EnterSession: &proto.EnterSessionNotification{
				SessionId: s.sessionID,
			},
		},
	}

	for _, user := range s.users {
		if err := user.Notifications.Send(sessionNotification); err != nil {
			log.Println(err)
		}
	}

	for {
	}
}
