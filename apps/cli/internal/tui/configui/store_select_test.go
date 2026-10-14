package configui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
	"github.com/stretchr/testify/suite"
)

type StoreSelectSuite struct {
	suite.Suite
	testStores []appconfig.StoreInfo
}

func TestStoreSelectSuite(t *testing.T) {
	suite.Run(t, new(StoreSelectSuite))
}

func (s *StoreSelectSuite) SetupTest() {
	s.testStores = []appconfig.StoreInfo{
		{
			ResourceName: "appConfig",
			StoreName:    "app-config",
			StoreKind:    appconfig.StoreKindParameterStore,
			PlaintextKeys: map[string]bool{
				"TIMEOUT":   true,
				"LOG_LEVEL": true,
			},
		},
		{
			ResourceName: "appSecrets",
			StoreName:    "app-secrets",
			StoreKind:    appconfig.StoreKindSecretsManager,
		},
	}
}

func (s *StoreSelectSuite) Test_renders_store_list() {
	model := NewStoreSelectModel(s.testStores, styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContainsAll(
		s.T(), testModel.Output(),
		"app-config", "app-secrets",
	)

	keyQ(testModel)
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))
}

func (s *StoreSelectSuite) Test_selecting_store_sets_choice() {
	model := NewStoreSelectModel(s.testStores, styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContains(s.T(), testModel.Output(), "app-config")

	keyEnter(testModel)
	time.Sleep(100 * time.Millisecond)

	testModel.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))

	finalModel := testModel.FinalModel(s.T()).(StoreSelectModel)
	s.True(finalModel.selected)
	s.Equal("app-config", finalModel.choice.StoreName)
}

func (s *StoreSelectSuite) Test_headless_store_output() {
	buf := &bytes.Buffer{}
	DispatchHeadlessStoreOutput(buf, s.testStores, nil)

	output := buf.String()
	s.Contains(output, "app-config")
	s.Contains(output, "app-secrets")
	s.Contains(output, "parameter-store")
	s.Contains(output, "secrets-manager")
	s.NotContains(output, "\033[")
}

func (s *StoreSelectSuite) Test_headless_store_output_empty() {
	buf := &bytes.Buffer{}
	DispatchHeadlessStoreOutput(buf, nil, nil)
	s.Contains(buf.String(), "No config stores found")
}
