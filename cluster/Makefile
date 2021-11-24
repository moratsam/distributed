compile:
	protoc api/v1/msg/*proto					\
		--go_out=. 									\
		--go_opt=paths=source_relative 		\
		--proto_path=.


	protoc api/v1/queue/*proto					\
		--go_out=. 									\
		--go-grpc_out=. 							\
		--go_opt=paths=source_relative 		\
		--go-grpc_opt=paths=source_relative	\
		--proto_path=.

test:
	go test -race ./...
