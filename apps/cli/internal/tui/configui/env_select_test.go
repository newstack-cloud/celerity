package configui

import (
	"bytes"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/newstack-cloud/celerity/apps/cli/internal/devconfig"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
	"github.com/stretchr/testify/suite"
)

type EnvSelectSuite struct {
	suite.Suite
	testEnvs []devconfig.EnvironmentInfo
}

func TestEnvSelectSuite(t *testing.T) {
	suite.Run(t, new(EnvSelectSuite))
}

func (s *EnvSelectSuite) SetupTest() {
	s.testEnvs = []devconfig.EnvironmentInfo{
		{
			Name:         "staging",
			DeployTarget: "aws",
			AppEnv:       "staging",
			Region:       "us-east-1",
			Profile:      "staging-acct",
		},
		{
			Name:         "production",
			DeployTarget: "aws",
			AppEnv:       "production",
			Region:       "eu-west-1",
			Profile:      "prod-acct",
		},
	}
}

func (s *EnvSelectSuite) Test_renders_environment_list() {
	model := NewEnvSelectModel(s.testEnvs, styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContainsAll(s.T(), testModel.Output(), "staging", "production")

	keyQ(testModel)
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))
}

func (s *EnvSelectSuite) Test_selecting_env_sets_choice() {
	model := NewEnvSelectModel(s.testEnvs, styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContains(s.T(), testModel.Output(), "staging")

	// Selecting the first env (staging) emits EnvSelectedMsg without quitting,
	// which the parent model handles, so quit manually here.
	keyEnter(testModel)

	// Give time for the selection to be processed
	time.Sleep(100 * time.Millisecond)

	// Quit the program
	testModel.Send(tea.KeyMsg{Type: tea.KeyCtrlC})
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))

	finalModel := testModel.FinalModel(s.T()).(EnvSelectModel)
	s.True(finalModel.selected)
	s.Equal("staging", finalModel.choice.Name)
}

func (s *EnvSelectSuite) Test_headless_env_output() {
	buf := &bytes.Buffer{}
	DispatchHeadlessEnvOutput(buf, s.testEnvs, nil)

	output := buf.String()
	s.Contains(output, "staging")
	s.Contains(output, "production")
	s.Contains(output, "us-east-1")
	s.Contains(output, "eu-west-1")
	s.NotContains(output, "\033[")
}

func (s *EnvSelectSuite) Test_headless_env_output_empty() {
	buf := &bytes.Buffer{}
	DispatchHeadlessEnvOutput(buf, nil, nil)
	s.Contains(buf.String(), "No environments found")
}
