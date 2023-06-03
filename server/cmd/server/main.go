package main

import (
	"log"
	"net"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/queue"
	"github.com/mcherdakov/soa-mafia/server/internal/rpc"
	"github.com/mcherdakov/soa-mafia/server/internal/session"
	"google.golang.org/grpc"
)

func run() error {
	listener, err := net.Listen("tcp", ":9000")
	if err != nil {
		return err
	}

	sessionManager := session.NewSessionManager()

	go sessionManager.Run()

	s := grpc.NewServer()
	proto.RegisterSOAMafiaServer(
		s,
		rpc.NewSOAMafiaServer(
			queue.NewQueue(sessionManager.Chan()),
			sessionManager,
		),
	)

	return s.Serve(listener)
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}
