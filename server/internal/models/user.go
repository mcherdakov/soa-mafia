package models

import "github.com/mcherdakov/soa-mafia/server/internal/generated/proto"

type User struct {
	Username      string
	Notifications proto.SOAMafia_ConnectQueueServer

	disconnectedChan chan struct{}
}

func NewUser(username string, s proto.SOAMafia_ConnectQueueServer) *User {
	return &User{
		Username:         username,
		Notifications:    s,
		disconnectedChan: make(chan struct{}),
	}
}

func (u *User) Disconnect() {
	select {
	case u.disconnectedChan <- struct{}{}:
	default:
	}
}

func (u *User) Disconnected() <-chan struct{} {
	return u.disconnectedChan
}
