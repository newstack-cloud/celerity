package appconfig

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type SecretsManagerSuite struct {
	suite.Suite
}

func TestSecretsManagerSuite(t *testing.T) {
	suite.Run(t, new(SecretsManagerSuite))
}

func (s *SecretsManagerSuite) newStore(mock *mockSecretsManagerClient) *secretsManagerStore {
	return &secretsManagerStore{
		client:   mock,
		secretID: "staging-my-store",
		logger:   zap.NewNop(),
	}
}

func (s *SecretsManagerSuite) Test_get_existing_key() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"DB_HOST":"localhost","DB_PASS":"secret"}`,
	}
	store := s.newStore(mock)

	entry, err := store.Get(context.Background(), "DB_HOST")
	s.Require().NoError(err)
	s.Equal("DB_HOST", entry.Key)
	s.Equal("localhost", entry.Value)
	s.True(entry.Sensitive)
}

func (s *SecretsManagerSuite) Test_get_missing_key() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"DB_HOST":"localhost"}`,
	}
	store := s.newStore(mock)

	_, err := store.Get(context.Background(), "MISSING")
	s.Error(err)
	s.Contains(err.Error(), "not found")
}

func (s *SecretsManagerSuite) Test_set_adds_key_to_existing() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"EXISTING":"val"}`,
	}
	store := s.newStore(mock)

	err := store.Set(context.Background(), "NEW_KEY", "new_val", true)
	s.Require().NoError(err)
	s.Require().NotNil(mock.lastPutValue)

	var data map[string]string
	s.Require().NoError(json.Unmarshal([]byte(*mock.lastPutValue), &data))
	s.Equal("val", data["EXISTING"])
	s.Equal("new_val", data["NEW_KEY"])
}

func (s *SecretsManagerSuite) Test_set_overwrites_existing_key() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"KEY":"old"}`,
	}
	store := s.newStore(mock)

	err := store.Set(context.Background(), "KEY", "new", false)
	s.Require().NoError(err)

	var data map[string]string
	s.Require().NoError(json.Unmarshal([]byte(*mock.lastPutValue), &data))
	s.Equal("new", data["KEY"])
}

func (s *SecretsManagerSuite) Test_delete_removes_key() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"KEEP":"a","REMOVE":"b"}`,
	}
	store := s.newStore(mock)

	err := store.Delete(context.Background(), "REMOVE")
	s.Require().NoError(err)

	var data map[string]string
	s.Require().NoError(json.Unmarshal([]byte(*mock.lastPutValue), &data))
	s.Equal("a", data["KEEP"])
	_, exists := data["REMOVE"]
	s.False(exists)
}

func (s *SecretsManagerSuite) Test_delete_missing_key_returns_error() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"KEY":"val"}`,
	}
	store := s.newStore(mock)

	err := store.Delete(context.Background(), "MISSING")
	s.Error(err)
	s.Contains(err.Error(), "not found")
}

func (s *SecretsManagerSuite) Test_list_returns_all_entries() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"A":"1","B":"2","C":"3"}`,
	}
	store := s.newStore(mock)

	entries, err := store.List(context.Background())
	s.Require().NoError(err)
	s.Len(entries, 3)

	keyMap := map[string]string{}
	for _, e := range entries {
		keyMap[e.Key] = e.Value
		s.True(e.Sensitive)
	}
	s.Equal("1", keyMap["A"])
	s.Equal("2", keyMap["B"])
	s.Equal("3", keyMap["C"])
}

func (s *SecretsManagerSuite) Test_import_merges_entries() {
	mock := &mockSecretsManagerClient{
		secretJSON: `{"EXISTING":"old"}`,
	}
	store := s.newStore(mock)

	err := store.Import(context.Background(), []ConfigEntry{
		{Key: "NEW1", Value: "v1"},
		{Key: "NEW2", Value: "v2"},
		{Key: "EXISTING", Value: "updated"},
	})
	s.Require().NoError(err)

	var data map[string]string
	s.Require().NoError(json.Unmarshal([]byte(*mock.lastPutValue), &data))
	s.Equal("updated", data["EXISTING"])
	s.Equal("v1", data["NEW1"])
	s.Equal("v2", data["NEW2"])
}

func (s *SecretsManagerSuite) Test_get_error_from_aws() {
	mock := &mockSecretsManagerClient{
		getErr: errMockAWS,
	}
	store := s.newStore(mock)

	_, err := store.Get(context.Background(), "KEY")
	s.Error(err)
	s.Contains(err.Error(), "reading store")
}

// mockSecretsManagerClient implements secretsManagerAPI for testing.
type mockSecretsManagerClient struct {
	secretJSON   string
	getErr       error
	putErr       error
	lastPutValue *string
}

func (m *mockSecretsManagerClient) GetSecretValue(
	ctx context.Context,
	params *secretsmanager.GetSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.GetSecretValueOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return &secretsmanager.GetSecretValueOutput{
		SecretString: aws.String(m.secretJSON),
	}, nil
}

func (m *mockSecretsManagerClient) PutSecretValue(
	ctx context.Context,
	params *secretsmanager.PutSecretValueInput,
	optFns ...func(*secretsmanager.Options),
) (*secretsmanager.PutSecretValueOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	m.lastPutValue = params.SecretString
	// Update stored JSON so subsequent reads reflect the write
	if params.SecretString != nil {
		m.secretJSON = *params.SecretString
	}
	return &secretsmanager.PutSecretValueOutput{}, nil
}
