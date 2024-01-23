protoc -I. --include_imports --include_source_info --descriptor_set_out=app/protos/helloworld.pb ./app/protos/helloworld.proto
protoc -I. --include_imports --include_source_info --descriptor_set_out=app/protos/book.pb ./app/protos/book.proto

protoc --go_out=. --go-grpc_out=. app/protos/book.proto
protoc --go_out=. --go-grpc_out=. app/protos/helloworld.proto

protoc -I. --include_imports --include_source_info --descriptor_set_out=app/protos/student.pb ./app/protos/student.proto

protoc --go_out=. --go-grpc_out=. app/protos/student.proto