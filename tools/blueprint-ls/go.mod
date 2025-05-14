module github.com/two-hundred/celerity/tools/blueprint-ls

go 1.23.4

replace github.com/two-hundred/celerity/libs/blueprint => ../../libs/blueprint

require (
	github.com/coreos/go-json v0.0.0-20231102161613-e49c8866685a
	github.com/davecgh/go-spew v1.1.1
	github.com/sourcegraph/jsonrpc2 v0.2.0
	github.com/stretchr/testify v1.10.0
	github.com/tailscale/hujson v0.0.0-20250226034555-ec1d1c113d33
	github.com/two-hundred/celerity/libs/blueprint v0.8.0
	github.com/two-hundred/celerity/libs/common v0.3.0
	github.com/two-hundred/ls-builder v0.2.3
	go.uber.org/zap v1.27.0
	gopkg.in/yaml.v3 v3.0.1
)

require (
	github.com/bradleyjkemp/cupaloy/v2 v2.8.0 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/gorilla/websocket v1.5.2 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/pmezard/go-difflib v1.0.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/net v0.23.0 // indirect
)
