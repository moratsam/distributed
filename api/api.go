package api

import(
	"context"

	"go.uber.org/zap"

	apigen "distributed/proto_gen/api"
	"distributed/node"
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

//RBC
func (s *Server) RBC(_ context.Context, request *apigen.RBCRequest) (*apigen.RBCResponse, error){
	s.logger.Info("handling RBC")

	done, err := s.node.RBC(request.Msg)
	if err != nil{
		s.logger.Error("failed RBC", zap.Error(err))
		return &apigen.RBCResponse{Done: done}, err
	}

	return &apigen.RBCResponse{Done: done}, nil
}


