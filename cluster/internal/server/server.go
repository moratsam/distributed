package server

import (
	"context"
	"sync"

	api "github.com/moratsam/distry/api/v1"
	"google.golang.org/grpc"
)

var _ api.QueueServer = (*grpcServer)(nil)

type grpcServer struct {
	api.UnimplementedQueueServer

	logger			*zap.Logger
	mu 				sync.Mutex
	subscriber_map map[api.MsgType] []client_streams
}

func newgrpcServer() (*grpcServer, error) {

	srv := &grpcServer{
		logger:				zap.L().Named("server"),
		subscriber_map:	make(map[api.MsgType] []api.Queue_SubscribeServer),
	}
	return srv, nil
}

//broadcast message to every subscriber
//TODO do i need to lock before broadcasting?
func (s *grpcServer) broadcast(message *api.Message) {
	//TODO get enum
	msg_type := message.GetMsgType()
	s.mu.Lock()
	defer s.mu.Unlock()
	for sub_stream := range s.subscriber_map[msg_type] {
		if err := sub_stream.Send(req); err != nil {
			s.logger.Error("failed to broadcast message", zap.Error(err))
		}
	}
}

//someone published something, so republish it to subscribers and send ack
func (s *grpcServer) Publish(stream api.Queue_PublishServer) error {
	for {
		select {
		case <- stream.Context().Done():
			return nil
		default:
			msg, err := stream.Recv()
			if err != nil {
				return err
			}

			go s.broadcast(msg)

			res = api.Ack{Ok: true}
			if err := stream.Send(res); err != nil {
				return err
			}
		}
	}
}

//someone sent a subscription request, so add him to subscriber_map
func (s *grpcServer) Subscribe(
	req		*api.SubscriptionRequest,
	stream	api.Queue_SubscribeServer
) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	msg_type := req.GetMsgType()
	if len(s.subscriber_map[msg_type]) == 0 {
		s.subscriber_map[msg_type] = make([]api.Queue_SubscribeServer, 0, 15)
	}
	s.subscriber_map[msg_type] = append(s.subscriber_map[msg_type], stream)
}
