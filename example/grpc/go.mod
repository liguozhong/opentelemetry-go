module go.opentelemetry.io/otel/example/grpc

go 1.13

replace go.opentelemetry.io/otel => ../..

require (
	github.com/golang/protobuf v1.3.2
	go.opentelemetry.io/otel v0.3.0
	google.golang.org/grpc v1.27.1
)
