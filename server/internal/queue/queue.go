package queue

import (
	"log"
	"sync"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/models"
	"github.com/mcherdakov/soa-mafia/server/internal/session"
)

type Queue struct {
	users     []*models.User
	sessionCh chan []*models.User

	mu sync.Mutex
}

func NewQueue(sessionCh chan []*models.User) *Queue {
	return &Queue{
		sessionCh: sessionCh,
	}
}

func (q *Queue) ConnectToQueue(user *models.User) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.users = append(q.users, user)
	if len(q.users) == session.SessionCapacity {
		q.sessionCh <- q.users[:]
		q.users = []*models.User{}

		return
	}

	q.sendNotification(&proto.Notifications{
		Notification: &proto.Notifications_UserConnected{
			UserConnected: &proto.UserConnectedNotification{
				Username: user.Username,
				Current:  q.usernames(),
			},
		},
	})
}

func (q *Queue) DisconnectFromQueue(username string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	filtered := []*models.User{}
	for _, queueUser := range q.users {
		if queueUser.Username == username {
			queueUser.Disconnect()

			continue
		}

		filtered = append(filtered, queueUser)
	}
	q.users = filtered

	q.sendNotification(&proto.Notifications{
		Notification: &proto.Notifications_UserDisconnected{
			UserDisconnected: &proto.UserDisconnectedNotification{
				Username: username,
				Current:  q.usernames(),
			},
		},
	})
}

func (q *Queue) sendNotification(notification *proto.Notifications) {
	for _, user := range q.users {
		err := user.Notifications.Send(notification)
		if err != nil {
			log.Println(err)
		}
	}
}

func (q *Queue) usernames() []string {
	usernames := make([]string, 0, len(q.users))
	for _, user := range q.users {
		usernames = append(usernames, user.Username)
	}

	return usernames
}
