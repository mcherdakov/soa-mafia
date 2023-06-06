package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/mcherdakov/soa-mafia/chat/client/internal/generated/proto"
)

type Message struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func input() string {
	cmd, err := bufio.NewReader(os.Stdin).ReadString('\n')
	if err != nil {
		log.Fatalln(err)
	}

	return strings.Trim(cmd, "\n")
}

func consume(topic, username string) {
	conn, err := amqp.Dial("amqp://guest:guest@rabbitmq:5672/")
	if err != nil {
		log.Fatalln(err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		log.Fatalln(err)
	}
	defer ch.Close()

	err = ch.ExchangeDeclare(topic, "fanout", false, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
	}

	q, err := ch.QueueDeclare("", false, false, true, false, nil)

	err = ch.QueueBind(q.Name, "", topic, false, nil)
	if err != nil {
		log.Fatalln(err)
	}

	msgsChan, err := ch.Consume(q.Name, "", true, false, false, false, nil)
	if err != nil {
		log.Fatalln(err)
	}

	for raw := range msgsChan {
		msg := Message{}

		if err := json.Unmarshal(raw.Body, &msg); err != nil {
			log.Fatalln(err)
		}

		if msg.Username != username {
			fmt.Printf("%s: %s\n", msg.Username, msg.Text)
		}
	}
}

func run() error {
	ctx := context.Background()

	conn, err := grpc.Dial(
		"server:9000",
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return err
	}

	client := proto.NewSOAChatClient(conn)

	fmt.Print("Enter username: ")
	username := input()

	fmt.Print("Enter session id: ")
	sessionIDString := input()
	sessionID, err := strconv.ParseInt(sessionIDString, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid session id: %w", err)
	}

	out, err := client.Connect(ctx, &proto.ConnectIn{SessionId: sessionID})
	if err != nil {
		return err
	}

	go consume(out.Topic, username)

	fmt.Println("Connected to chat. You can send and recieve messages now")

	for {
		text := input()
		_, err := client.SendMessage(ctx, &proto.SendMessageIn{
			SessionId: sessionID,
			Username:  username,
			Text:      text,
		})
		if err != nil {
			return err
		}
	}
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}
