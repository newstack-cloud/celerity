module github.com/two-hundred/celerity/tools/blueprint-ls

go 1.22.2

replace github.com/two-hundred/celerity/libs/blueprint => ../../libs/blueprint

require (
	github.com/davecgh/go-spew v1.1.1
	github.com/two-hundred/celerity/libs/blueprint v0.0.0-00010101000000-000000000000
	github.com/two-hundred/ls-builder v0.2.3
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/gorilla/websocket v1.5.2 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/sourcegraph/jsonrpc2 v0.2.0 // indirect
	github.com/two-hundred/celerity/libs/common v0.0.0-20240912224249-e73f42b3f4d8 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.23.0 // indirect
)
