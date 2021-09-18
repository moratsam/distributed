package rbc0

import(
	_"fmt"
	"strconv"

	"go.uber.org/zap"

	"distry/messages"
	"distry/omni"
)

//struct to keep tab on a rbc round
type roundTab struct{
	//stage this node is in in regards to a rbc round.
	//'1':INIT, '2':ECHO, '3':READY, '4':ACCEPTED
	localStage uint32;
	peersStageTotals map[uint32]int; //map [ STAGE -> number of all nodes in stage STAGE ]
}

type Manager struct{
	logger	*zap.Logger
	omniManager *omni.Manager

	//map [PROTOCOL_ID -> message payload ]
	msgPayloadMap map[string]string

	//map [ PROTOCOL_ID -> map [ SENDER_ID -> stage] ]
	//as per bracha's article, each rbc round is defined by unique ProtocolID
	//for every round , store a map of which stage which sender is in
	rbcMap		map[string]map[string]uint32

	//map [ PROTOCOL_ID -> roundTab ]
	//for every round, keep tab on totals
	roundTabMap	map[string]*roundTab

	peersNum int //number of peers in the network

	//as per bracha's article, each time msg INIT is broadcasted it needs a new protocolID.
	//this is implemented using a counter, which increments after each INIT bcast
	protocolCnt int
}

func NewManager(logger *zap.Logger, peersNum int, omniManager *omni.Manager) *Manager{
	if logger == nil{
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:				logger,
		omniManager:		omniManager,
		msgPayloadMap:		make(map[string]string),
		rbcMap:				make(map[string](map[string]uint32)),
		roundTabMap:		make(map[string]*roundTab),
		peersNum:			peersNum,
	}

	go m.msgReceiver()
	return m
}

func (m *Manager) msgReceiver(){
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

		mapOfRound, exists := m.rbcMap[msg.ProtocolID]
		if !exists{ //NEW MESSAGE ROUND (NEW PROTOCOL_ID)
			m.msgPayloadMap[msg.ProtocolID] = msg.Payload
			m.rbcMap[msg.ProtocolID] = make(map[string]uint32)
			rt := &roundTab{0, map[uint32]int{1:0, 2:0, 3:0}}
			m.roundTabMap[msg.ProtocolID] = rt
			m.roundTabMap[msg.ProtocolID].peersStageTotals[msg.Type]++
		} else {
			senderNodeStage, exists := mapOfRound[msg.SenderID]
			if !exists || senderNodeStage < msg.Type{
				m.rbcMap[msg.ProtocolID][msg.SenderID] = msg.Type
				m.roundTabMap[msg.ProtocolID].peersStageTotals[msg.Type]++
			} else {
				//sender resent a message from the same stage of a round. UNDEFINED BEHAVIOUR
				m.logger.Warn("some weird sender stage fuckery")
				continue
			}
		}

		m.checkRound(msg.ProtocolID)
	}
}

//follow the protocol description
//after a message for a round was received, check if
//enough inits/echos/readys was received to move local node to next stage
//if yes, broadcast next stage message
//after local stage is ACCEPT, send signal to other parts of the node and cleanup maps
func (m *Manager) checkRound(protocolID string){
	localStage := m.roundTabMap[protocolID].localStage
	inits := m.roundTabMap[protocolID].peersStageTotals[1]
	echos := m.roundTabMap[protocolID].peersStageTotals[2]
	readys := m.roundTabMap[protocolID].peersStageTotals[3]
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
		m.roundTabMap[protocolID].localStage = 4
		//TODO
		//signal message was accepted
		m.logger.Info("rbc0 ACCEPTADO:", zap.String("pld", m.msgPayloadMap[protocolID]))
		//	TODO
		//cleanup maps
		//delete(m.msgPayloadMap, protocolID)
		//delete(m.rbcMap, protocolID)
		//delete(m.roundTabMap, protocolID)
	} else if echos >= (n+t)/2 || readys >= t+1{ // enter READY stage
		m.roundTabMap[protocolID].localStage = 3
		m.broadcast(protocolID, 3) //broadcast ready
	} else if inits == 1 || echos >= (n+t)/2 || readys >= t+1{ // enter ECHO stage
		m.roundTabMap[protocolID].localStage = 2
		m.broadcast(protocolID, 2) //broadcast echo
	}
}

//stage: '1':INIT, '2':ECHO, '3':READY
func (m *Manager) broadcast(protocolID string, stage uint32){
	msg := messages.MsgRbc0{
		ProtocolID:		protocolID,
		Type:				stage, //INIT, see proto/messages.proto
		Payload:			m.msgPayloadMap[protocolID],
	}
	if err := m.omniManager.OmniPublisher(&msg); err != nil{
		m.logger.Error("sending rbc0 msg: FAILED")
	}
}

func (m *Manager) InitBroadcast(nodeID, payload string) (bool, error){
	protocolID := nodeID + "_" + strconv.Itoa(m.protocolCnt)
	m.msgPayloadMap[protocolID] = payload //populate payload map for new message

	m.broadcast(protocolID, 1)
	m.logger.Debug("sending rbc0 INIT: DONE")

	m.protocolCnt++
	//TODO
	//wait for message to be ACCEPTADO
	return true, nil
}

