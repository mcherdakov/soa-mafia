package session

import "github.com/mcherdakov/soa-mafia/server/internal/models"

type SessionManager struct {
	input        chan []*models.User
	maxSessionID int64
	sessions     map[int64]*Session
}

func NewSessionManager() *SessionManager {
	return &SessionManager{
		maxSessionID: 0,
		input:        make(chan []*models.User),
		sessions:     map[int64]*Session{},
	}
}

func (sm *SessionManager) Chan() chan []*models.User {
	return sm.input
}

func (sm *SessionManager) Run() {
	for users := range sm.input {
		sessionID := sm.maxSessionID + 1

		session := NewSession(users, sm.maxSessionID+1)
		sm.sessions[sessionID] = session
		sm.maxSessionID += 1

		go session.Run()
	}
}

func (sm *SessionManager) SessionByID(sessionID int64) *Session {
	return sm.sessions[sessionID]
}
