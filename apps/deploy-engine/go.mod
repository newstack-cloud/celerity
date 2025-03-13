module github.com/two-hundred/celerity/apps/deploy-engine

go 1.22.2

replace github.com/two-hundred/celerity/libs/blueprint => ../../libs/blueprint

require (
	github.com/gorilla/mux v1.8.1
	github.com/spf13/afero v1.11.0
	github.com/two-hundred/celerity/libs/blueprint v0.3.2
	go.uber.org/zap v1.27.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/matoous/go-nanoid/v2 v2.1.0 // indirect
	github.com/two-hundred/celerity/libs/common v0.1.0 // indirect
	go.uber.org/multierr v1.10.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
