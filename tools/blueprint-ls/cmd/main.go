package main

import (
	"github.com/tliron/commonlog"
	_ "github.com/tliron/commonlog/simple"
	"github.com/tliron/glsp/server"
	"github.com/two-hundred/celerity/tools/blueprint-ls/pkg/languageserver"
)

func main() {
	path := "blueprint-ls.log"
	commonlog.Configure(2, &path)

	state := languageserver.NewState()
	app := languageserver.NewApplication(state)
	app.Setup()

	server := server.NewServer(app.Handler(), languageserver.Name, true)

	server.RunStdio()
}
