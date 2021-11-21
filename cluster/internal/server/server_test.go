package server

import (
	"context"
	"net"
	"testing"
	_"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	api "github.com/moratsam/distry/cluster/api/v1"
)

func TestServer(t *testing.T){
	for scenario, fn := range map[string]func(
		t *testing.T,
		client api.QueueClient,
	){
		"subscribe and hear own publication": testSubscribeToSelf,
	} {
		t.Run(scenario, func(t *testing.T){
			client, teardown := setupTest(t, nil)
			defer teardown()
			fn(t, client)
		})
	}
}

func setupTest(t *testing.T, fn func()) (
	client	api.QueueClient,
	teardown	func(),
) {
	t.Helper()

	l, err := net.Listen("tcp", ":0")
	require.NoError(t, err)

	clientOptions := []grpc.DialOption{grpc.WithInsecure()}
	cc, err := grpc.Dial(l.Addr().String(), clientOptions...)
	require.NoError(t, err)

	server, err := NewGRPCServer()
	require.NoError(t, err)

	go func(){
		server.Serve(l)
	}()

	client = api.NewQueueClient(cc)
	
	return client, func(){
		server.Stop()
		cc.Close()
		l.Close()
	}
}

func testSubscribeToSelf(t *testing.T, client api.QueueClient){
	ctx := context.Background()
	//get sub stream
	sub_stream, err := client.Subscribe(
		ctx,
		&api.SubscriptionRequest{
			Type: api.MsgType_VANILLA,
		},
	)
	require.NoError(t, err)

	//get pub stream
	pub_stream, err := client.Publish(ctx)
	require.NoError(t, err)

	//publish msg on pub_stream
	err = pub_stream.Send(&api.Message{
		Type: api.MsgType_VANILLA,
		Data: "AI reconquista",
	})
	require.NoError(t, err)

	//receive ack
	ack, err := pub_stream.Recv()
	require.NoError(t, err)
	if ack.Ok != true {
		t.Fatalf("got ok: %v, expected: %v", ack.Ok, true)
	}

	//receive msg from sub_stream
	msg, err := sub_stream.Recv()
	require.NoError(t, err)
	if msg.Type != api.MsgType_VANILLA {
		t.Fatalf("got msg type: %v, expected: %v", msg.Type, api.MsgType_VANILLA)
	}
	if msg.Data != "AI reconquista" {
		t.Fatalf("got msg data: %v, expected: %v", msg.Data, "AI reconquista")
	}

}
