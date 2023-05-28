package session

import "github.com/mcherdakov/soa-mafia/server/internal/models"

type SessionManager struct {
	input        chan []*models.User
	maxSessionID int64
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		maxSessionID: 0,
		input:        make(chan []*models.User),
	}
}

func (sm *SessionManager) Chan() chan []*models.User {
	return sm.input
}

func (sm *SessionManager) Run() {
	for users := range sm.input {
		go NewSession(users, sm.maxSessionID+1).Run()
		sm.maxSessionID += 1
	}
}
