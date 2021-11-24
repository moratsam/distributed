package server

import (
	"context"
	"sync"

	"google.golang.org/grpc"
	"go.uber.org/zap"

	api_msg "github.com/moratsam/distry/cluster/api/v1/msg"
	api_queue "github.com/moratsam/distry/cluster/api/v1/queue"
)

var _ api_queue.QueueServer = (*grpcServer)(nil)

type grpcServer struct {
	api_queue.UnimplementedQueueServer

	logger			*zap.Logger
	mu 				sync.Mutex
	subscriber_map map[api_msg.MsgType] []*api_queue.Queue_SubscribeServer
}

func NewGRPCServer() (*grpc.Server, error){
	gsrv := grpc.NewServer()
	srv, err := newgrpcServer()
	if err != nil {
		return nil, err
	}
	api_queue.RegisterQueueServer(gsrv, srv)
	return gsrv, nil
}

func newgrpcServer() (*grpcServer, error) {
	srv := &grpcServer{
		logger:				zap.L().Named("server"),
		subscriber_map:	make(map[api_msg.MsgType] []*api_queue.Queue_SubscribeServer),
	}
	return srv, nil
}

//broadcast message to every subscriber
//TODO can I use just read lock before broadcasting?
func (s *grpcServer) broadcast(msg *api_msg.Msg) {
	msg_type := msg.GetType()
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, sub_stream := range s.subscriber_map[msg_type] {
		if err := (*sub_stream).Send(msg); err != nil {
			s.logger.Error("failed to broadcast message", zap.Error(err))
		}
	}
}

//someone published something, so republish it to subscribers and send ack
func (s *grpcServer) Publish(ctx context.Context, msg *api_msg.Msg) (*api_queue.Ack, error) {

		go s.broadcast(msg)

		res := &api_queue.Ack{Ok: true}
		return res, nil
}

//someone sent a subscription request, so add him to subscriber_map
func (s *grpcServer) Subscribe(
	req		*api_queue.SubscriptionRequest,
	stream	api_queue.Queue_SubscribeServer,
) error {

	s.mu.Lock()
	
	msg_type := req.GetType()
	if len(s.subscriber_map[msg_type]) == 0 {
		s.subscriber_map[msg_type] = make([]*api_queue.Queue_SubscribeServer, 0, 15)
	}
	s.subscriber_map[msg_type] = append(s.subscriber_map[msg_type], &stream)

	s.mu.Unlock()

	for {
		select{
			case <- stream.Context().Done():
				return nil
			default:
		}
	}
}
