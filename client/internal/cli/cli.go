package cli

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/mcherdakov/soa-mafia/client/internal/generated/proto"
)

type state int

const (
	stateNew state = iota
	stateNotConnectedToQueue
	stateConnectedToQueue

	stateWaitingSessionAck
	stateConnectedToSession
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
	fmt.Printf("You can connect to queue by typing %q\n", commandConnect)

	if c.input() != commandConnect {
		c.invalidCommand()
		return
	}

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
	fmt.Printf(
		"You are connected to session %d. Your role is %s\n",
		session.sessionID,
		roleName(session.role),
	)

	for {
	}
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
