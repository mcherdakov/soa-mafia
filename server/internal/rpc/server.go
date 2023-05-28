package rpc

import (
	"context"
	"log"

	"github.com/mcherdakov/soa-mafia/server/internal/generated/proto"
	"github.com/mcherdakov/soa-mafia/server/internal/models"
	"github.com/mcherdakov/soa-mafia/server/internal/queue"
)

type SOAMafiaServer struct {
	proto.UnimplementedSOAMafiaServer

	queue *queue.Queue
}

func NewSOAMafiaServer(q *queue.Queue) *SOAMafiaServer {
	return &SOAMafiaServer{
		queue: q,
	}
}

func (s *SOAMafiaServer) ConnectQueue(in *proto.ConnectQueueIn, srv proto.SOAMafia_ConnectQueueServer) error {
	user := models.NewUser(in.Username, srv)

	s.queue.ConnectToQueue(user)
	log.Printf("user %s connected", user.Username)

	select {
	case <-srv.Context().Done():
		s.queue.DisconnectFromQueue(user.Username)
	case <-user.Disconnected():
	}

	log.Printf("user %s disconnected", user.Username)

	return nil
}

func (s *SOAMafiaServer) DisconnectQueue(ctx context.Context, in *proto.DisconnectQueueIn) (*proto.DisconnectQueueOut, error) {
	s.queue.DisconnectFromQueue(in.Username)

	return &proto.DisconnectQueueOut{Ok: true}, nil
}
