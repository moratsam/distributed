package server

import (
	"context"
	"net"
	"testing"
	_"time"

	"github.com/stretchr/testify/require"
	"google.golang.org/grpc"

	api_msg "github.com/moratsam/distry/cluster/api/v1/msg"
	api_queue "github.com/moratsam/distry/cluster/api/v1/queue"
)

func TestServer(t *testing.T){
	for scenario, fn := range map[string]func(
		t *testing.T,
		client1 api_queue.QueueClient,
		client2 api_queue.QueueClient,
	){
		"subscribe and hear own publication": testSubscribeToSelf,
		"subscribe and hear another's publication": testSubscribeAndHearAnother,
	} {
		t.Run(scenario, func(t *testing.T){
			client1, client2, teardown := setupTest(t, nil)
			defer teardown()
			fn(t, client1, client2)
		})
	}
}

func setupTest(t *testing.T, fn func()) (
	client	api_queue.QueueClient,
	client2	api_queue.QueueClient,
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

	client = api_queue.NewQueueClient(cc)
	client2 = api_queue.NewQueueClient(cc)
	
	return client, client2, func(){
		server.Stop()
		cc.Close()
		l.Close()
	}
}

func testSubscribeToSelf(t *testing.T, client, _ api_queue.QueueClient){
	ctx := context.Background()
	//get sub stream
	sub_stream, err := client.Subscribe(
		ctx,
		&api_queue.SubscriptionRequest{
			Type: api_msg.MsgType_VANILLA,
		},
	)
	require.NoError(t, err)

	//publish msg, receive ack
	ack, err := client.Publish(
		ctx, 
		&api_msg.Msg{
			Type: api_msg.MsgType_VANILLA,
			Data: "AI reconquista",
		},
	)
	require.NoError(t, err)
	if ack.Ok != true {
		t.Fatalf("got ok: %v, expected: %v", ack.Ok, true)
	}

	//receive msg from sub_stream
	msg, err := sub_stream.Recv()
	require.NoError(t, err)
	if msg.Type != api_msg.MsgType_VANILLA {
		t.Fatalf("got msg type: %v, expected: %v", msg.Type, api_msg.MsgType_VANILLA)
	}
	if msg.Data != "AI reconquista" {
		t.Fatalf("got msg data: %v, expected: %v", msg.Data, "AI reconquista")
	}
}

func testSubscribeAndHearAnother(t *testing.T, client1, client2 api_queue.QueueClient){
	ctx := context.Background()

	//client1 gets sub stream
	sub_stream, err := client1.Subscribe(
		ctx,
		&api_queue.SubscriptionRequest{
			Type: api_msg.MsgType_VANILLA,
		},
	)
	require.NoError(t, err)

	//client2 published msg
	ack, err := client2.Publish(
		ctx,
		&api_msg.Msg{
			Type: api_msg.MsgType_VANILLA,
			Data: "kurbarija",
		},
	)
	require.NoError(t, err)
	if ack.Ok != true {
		t.Fatalf("got ok %v, expected %v", ack.Ok, true)
	}

	//client1 receives client2's publication
	msg, err := sub_stream.Recv()
	require.NoError(t, err)
	if msg.Type != api_msg.MsgType_VANILLA {
		t.Fatalf("got msg type: %v, expected: %v", msg.Type, api_msg.MsgType_VANILLA)
	}
	if msg.Data != "kurbarija" {
		t.Fatalf("got msg data: %v, expected: %v", msg.Data, "kurbarija")
	}
}

