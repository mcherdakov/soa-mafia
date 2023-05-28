gen:
	protoc -I./proto --go_out=internal/generated ./proto/service.proto
