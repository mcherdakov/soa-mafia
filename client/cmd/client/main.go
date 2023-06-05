package main

import (
	"context"
	"log"

	"github.com/mcherdakov/soa-mafia/client/internal/cli"
	"github.com/mcherdakov/soa-mafia/client/internal/generated/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func run() error {
	conn, err := grpc.Dial(
		"server:9000",
		grpc.WithTransportCredentials(
			insecure.NewCredentials(),
		),
	)
	if err != nil {
		return err
	}

	client := proto.NewSOAMafiaClient(conn)
	return cli.NewCLI(client).Run(context.Background())
}

func main() {
	if err := run(); err != nil {
		log.Fatalln(err)
	}
}
