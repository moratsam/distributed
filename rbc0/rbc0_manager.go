package rbc0

import(
	"strconv"
	"sync"

	"go.uber.org/zap"

	"distry/messages"
	"distry/omni"
)

//struct to keep info on a rbc round
//round begins when some node INITs a message and ends with a message is ACCEPTED
type roundInfo struct{
	//stage this node is in in regards to a rbc round.
	//'1':INIT, '2':ECHO, '3':READY, '4':ACCEPTED
	localStage uint32;
	peersPerStage map[uint32]int; //map [ STAGE -> number of all nodes in stage STAGE ]
	payload string //content of the message of this round
	//map [ SENDER_ID -> stage]
	//store a map of which stage which peer is in
	stageOfPeer	map[string]uint32

	
}

type Manager struct{
	logger	*zap.Logger
	omniManager *omni.Manager

	//map [ PROTOCOL_ID -> roundInfo ]
	//for every round, keep info on it
	roundInfoMap	map[string]*roundInfo

	peersNum int //number of peers in the network

	//as per bracha's article, each time msg INIT is broadcasted it needs a new protocolID.
	//this is implemented using a counter, which increments after each INIT bcast
	protocolCnt int

	//this Manager sends ACCEPTED rbc messages via msgPublisher to msgPublishers
	//other parts of the node are on the subscription end of the msgPublishers
	msgPublisher		messages.Publisher
	msgPublishers		[]messages.Publisher
	msgPublishersLock	sync.RWMutex
}

func NewManager(logger *zap.Logger, peersNum int, omniManager *omni.Manager) *Manager{
	if logger == nil{
		logger = zap.NewNop()
	}

	pub, sub := messages.NewSubscription()

	m := &Manager{
		logger:			logger,
		omniManager:	omniManager,
		roundInfoMap:	make(map[string]*roundInfo),
		peersNum:		peersNum,
		msgPublisher:	pub,
		msgPublishers:	make([]messages.Publisher, 0),
	}

	go m.messageForwarder(sub)
	go m.omniMsgReceiver()
	return m
}

func (m *Manager) omniMsgReceiver(){
	sub := m.omniManager.SubscribeToMessages()

	for{
		in, err := sub.Next()
		if err != nil{
			m.logger.Error("failed receiving msg from omniManager", zap.Error(err))
			continue
		}

		var msg messages.MsgRbc0
		switch in.(type){
			case messages.MsgRbc0:
				//m.logger.Debug("received rbc0 msg")
				msg = in.(messages.MsgRbc0)
			default:
				m.logger.Debug("rbc0 discarding foreign msg type")
				continue
		}

		thisRoundInfo, exists := m.roundInfoMap[msg.ProtocolID]
		if exists && thisRoundInfo.localStage == 4 { //this round was already accepted
			continue
		}
		if !exists{ //NEW MESSAGE ROUND (NEW PROTOCOL_ID)
			m.logger.Debug("new message round")
			m.instantiateRoundInfo(msg.ProtocolID, msg.Payload)
			m.roundInfoMap[msg.ProtocolID].peersPerStage[msg.Type]++
		} else { //UPDATE ROUND
			senderNodeStage, exists := thisRoundInfo.stageOfPeer[msg.SenderID]
			if !exists || senderNodeStage < msg.Type{
				m.roundInfoMap[msg.ProtocolID].stageOfPeer[msg.SenderID] = msg.Type
				m.roundInfoMap[msg.ProtocolID].peersPerStage[msg.Type]++
			} else {
				//sender resent a message from the same stage of a round. UNDEFINED BEHAVIOUR
				m.logger.Warn("some weird sender stage fuckery")
				continue
			}
		}

		m.checkRound(msg.ProtocolID)
	}
}

func (m *Manager) instantiateRoundInfo(protocolID, payload string){
	var ri roundInfo
	ri.localStage = 0
	ri.peersPerStage = map[uint32]int{1:0, 2:0, 3:0}
	ri.payload = payload
	ri.stageOfPeer = make(map[string]uint32)
	m.roundInfoMap[protocolID] = &ri
}


//follow the protocol description
//after a message for a round was received, check if
//enough inits/echos/readys have been received to move local node to next stage
//if yes, broadcast next stage message
//after local stage is ACCEPT, send signal to other parts of the node and cleanup maps
func (m *Manager) checkRound(protocolID string){
	localStage := m.roundInfoMap[protocolID].localStage
	inits := m.roundInfoMap[protocolID].peersPerStage[1]
	echos := m.roundInfoMap[protocolID].peersPerStage[2]
	readys := m.roundInfoMap[protocolID].peersPerStage[3]
	if localStage == 4{ //received messages after already accepted, just ignore
		return
	}

	n := m.peersNum //number of all nodes (matching bracha's naming scheme)
	t := n/3 -1 //max number of faulty proccesses (matching bracha's naming scheme)
	if t < 0{ //so algo works even when peersNum <= 3
		t = 0
	}

	if readys >= 1 + 2*t{ // enter ACCEPT stage
		if localStage != 3{ //if READY was not yet sent before, send it
			m.broadcast(protocolID, 3) //broadcast ready
		}
		m.roundInfoMap[protocolID].localStage = 4
		m.logger.Info("round %s ACCEPTADO:",
			zap.String("protocolID", protocolID),
			zap.String("pld", m.roundInfoMap[protocolID].payload),
		)

		//send out accepted message to other the messageForwarder
		var msg messages.MsgRbc0
		msg.Payload = m.roundInfoMap[protocolID].payload
		if err := m.msgPublisher.Publish(msg); err != nil{
			m.logger.Error("failed passing rbc message to messageForwarder")
		}
		//cleanup round resources
		m.roundInfoMap[protocolID].payload = ""
		m.roundInfoMap[protocolID].peersPerStage = nil
		m.roundInfoMap[protocolID].stageOfPeer = nil
	} else if echos >= (n+t)/2 || readys >= t+1{ // enter READY stage
		m.roundInfoMap[protocolID].localStage = 3
		m.broadcast(protocolID, 3) //broadcast ready
	} else if inits == 1 || echos >= (n+t)/2 || readys >= t+1{ // enter ECHO stage
		m.roundInfoMap[protocolID].localStage = 2
		m.broadcast(protocolID, 2) //broadcast echo
	}
}

//stage: '1':INIT, '2':ECHO, '3':READY
func (m *Manager) broadcast(protocolID string, stage uint32){
	msg := messages.MsgRbc0{
		ProtocolID:		protocolID,
		Type:				stage, //INIT, see proto/messages.proto
		Payload:			m.roundInfoMap[protocolID].payload,
	}
	if err := m.omniManager.OmniPublisher(&msg); err != nil{
		m.logger.Error("sending rbc0 msg in round FAILED", zap.String("protocolID", protocolID))
	}
}


//forward messages received from omni network to other parts of the node (like rbc0)
func (m *Manager) messageForwarder(sub messages.Subscriber){
	for{
		msg, err := sub.Next()
		if err != nil{
			m.logger.Error("failer receiving msg from rbc receiver", zap.Error(err))
			continue
		}

		m.msgPublishersLock.Lock()
		for _, pub := range m.msgPublishers{
			if pub.Closed(){
				continue
			} else if err := pub.Publish(msg); err != nil{
				m.logger.Error("failed forwarding messsage", zap.Error(err))
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

func (m *Manager) Broadcast(nodeID, payload string) (bool, error){
	protocolID := nodeID + "_" + strconv.Itoa(m.protocolCnt)
	m.instantiateRoundInfo(protocolID, payload)
	sub := m.SubscribeToMessages()

	m.broadcast(protocolID, 1)
	m.logger.Debug("sending rbc0 INIT: DONE", zap.String("protocolID", protocolID))
	m.protocolCnt++

	//wait for message to be ACCEPTADO
	//TODO timeout ?
	_, err := sub.Next()
	sub.Close()
	if err != nil {
		m.logger.Error("failed receiving message initiated by BROADCAST", zap.Error(err))
		return false, err
	}
	return true, nil
}
