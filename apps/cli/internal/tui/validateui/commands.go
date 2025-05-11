package validateui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/two-hundred/celerity/apps/cli/internal/engine"
	"github.com/two-hundred/celerity/libs/deploy-engine-client/types"
	"go.uber.org/zap"
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

func startValidateStreamCmd(model ValidateModel, logger *zap.Logger) tea.Cmd {
	return func() tea.Msg {
		blueprintValidation, err := model.engine.CreateBlueprintValidation(
			context.TODO(),
			&types.CreateBlueprintValidationPayoad{
				BlueprintDocumentInfo: types.BlueprintDocumentInfo{
					FileSourceScheme: "file",
					Directory:        "/",
					BlueprintFile:    model.blueprintFile,
				},
			},
		)
		if err != nil {
			time.Sleep(10 * time.Second)
			return ValidateErrMsg{engine.SimplifyError(err, logger)}
		}

		err = model.engine.StreamBlueprintValidationEvents(
			context.TODO(),
			blueprintValidation.ID,
			model.resultStream,
			model.errStream,
		)
		if err != nil {
			return ValidateErrMsg{err}
		}
		return nil
	}
}

func waitForNextResultCmd(model ValidateModel) tea.Cmd {
	return func() tea.Msg {
		event := <-model.resultStream
		return ValidateResultMsg(&event)
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
