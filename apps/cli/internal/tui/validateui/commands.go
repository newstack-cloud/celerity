package validateui

import (
	"context"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/newstack-cloud/bluelink/libs/deploy-engine-client/types"
	"github.com/newstack-cloud/celerity/apps/cli/internal/engine"
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
			&types.CreateBlueprintValidationPayload{
				BlueprintDocumentInfo: types.BlueprintDocumentInfo{
					FileSourceScheme: "file",
					Directory:        "/",
					BlueprintFile:    model.blueprintFile,
				},
			},
			&types.CreateBlueprintValidationQuery{},
		)
		if err != nil {
			return ValidateErrMsg{engine.SimplifyError(err, logger)}
		}

		// LastEventID is exclusive and comes from the create call, so the
		// stream starts after whatever had already happened by then rather
		// than replaying it.
		err = model.engine.StreamBlueprintValidationEvents(
			context.TODO(),
			blueprintValidation.Data.ID,
			blueprintValidation.LastEventID,
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
