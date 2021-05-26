package entities

import(
	genent "distributed/proto_gen/entities"
)


type EntRBC struct{
	Type uint32;
	Msg, Signature string;
}
func (e *EntRBC) MarshalToProtobuf() *genent.Entity{
	return &genent.Entity{
		Type: genent.Entity_RBC,
		Rbc: &genent.Rbc{
			Type:			e.Type,
			Msg:			e.Msg,
			Signature:	e.Signature,
		},
	}
}
