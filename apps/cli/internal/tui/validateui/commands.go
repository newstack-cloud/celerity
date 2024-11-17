package validateui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/two-hundred/celerity/libs/deploy-engine/core"
)

var (
	True = true
)

func selectBlueprintCmd(blueprintFile string) tea.Cmd {
	return func() tea.Msg {
		return SelectBlueprintMsg{
			blueprintFile: blueprintFile,
		}
	}
}

func clearSelectedBlueprintCmd() tea.Cmd {
	return func() tea.Msg {
		return ClearSelectedBlueprintMsg{}
	}
}

func startValidateStreamCmd(model ValidateModel) tea.Cmd {
	return func() tea.Msg {
		err := model.engine.ValidateStream(context.TODO(), &core.ValidateParams{
			BlueprintFile: &model.blueprintFile,
			BlueprintOnly: &True,
		}, model.resultStream, model.errStream)
		if err != nil {
			return ValidateErrMsg{err}
		}
		return nil
	}
}

func waitForNextResultCmd(model ValidateModel) tea.Cmd {
	return func() tea.Msg {
		return ValidateResultMsg(<-model.resultStream)
	}
}

func checkForErrCmd(model ValidateModel) tea.Cmd {
	return func() tea.Msg {
		var err error
		select {
		case <-time.After(1 * time.Second):
			break
		case newErr := <-model.errStream:
			err = newErr
		}
		return ValidateErrMsg{err}
	}
}
