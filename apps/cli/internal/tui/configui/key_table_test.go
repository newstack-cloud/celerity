package configui

import (
	"bytes"
	"context"
	"testing"
	"time"

	"github.com/charmbracelet/x/exp/teatest"
	"github.com/newstack-cloud/celerity/apps/cli/internal/appconfig"
	styles "github.com/newstack-cloud/deploy-cli-sdk/styles"
	"github.com/stretchr/testify/suite"
)

type KeyTableSuite struct {
	suite.Suite
	testEntries []appconfig.ConfigEntry
}

func TestKeyTableSuite(t *testing.T) {
	suite.Run(t, new(KeyTableSuite))
}

func (s *KeyTableSuite) SetupTest() {
	s.testEntries = []appconfig.ConfigEntry{
		{Key: "TIMEOUT", Value: "5000", Sensitive: false},
		{Key: "LOG_LEVEL", Value: "info", Sensitive: false},
		{Key: "API_KEY", Value: "secret123", Sensitive: true},
		{Key: "DB_PASSWORD", Value: "dbpass", Sensitive: true},
	}
}

func (s *KeyTableSuite) Test_renders_keys_with_masked_secrets() {
	mock := &mockConfigStore{entries: s.testEntries}
	model := NewKeyTableModel(context.Background(), s.testEntries, mock, "staging-app-config", styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContainsAll(
		s.T(), testModel.Output(),
		"TIMEOUT", "5000",
		"API_KEY", "********",
	)

	keyQ(testModel)
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))
}

func (s *KeyTableSuite) Test_toggle_show_secrets() {
	mock := &mockConfigStore{entries: s.testEntries}
	model := NewKeyTableModel(context.Background(), s.testEntries, mock, "staging-app-config", styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContains(s.T(), testModel.Output(), "********")

	// Press 's' to toggle secrets
	keyRune(testModel, 's')

	waitForContains(s.T(), testModel.Output(), "secret123")

	keyQ(testModel)
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))
}

func (s *KeyTableSuite) Test_navigate_down() {
	mock := &mockConfigStore{entries: s.testEntries}
	model := NewKeyTableModel(context.Background(), s.testEntries, mock, "staging-app-config", styles.NewDefaultStyles(styles.NewCelerityPalette()))

	testModel := teatest.NewTestModel(
		s.T(), model,
		teatest.WithInitialTermSize(120, 40),
	)

	waitForContains(s.T(), testModel.Output(), "> TIMEOUT")

	keyDown(testModel)
	waitForContains(s.T(), testModel.Output(), "> LOG_LEVEL")

	keyQ(testModel)
	testModel.WaitFinished(s.T(), teatest.WithFinalTimeout(5*time.Second))
}

func (s *KeyTableSuite) Test_headless_key_output() {
	buf := &bytes.Buffer{}
	DispatchHeadlessKeyOutput(
		buf, "staging-app-config", s.testEntries, false, nil,
	)

	output := buf.String()
	s.Contains(output, "TIMEOUT")
	s.Contains(output, "5000")
	s.Contains(output, "********")
	s.NotContains(output, "secret123")
	s.NotContains(output, "\033[")
}

func (s *KeyTableSuite) Test_headless_key_output_show_secrets() {
	buf := &bytes.Buffer{}
	DispatchHeadlessKeyOutput(
		buf, "staging-app-config", s.testEntries, true, nil,
	)

	output := buf.String()
	s.Contains(output, "secret123")
	s.NotContains(output, "********")
}

// mockConfigStore implements appconfig.ConfigStore for testing.
type mockConfigStore struct {
	entries   []appconfig.ConfigEntry
	getErr    error
	setErr    error
	deleteErr error
	listErr   error
	importErr error
	setCalls  []setCall
}

type setCall struct {
	Key, Value string
	Sensitive  bool
}

func (m *mockConfigStore) Get(
	ctx context.Context, key string,
) (*appconfig.ConfigEntry, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	for _, e := range m.entries {
		if e.Key == key {
			return &e, nil
		}
	}
	return nil, nil
}

func (m *mockConfigStore) Set(
	ctx context.Context,
	key, value string,
	sensitive bool,
) error {
	if m.setErr != nil {
		return m.setErr
	}
	m.setCalls = append(m.setCalls, setCall{key, value, sensitive})
	return nil
}

func (m *mockConfigStore) Delete(
	ctx context.Context, key string,
) error {
	return m.deleteErr
}

func (m *mockConfigStore) List(
	ctx context.Context,
) ([]appconfig.ConfigEntry, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}
	return m.entries, nil
}

func (m *mockConfigStore) Import(
	ctx context.Context,
	entries []appconfig.ConfigEntry,
) error {
	return m.importErr
}
