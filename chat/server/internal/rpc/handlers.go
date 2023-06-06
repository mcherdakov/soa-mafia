package rpc

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/mcherdakov/soa-mafia/chat/server/internal/generated/proto"
	amqp "github.com/rabbitmq/amqp091-go"
)

type SOAChatServer struct {
	proto.UnimplementedSOAChatServer

	channel *amqp.Channel
}

func NewSOAChatServer(ch *amqp.Channel) *SOAChatServer {
	return &SOAChatServer{
		channel: ch,
	}
}

func (s *SOAChatServer) Connect(ctx context.Context, in *proto.ConnectIn) (*proto.ConnectOut, error) {
	queueName := fmt.Sprint(in.SessionId)

	err := s.channel.ExchangeDeclare(
		queueName,
		"fanout",
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return nil, err
	}

	return &proto.ConnectOut{
		Topic: queueName,
	}, nil
}

type Message struct {
	Username string `json:"username"`
	Text     string `json:"text"`
}

func (s *SOAChatServer) SendMessage(ctx context.Context, in *proto.SendMessageIn) (*proto.SendMessageOut, error) {
	body, err := json.Marshal(Message{in.Username, in.Text})
	if err != nil {
		return nil, err
	}

	err = s.channel.Publish(
		fmt.Sprint(in.SessionId),
		"",
		false,
		false,
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        body,
		},
	)
	if err != nil {
		return nil, err
	}

	return &proto.SendMessageOut{}, nil
}
