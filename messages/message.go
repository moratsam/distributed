package messages

import(

	genmsg "distry/proto_gen/messages"

)

//message represents any packet which will get sent between nodes
type Message interface{
	//MarshalToProtobuf maps messages into protobuf messages
	MarshalToProtobuf() *genmsg.Message
}

func UnmarshalFromProtobuf(m *genmsg.Message) interface{} {
	switch m.Type{
		case genmsg.Message_RBC0:
			return MsgRbc0{
				SenderID:		m.Rbc0.SenderId,
				ProtocolID:		m.Rbc0.ProtocolId,
				Type:				m.Rbc0.Type,
				Payload:			m.Rbc0.Payload,
			}
	}

	return false;
}


