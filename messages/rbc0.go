package messages

import(
	genmsg "distry/proto_gen/messages"
)


type MsgRbc0 struct{
	Type uint32;
	SenderID, ProtocolID, Payload string;
}
func (m MsgRbc0) MarshalToProtobuf() *genmsg.Message{
	return &genmsg.Message{
		Type: genmsg.Message_RBC0,
		Rbc0: &genmsg.Rbc0{
			SenderId:		m.SenderID,
			ProtocolId:		m.ProtocolID,
			Type:				m.Type,
			Payload:			m.Payload,
		},
	}
}
