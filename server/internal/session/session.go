package session

import (
	"fmt"
	"log"
	"math/rand"
	"time"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/models"
)

const SessionCapacity = 4

type Command struct {
	Cmd      *proto.Commands
	Username string
}

type Session struct {
	users []*models.User
	alive map[string]*models.User
	roles map[string]proto.Role

	sessionID int64
	day       int64
	cmdChan   chan Command

	killed      *string
	mafiaReveal *string
}

func NewSession(users []*models.User, sessionID int64) *Session {
	alive := make(map[string]*models.User, len(users))

	for _, user := range users {
		alive[user.Username] = user
	}

	return &Session{
		users:     users,
		alive:     alive,
		sessionID: sessionID,
		roles:     make(map[string]proto.Role),
		cmdChan:   make(chan Command),
	}
}

func (s *Session) CmdChan() chan Command {
	return s.cmdChan
}

func (s *Session) Run() {
	log.Printf("running session %d\n", s.sessionID)

	roles := s.genRoles()

	for i, user := range s.users {
		s.roles[user.Username] = roles[i]

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
		end, err := s.runRound()
		if err != nil {
			log.Fatalln(err)
		}

		if end {
			return
		}
	}
}

func (s *Session) runRound() (bool, error) {
	s.day += 1

	roundStart := &proto.Notifications{
		Notification: &proto.Notifications_RoundStart{
			RoundStart: &proto.RoundStartNotification{
				Day:            s.day,
				KilledUsername: s.killed,
				MafiaUsername:  s.mafiaReveal,
				Remaining:      s.makeRemaining(),
			},
		},
	}

	for _, user := range s.users {
		if err := user.Notifications.Send(roundStart); err != nil {
			return false, err
		}
	}

	var votedOut *string

	if s.day == 1 {
		if err := s.awaitPass(); err != nil {
			return false, err
		}
	} else {
		voteResult, err := s.awaitVote()
		if err != nil {
			return false, err
		}

		votedOut = &voteResult
		delete(s.alive, voteResult)
	}

	if s.checkGameEnd() {
		return true, nil
	}

	nightTime := &proto.Notifications{
		Notification: &proto.Notifications_NightTime{
			NightTime: &proto.NightTimeNotification{
				VotedOut:  votedOut,
				Remaining: s.makeRemaining(),
			},
		},
	}

	for _, user := range s.users {
		if err := user.Notifications.Send(nightTime); err != nil {
			return false, err
		}
	}

	if s.day == 1 {
		if err := s.awaitPass(); err != nil {
			return false, err
		}
	} else {
		if err := s.awaitMafiaAndDetective(); err != nil {
			return false, err
		}

		if s.checkGameEnd() {
			return true, nil
		}
	}

	return false, nil
}

func (s *Session) checkGameEnd() bool {
	mafiaCount := 0
	civilianCount := 0

	for username := range s.alive {
		if s.roles[username] == proto.Role_CIVILIAN || s.roles[username] == proto.Role_DETECITVE {
			civilianCount++
		}

		if s.roles[username] == proto.Role_MAFIA {
			mafiaCount++
		}
	}

	var winRole *proto.Role

	if mafiaCount >= civilianCount {
		winRole = ptr(proto.Role_MAFIA)
	}

	if mafiaCount == 0 {
		winRole = ptr(proto.Role_CIVILIAN)
	}

	if winRole == nil {
		return false
	}

	result := &proto.Notifications{
		Notification: &proto.Notifications_ResultNotification{
			ResultNotification: &proto.ResultNotification{
				Winner: *winRole,
			},
		},
	}

	for _, user := range s.users {
		if err := user.Notifications.Send(result); err != nil {
			log.Println(err)
		}
	}

	return true
}

func (s *Session) awaitMafiaAndDetective() error {
	count := 0
	for username, role := range s.roles {
		if (role == proto.Role_MAFIA || role == proto.Role_DETECITVE) && s.alive[username] != nil {
			count++
		}
	}

	for i := 0; i < count; i++ {
		cmd := <-s.cmdChan

		switch cmd.Cmd.Command.(type) {
		case *proto.Commands_KillCommand:
			if s.roles[cmd.Username] != proto.Role_MAFIA {
				return fmt.Errorf("invalid role: expected mafia")
			}

			s.killed = &cmd.Cmd.GetKillCommand().Username
			delete(s.alive, *s.killed)
		case *proto.Commands_CheckCommand:
			if s.roles[cmd.Username] != proto.Role_DETECITVE {
				return fmt.Errorf("invalid role: expected detective")
			}

			check := cmd.Cmd.GetCheckCommand().Username
			if s.roles[check] == proto.Role_MAFIA {
				s.mafiaReveal = &check
			} else {
				s.mafiaReveal = nil
			}
		}
	}

	return nil
}

func (s *Session) awaitVote() (string, error) {
	votes := make(map[string]string, len(s.alive))
	count := map[string]int{}

	for cmd := range s.cmdChan {
		vote, ok := cmd.Cmd.Command.(*proto.Commands_VoteCommand)
		if !ok {
			return "", fmt.Errorf("invalid command, expected vote")
		}

		if _, ok := s.alive[cmd.Username]; !ok {
			return "", fmt.Errorf("dead user is trying to vote")
		}

		votes[cmd.Username] = vote.VoteCommand.Username
		count[vote.VoteCommand.Username] += 1

		if len(votes) == len(s.alive) {
			break
		}
	}

	curMax := 0
	curUser := ""

	for username, cnt := range count {
		if cnt > curMax {
			curMax = cnt
			curUser = username
		}
	}

	return curUser, nil
}

func (s *Session) awaitPass() error {
	alreadyAwaited := make(map[string]struct{}, SessionCapacity)

	for cmd := range s.cmdChan {
		_, ok := cmd.Cmd.Command.(*proto.Commands_PassCommand)
		if !ok {
			return fmt.Errorf("invalid command, expected pass")
		}

		if _, ok := alreadyAwaited[cmd.Username]; ok {
			continue
		}

		alreadyAwaited[cmd.Username] = struct{}{}

		if len(alreadyAwaited) == SessionCapacity {
			break
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

func (s *Session) makeRemaining() []string {
	res := []string{}

	for username := range s.alive {
		res = append(res, username)
	}

	return res
}

func ptr[T any](val T) *T {
	return &val
}
