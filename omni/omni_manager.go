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


//Manager manages omnidisk traffic through the pubsub and implements operations
type Manager struct{
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
func NewManager(logger *zap.Logger, nodeID peer.ID, kadDHT *dht.IpfsDHT, ps *pubsub.PubSub) (*Manager, error){
	if logger == nil{
		logger = zap.NewNop()
	}


	m := &Manager{
		logger:				logger,
		NodeID:				nodeID,
		ps:					ps,
		kadDHT:				kadDHT,
		msgPublishers:		make([]messages.Publisher, 0),
	}

	if err := m.joinOmniNet(); err != nil{
		m.logger.Error("failed joining omni network")
		return nil, err
	}

	return m, nil
}

func (m *Manager) joinOmniNet() error{
	cleanup := func(topic *pubsub.Topic, subscription *pubsub.Subscription){
		if topic != nil{
			_ = topic.Close()
		}
		if subscription != nil{
			subscription.Cancel()
		}
	}

	m.logger.Debug("joining omnidisk topic")
	topic, err := m.ps.Join(topicName)
	if err != nil{
		m.logger.Error("failed joining omni topic", zap.Error(err))
		return err
	}

	m.logger.Debug("subscribing to omni topic")
	subscription, err := topic.Subscribe()
	if err != nil{
		m.logger.Error("subscribing to omni topic: FAILED", zap.Error(err))
		cleanup(topic, subscription)
		return err
	}
	m.logger.Debug("successfuly joined omni network")

	m.topic = topic
	m.subscription = subscription

	m.logger.Debug("launching omni receiver routine")
	go m.omniReceiver()

	return nil
}

//---------------------------</SETUP>
//input: msg
//complete it (with SenderID or Signature etc..),
//then marshal & publish to omni network
func (m *Manager) OmniPublisher(msg messages.Message) error{
	var pb *genmsg.Message
	switch msg.(type){
		case *messages.MsgRbc0:
			rbc0 := msg.(*messages.MsgRbc0)
			(*rbc0).SenderID = m.NodeID.String()
			pb = (*rbc0).MarshalToProtobuf()
		default:
			m.logger.Error("trying to omni-publish foreign msg type")
			return errors.New("foreign msg type")
	}
	out, err := pb.Marshal()
	if err != nil{
		m.logger.Error("failed marshalling omni message", zap.Error(err))
		return errors.Wrap(err, "marshalling omni message")
	}

	if err := m.topic.Publish(context.Background(), out); err != nil{
		m.logger.Error("failed publishing omni message", zap.Error(err))
		return errors.Wrap(err, "publishing omni message")
	}

	return nil
}

//receive messages from omni network, process them a bit (verify signature)
//pass them to messageForwarder which will dispatch them to other parts of the node
func (m *Manager) omniReceiver(){
	pub, sub := messages.NewSubscription()
	go m.messageForwarder(sub)

	for{
		//m.logger.Debug("received omni msg")
		omniMsg, err := m.subscription.Next(context.Background())
		if err != nil{
			m.logger.Error("failed receiving omni message", zap.Error(err))
		}
		if omniMsg.ReceivedFrom == m.NodeID{
			continue
		}
		/*
		*/

		in := genmsg.Message{}
		if err := in.Unmarshal(omniMsg.Data); err != nil{
			m.logger.Warn("cannot unmarshal omni message. Ignoring", zap.Error(err))
			continue;
		}

		//TODO
		//message signature will come into play here

		msg := messages.UnmarshalFromProtobuf(&in).(messages.MsgRbc0)

		if err := pub.Publish(msg); err != nil{
			m.logger.Error("failed passing omni message to messageForwarder", zap.Error(err))
		}
	}
}

//forward messages received from omni network to other parts of the node (like rbc0)
func (m *Manager) messageForwarder(sub messages.Subscriber){
	for{
		msg, err := sub.Next()
		if err != nil{
			m.logger.Error("failed receiving msg from omniReceiver", zap.Error(err))
			continue
		}

		m.msgPublishersLock.Lock()
		for _, pub := range m.msgPublishers{
			if pub.Closed(){
				continue
			} else if err := pub.Publish(msg); err != nil{
				m.logger.Error("failed forwarding message", zap.Error(err))
			}
		}
		m.msgPublishersLock.Unlock()
	}
}

//other parts of the node can call this to receive subscriber end of channel
//over which messageForwarder will publish messages
func (m *Manager) SubscribeToMessages() messages.Subscriber{
	pub, sub := messages.NewSubscription()
	m.msgPublishersLock.Lock()
	defer m.msgPublishersLock.Unlock()
	m.msgPublishers = append(m.msgPublishers, pub)

	return sub
}

//---------MESSAGING


