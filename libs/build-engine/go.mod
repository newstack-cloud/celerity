module github.com/two-hundred/celerity/libs/build-engine

go 1.22.2

replace github.com/two-hundred/celerity/libs/blueprint => ../blueprint

require (
	github.com/spf13/afero v1.11.0
	github.com/two-hundred/celerity/libs/blueprint v0.0.0-00010101000000-000000000000
	go.uber.org/zap v1.27.0
	google.golang.org/grpc v1.64.0
	google.golang.org/protobuf v1.34.1
)

require (
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/two-hundred/celerity/libs/common v0.0.0-20240813183650-16760343a5b8 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.19.0 // indirect
	golang.org/x/sys v0.15.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20231120223509-83a465c0220f // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
