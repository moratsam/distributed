package api

import(
	"context"

	"go.uber.org/zap"

	apigen "distry/proto_gen/api"
	"distry/node"
)

type Server struct{
	apigen.UnimplementedApiServer

	logger *zap.Logger
	node node.Node
}

func NewServer(logger *zap.Logger, node node.Node) *Server{
	return &Server{
		logger:	logger,
		node:		node,
	}
}


//PING
func (s *Server) Ping(_ context.Context, _ *apigen.PingRequest) (*apigen.PingResponse, error){
	s.logger.Info("handling Ping")

	return &apigen.PingResponse{}, nil
}

//Rbc0
func (s *Server) Rbc0(_ context.Context, request *apigen.Rbc0Request) (*apigen.Rbc0Response, error){
	s.logger.Info("handling Rbc0")

	done, err := s.node.Rbc0(request.Payload)
	if err != nil{
		s.logger.Error("failed Rbc0", zap.Error(err))
		return &apigen.Rbc0Response{Done: done}, err
	}

	return &apigen.Rbc0Response{Done: done}, nil
}


