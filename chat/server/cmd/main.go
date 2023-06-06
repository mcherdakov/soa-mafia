package main

import (
	"fmt"
	"log"
	"net"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"google.golang.org/grpc"

	"github.com/mcherdakov/soa-mafia/chat/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/chat/server/internal/rpc"
)

func run() error {
	listener, err := net.Listen("tcp", ":9000")

	var conn *amqp.Connection
	for {
		conn, err = amqp.Dial("amqp://guest:guest@rabbitmq:5672")
		if err != nil {
			fmt.Println(err)

			// wait until rabbitmq starts up
			time.Sleep(time.Second)
			continue
		}

		break
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return err
	}
	defer ch.Close()

	s := grpc.NewServer()
	proto.RegisterSOAChatServer(
		s,
		rpc.NewSOAChatServer(ch),
	)

	fmt.Println("Starting server")

	return s.Serve(listener)
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}
