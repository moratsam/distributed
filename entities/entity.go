package entities

import(

	genent "distributed/proto_gen/entities"

)

//entity represents any packet which will get sent between nodes
type Entity interface{
	//MarshalToProtobuf maps entities into protobuf entities
	MarshalToProtobuf() *genent.Entity
}

func UnmarshalFromProtobuf(e *genent.Entity) interface{} {
	switch e.Type{
		case genent.Entity_RBC:
			return EntRBC{
				Type:			e.Rbc.Type,
				Msg:			e.Rbc.Msg,
				Signature:	e.Rbc.Signature,
			}
	}

	return false;
}


