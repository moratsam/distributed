package omni

import (
	"context"
	_"fmt"
	_"math"
	"sync"
	_"time"

	"github.com/libp2p/go-libp2p-core/peer"
	dht "github.com/libp2p/go-libp2p-kad-dht"
	pubsub "github.com/libp2p/go-libp2p-pubsub"
	"github.com/pkg/errors"
	"go.uber.org/zap"

	"distry/messages"
	genmsg "distry/proto_gen/messages"
)

const (
	topicName = "reconquista_omni"
)


//OmniManager manages omnidisk traffic through the pubsub and implements operations
type OmniManager struct{
	logger		*zap.Logger
	NodeID		peer.ID
	kadDHT		*dht.IpfsDHT

	ps					*pubsub.PubSub
	subscription	*pubsub.Subscription
	topic				*pubsub.Topic

	msgPublishers			[]messages.Publisher
	msgPublishersLock		sync.RWMutex

}


//---------------------------<HELPERS>
//---------------------------</HELPERS>
//---------------------------<SETUP>
func NewOmniManager(logger *zap.Logger, nodeID peer.ID, kadDHT *dht.IpfsDHT, ps *pubsub.PubSub) (*OmniManager, error){
	if logger == nil{
		logger = zap.NewNop()
	}


	om := &OmniManager{
		logger:				logger,
		NodeID:				nodeID,
		ps:					ps,
		kadDHT:				kadDHT,
		msgPublishers:		make([]messages.Publisher, 0),
	}

	if err := om.joinOmniNet(); err != nil{
		om.logger.Error("failed joining omni network")
		return nil, err
	}

	return om, nil
}

func (om *OmniManager) joinOmniNet() error{
	cleanup := func(topic *pubsub.Topic, subscription *pubsub.Subscription){
		if topic != nil{
			_ = topic.Close()
		}
		if subscription != nil{
			subscription.Cancel()
		}
	}

	om.logger.Debug("joining omnidisk topic")
	topic, err := om.ps.Join(topicName)
	if err != nil{
		om.logger.Error("failed joining omni topic", zap.Error(err))
		return err
	}

	om.logger.Debug("subscribing to omni topic")
	subscription, err := topic.Subscribe()
	if err != nil{
		om.logger.Error("subscribing to omni topic: FAILED", zap.Error(err))
		cleanup(topic, subscription)
		return err
	}
	om.logger.Debug("successfuly joined omni network")

	om.topic = topic
	om.subscription = subscription

	om.logger.Debug("launching omni receiver routine")
	go om.omniReceiver()

	return nil
}

//---------------------------</SETUP>
//input: msg
//complete it (with SenderID or Signature etc..),
//then marshal & publish to omni network
func (om *OmniManager) OmniPublisher(msg messages.Message) error{
	var pb *genmsg.Message
	switch msg.(type){
		case *messages.MsgRbc0:
			rbc0 := msg.(*messages.MsgRbc0)
			(*rbc0).SenderID = om.NodeID.String()
			pb = (*rbc0).MarshalToProtobuf()
		default:
			om.logger.Error("trying to omni-publish foreign msg type")
			return errors.New("foreign msg type")
	}
	out, err := pb.Marshal()
	if err != nil{
		om.logger.Error("failed marshalling omni message", zap.Error(err))
		return errors.Wrap(err, "marshalling omni message")
	}

	if err := om.topic.Publish(context.Background(), out); err != nil{
		om.logger.Error("failed publishing omni message", zap.Error(err))
		return errors.Wrap(err, "publishing omni message")
	}

	return nil
}

//receive messages from omni network, process them a bit (verify signature)
//pass them to messageForwarder which will dispatch them to other parts of the node
func (om *OmniManager) omniReceiver(){
	pub, sub := messages.NewSubscription()
	go om.messageForwarder(sub)

	for{
		//om.logger.Debug("received omni msg")
		omniMsg, err := om.subscription.Next(context.Background())
		if err != nil{
			om.logger.Error("failed receiving omni message", zap.Error(err))
		}
		if omniMsg.ReceivedFrom == om.NodeID{
			continue
		}
		/*
		*/

		in := genmsg.Message{}
		if err := in.Unmarshal(omniMsg.Data); err != nil{
			om.logger.Warn("cannot unmarshal omni message. Ignoring", zap.Error(err))
			continue;
		}

		//TODO
		//message signature will come into play here

		msg := messages.UnmarshalFromProtobuf(&in).(messages.MsgRbc0)

		if err := pub.Publish(msg); err != nil{
			om.logger.Error("failed passing omni message to messageForwarder", zap.Error(err))
		}
	}
}

//forward messages received from omni network to other parts of the node (like rbc0)
func (om *OmniManager) messageForwarder(sub messages.Subscriber){
	for{
		msg, err := sub.Next()
		if err != nil{
			om.logger.Error("failed receiving msg from omniReceiver", zap.Error(err))
			continue
		}

		om.msgPublishersLock.Lock()
		for _, pub := range om.msgPublishers{
			if pub.Closed(){
				continue
			} else if err := pub.Publish(msg); err != nil{
				om.logger.Error("failed forwarding message", zap.Error(err))
			}
		}
		om.msgPublishersLock.Unlock()
	}
}

//other parts of the node can call this to receive subscriber end of channel
//over which messageForwarder will publish messages
func (om *OmniManager) SubscribeToMessages() messages.Subscriber{
	pub, sub := messages.NewSubscription()
	om.msgPublishersLock.Lock()
	defer om.msgPublishersLock.Unlock()
	om.msgPublishers = append(om.msgPublishers, pub)

	return sub
}

//---------MESSAGING


