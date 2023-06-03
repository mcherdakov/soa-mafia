package session

import (
	"log"
	"math/rand"
	"time"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/models"
)

const SessionCapacity = 4

type Session struct {
	users     []*models.User
	sessionID int64
	day       int64
}

func NewSession(users []*models.User, sessionID int64) *Session {
	return &Session{
		users:     users,
		sessionID: sessionID,
	}
}

func (s *Session) Run() {
	log.Printf("running session %d\n", s.sessionID)

	roles := s.genRoles()

	for i, user := range s.users {
		sessionNotification := &proto.Notifications{
			Notification: &proto.Notifications_EnterSession{
				EnterSession: &proto.EnterSessionNotification{
					SessionId: s.sessionID,
					Role:      roles[i],
				},
			},
		}

		if err := user.Notifications.Send(sessionNotification); err != nil {
			log.Println(err)
		}
	}

	time.Sleep(time.Second * 5)

	for {
		if err := s.runRound(); err != nil {
			log.Fatalln(err)
		}
	}
}

func (s *Session) runRound() error {
	s.day += 1

	roundStart := &proto.Notifications{
		Notification: &proto.Notifications_RoundStart{
			RoundStart: &proto.RoundStartNotification{
				Day:            s.day,
				KilledUsername: nil,
				MafiaUsername:  nil,
			},
		},
	}

	for _, user := range s.users {
		if err := user.Notifications.Send(roundStart); err != nil {
			return err
		}
	}

	return nil
}

func (s *Session) genRoles() []proto.Role {
	var roles []proto.Role

	switch SessionCapacity {
	case 4:
		roles = []proto.Role{
			proto.Role_MAFIA,
			proto.Role_DETECITVE,
			proto.Role_CIVILIAN,
			proto.Role_CIVILIAN,
		}
	default:
		log.Fatalln("unsupported session capacity")
	}

	rand.Shuffle(len(roles), func(i, j int) {
		roles[i], roles[j] = roles[j], roles[i]
	})

	return roles
}
