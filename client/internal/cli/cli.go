package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"math/rand"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mcherdakov/soa-mafia/client/internal/generated/proto"
)

type mode int

const (
	modeManual mode = iota
	modeAuto
)

type state int

const (
	stateNew state = iota
	stateNotConnectedToQueue
	stateConnectedToQueue

	stateWaitingSessionAck
	stateConnectedToSession
	stateDead
)

type sessionInfo struct {
	sessionID int64
	role      proto.Role
}

type command string

const (
	commandConnect = "connect"
)

type CLI struct {
	client proto.SOAMafiaClient
	reader *bufio.Reader

	userState state
	stateLock sync.Mutex

	username           string
	notificationStream proto.SOAMafia_ConnectQueueClient
	enterSession       chan sessionInfo
	day                int64
}

func NewCLI(client proto.SOAMafiaClient) *CLI {
	return &CLI{
		client:       client,
		reader:       bufio.NewReader(os.Stdin),
		userState:    stateNew,
		enterSession: make(chan sessionInfo),
	}
}

func (c *CLI) Run(ctx context.Context) error {
	go c.handleNotifications()

	for {
		switch c.userState {
		case stateNew:
			c.handleStateNew()
		case stateNotConnectedToQueue:
			c.handleStateNotConnectedToQueue(ctx)
		case stateConnectedToQueue:
			c.handleStateConnectedToQueue(ctx)
		}
	}
}

func (c *CLI) handleNotifications() {
	for {
		if c.notificationStream == nil {
			time.Sleep(time.Millisecond * 10)
			continue
		}

		msg, err := c.notificationStream.Recv()

		c.stateLock.Lock()

		if err != nil && c.userState < stateConnectedToQueue {
			c.notificationStream = nil
			c.stateLock.Unlock()
			continue
		}

		c.stateLock.Unlock()

		if err != nil {
			log.Fatalln(err)
		}

		switch msg.Notification.(type) {
		case *proto.Notifications_UserConnected:
			connected := msg.GetUserConnected()

			if connected.Username != c.username {
				fmt.Printf(
					"user %s connected to queue\n",
					connected.Username,
				)
			}

			c.printCurrentUsers(connected.Current)
		case *proto.Notifications_UserDisconnected:
			disconnected := msg.GetUserDisconnected()

			if disconnected.Username != c.username {
				fmt.Printf(
					"user %s disconneced from queue\n",
					disconnected.Username,
				)
			}

			c.printCurrentUsers(disconnected.Current)
		case *proto.Notifications_EnterSession:
			enterSession := msg.GetEnterSession()
			c.enterSession <- sessionInfo{
				sessionID: enterSession.SessionId,
				role:      enterSession.Role,
			}

			return
		}
	}
}

func (c *CLI) handleStateNew() {
	fmt.Print("Enter username: ")

	username := c.input()
	c.username = username
	c.userState = stateNotConnectedToQueue
}

func (c *CLI) handleStateNotConnectedToQueue(ctx context.Context) {
	stream, err := c.client.ConnectQueue(ctx, &proto.ConnectQueueIn{
		Username: c.username,
	})
	if err != nil {
		fmt.Println(err)
		return
	}

	c.notificationStream = stream

	c.stateLock.Lock()
	c.userState = stateConnectedToQueue
	c.stateLock.Unlock()
}

func (c *CLI) handleStateConnectedToQueue(ctx context.Context) {
	fmt.Println("waiting for more people to join queue...")

	session := <-c.enterSession
	if err := c.runSession(ctx, session); err != nil {
		log.Fatalln(err)
	}
}

func (c *CLI) runSession(ctx context.Context, info sessionInfo) error {
	fmt.Printf(
		"You are connected to session %d. Your role is %s\nEnter mode: manual/auto, default - manual\n",
		info.sessionID,
		roleName(info.role),
	)

	var m mode
	switch c.input() {
	case "manual":
		m = modeManual
	case "auto":
		m = modeAuto
	}

	for {
		if err := c.handleDay(ctx, info, m); err != nil {
			return err
		}

		if err := c.handleNight(ctx, info, m); err != nil {
			return err
		}
	}
}

func (c *CLI) handleNight(ctx context.Context, info sessionInfo, m mode) error {
	nt, err := c.awaitNightTime()
	if err != nil {
		return err
	}

	if nt.VotedOut != nil {
		fmt.Printf("%s was voted out.\n", *nt.VotedOut)

		if *nt.VotedOut == c.username {
			c.userState = stateDead
		}
	}

	fmt.Println("Night falls.")

	if c.day == 1 {
		fmt.Println("Night 1, no action today. Enter any text to proceed")
		c.inputAnyting(m)

		_, err = c.client.SendCommand(ctx, &proto.SendCommandIn{
			SessionId: info.sessionID,
			Username:  c.username,
			Command: &proto.Commands{
				Command: &proto.Commands_PassCommand{
					PassCommand: &proto.PassCommand{},
				},
			},
		})

		return err
	}

	availableUsers := strings.Join(nt.Remaining, ", ")

	switch info.role {
	case proto.Role_CIVILIAN:
		return nil
	case proto.Role_MAFIA:
		if c.userState == stateDead {
			return nil
		}

		fmt.Printf("Pick your victim: %s\n", availableUsers)
		victim := c.getUsername(nt.Remaining, m)

		_, err := c.client.SendCommand(ctx, &proto.SendCommandIn{
			SessionId: info.sessionID,
			Username:  c.username,
			Command: &proto.Commands{
				Command: &proto.Commands_KillCommand{
					KillCommand: &proto.KillCommand{
						Username: victim,
					},
				},
			},
		})

		return err
	case proto.Role_DETECITVE:
		if c.userState == stateDead {
			return nil
		}

		fmt.Printf("Pick your suspect: %s\n", availableUsers)
		suspect := c.getUsername(nt.Remaining, m)

		_, err := c.client.SendCommand(ctx, &proto.SendCommandIn{
			SessionId: info.sessionID,
			Username:  c.username,
			Command: &proto.Commands{
				Command: &proto.Commands_CheckCommand{
					CheckCommand: &proto.CheckCommand{
						Username: suspect,
					},
				},
			},
		})

		return err
	}

	return nil
}

func (c *CLI) handleDay(ctx context.Context, info sessionInfo, m mode) error {
	rs, err := c.awaitRoundStart()
	if err != nil {
		return err
	}

	c.day = rs.Day

	if rs.Day == 1 {
		fmt.Println("Day 1, no vote today. Enter any text to proceed")
		c.inputAnyting(m)

		_, err = c.client.SendCommand(ctx, &proto.SendCommandIn{
			SessionId: info.sessionID,
			Username:  c.username,
			Command: &proto.Commands{
				Command: &proto.Commands_PassCommand{
					PassCommand: &proto.PassCommand{},
				},
			},
		})

		return err
	}

	if rs.KilledUsername != nil {
		fmt.Printf("%s was killed last night\n", *rs.KilledUsername)
		if c.username == *rs.KilledUsername {
			c.userState = stateDead
		}
	}
	if rs.MafiaUsername != nil {
		fmt.Printf("Detective found out that %s is mafia\n", *rs.MafiaUsername)
	}
	fmt.Printf(
		"Day %d. Discuss and enter username to vote, remaining: %s\n",
		rs.Day,
		strings.Join(rs.Remaining, ", "),
	)

	if c.userState == stateDead {
		return nil
	}

	vote := c.getUsername(rs.Remaining, m)
	_, err = c.client.SendCommand(ctx, &proto.SendCommandIn{
		SessionId: info.sessionID,
		Username:  c.username,
		Command: &proto.Commands{
			Command: &proto.Commands_VoteCommand{
				VoteCommand: &proto.VoteCommand{
					Username: vote,
				},
			},
		},
	})

	return err
}

func (c *CLI) inputAnyting(m mode) {
	defer func() {
		fmt.Println("Waiting for other players")
	}()

	if m == modeAuto {
		return
	}

	c.input()
}

func (c *CLI) getUsername(whiteList []string, m mode) string {
	defer func() {
		fmt.Println("Waiting for other players to pick")
	}()

	if m == modeAuto {
		name := whiteList[rand.Intn(len(whiteList))]
		fmt.Printf("Your pick is %s\n", name)

		return name
	}

	set := map[string]struct{}{}
	for _, username := range whiteList {
		set[username] = struct{}{}
	}

	for {
		name := c.input()
		if _, ok := set[name]; !ok {
			fmt.Println("invalid username, try again")
			continue
		}

		fmt.Printf("Your pick is %s\n", name)
		return name
	}
}

func (c *CLI) awaitRoundStart() (*proto.RoundStartNotification, error) {
	msg, err := c.notificationStream.Recv()
	if err != nil {
		return nil, err
	}

	switch msg.Notification.(type) {
	case *proto.Notifications_RoundStart:
		rs := msg.GetRoundStart()
		return rs, nil
	case *proto.Notifications_ResultNotification:
		result := msg.GetResultNotification()
		c.handleResult(result)

		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected notification")
	}
}

func (c *CLI) awaitNightTime() (*proto.NightTimeNotification, error) {
	msg, err := c.notificationStream.Recv()
	if err != nil {
		return nil, err
	}

	switch msg.Notification.(type) {
	case *proto.Notifications_NightTime:
		nt := msg.GetNightTime()
		return nt, nil
	case *proto.Notifications_ResultNotification:
		result := msg.GetResultNotification()
		c.handleResult(result)

		return nil, nil
	default:
		return nil, fmt.Errorf("unexpected notification")
	}
}

func (c *CLI) handleResult(result *proto.ResultNotification) {
	fmt.Printf("game finished, winner role is %s\n", roleName(result.Winner))
	os.Exit(0)
}

func (c *CLI) input() string {
	cmd, err := c.reader.ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}

	return strings.Trim(cmd, "\n")
}

func (c *CLI) invalidCommand() {
	fmt.Println("invalid command")
}

func (c *CLI) printCurrentUsers(users []string) {
	if len(users) == 0 {
		fmt.Println("Currently there are no users in the queue")
		return
	}

	fmt.Printf("Users in the queue: %s\n", strings.Join(users, ", "))
}

func roleName(r proto.Role) string {
	switch r {
	case proto.Role_CIVILIAN:
		return "civilian"
	case proto.Role_DETECITVE:
		return "detective"
	case proto.Role_MAFIA:
		return "mafia"
	default:
		return "unknown role"
	}
}
