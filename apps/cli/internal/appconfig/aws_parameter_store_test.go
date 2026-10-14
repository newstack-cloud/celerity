package appconfig

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ssm"
	ssmtypes "github.com/aws/aws-sdk-go-v2/service/ssm/types"
	"github.com/stretchr/testify/suite"
	"go.uber.org/zap"
)

type ParameterStoreSuite struct {
	suite.Suite
}

func TestParameterStoreSuite(t *testing.T) {
	suite.Run(t, new(ParameterStoreSuite))
}

func (s *ParameterStoreSuite) newStore(mock *mockSSMClient) *parameterStoreStore {
	return &parameterStoreStore{
		client:     mock,
		pathPrefix: "/staging-my-store/",
		plaintextKeys: map[string]bool{
			"TIMEOUT":   true,
			"LOG_LEVEL": true,
		},
		logger: zap.NewNop(),
	}
}

func (s *ParameterStoreSuite) Test_get_string_parameter() {
	mock := &mockSSMClient{
		params: map[string]ssmtypes.Parameter{
			"/staging-my-store/TIMEOUT": {
				Name:  aws.String("/staging-my-store/TIMEOUT"),
				Value: aws.String("5000"),
				Type:  ssmtypes.ParameterTypeString,
			},
		},
	}
	store := s.newStore(mock)

	entry, err := store.Get(context.Background(), "TIMEOUT")
	s.Require().NoError(err)
	s.Equal("TIMEOUT", entry.Key)
	s.Equal("5000", entry.Value)
	s.False(entry.Sensitive)
}

func (s *ParameterStoreSuite) Test_get_secure_string_parameter() {
	mock := &mockSSMClient{
		params: map[string]ssmtypes.Parameter{
			"/staging-my-store/API_KEY": {
				Name:  aws.String("/staging-my-store/API_KEY"),
				Value: aws.String("secret123"),
				Type:  ssmtypes.ParameterTypeSecureString,
			},
		},
	}
	store := s.newStore(mock)

	entry, err := store.Get(context.Background(), "API_KEY")
	s.Require().NoError(err)
	s.True(entry.Sensitive)
}

func (s *ParameterStoreSuite) Test_get_string_not_in_plaintext_list_is_not_sensitive() {
	mock := &mockSSMClient{
		params: map[string]ssmtypes.Parameter{
			"/staging-my-store/CUSTOM_FLAG": {
				Name:  aws.String("/staging-my-store/CUSTOM_FLAG"),
				Value: aws.String("enabled"),
				Type:  ssmtypes.ParameterTypeString,
			},
		},
	}
	store := s.newStore(mock)

	// CUSTOM_FLAG is not in the plaintext list but is stored as String
	entry, err := store.Get(context.Background(), "CUSTOM_FLAG")
	s.Require().NoError(err)
	s.False(entry.Sensitive)
}

func (s *ParameterStoreSuite) Test_set_plaintext_uses_string_type() {
	mock := &mockSSMClient{params: map[string]ssmtypes.Parameter{}}
	store := s.newStore(mock)

	err := store.Set(context.Background(), "TIMEOUT", "3000", false)
	s.Require().NoError(err)
	s.Equal(ssmtypes.ParameterTypeString, mock.lastPutType)
}

func (s *ParameterStoreSuite) Test_set_sensitive_uses_secure_string_type() {
	mock := &mockSSMClient{params: map[string]ssmtypes.Parameter{}}
	store := s.newStore(mock)

	err := store.Set(context.Background(), "API_KEY", "secret", true)
	s.Require().NoError(err)
	s.Equal(ssmtypes.ParameterTypeSecureString, mock.lastPutType)
}

func (s *ParameterStoreSuite) Test_set_plaintext_key_with_secret_flag_uses_secure_string() {
	mock := &mockSSMClient{params: map[string]ssmtypes.Parameter{}}
	store := s.newStore(mock)

	// Even though TIMEOUT is in plaintext list, --secret flag forces SecureString
	err := store.Set(context.Background(), "TIMEOUT", "val", true)
	s.Require().NoError(err)
	s.Equal(ssmtypes.ParameterTypeSecureString, mock.lastPutType)
}

func (s *ParameterStoreSuite) Test_delete_parameter() {
	mock := &mockSSMClient{params: map[string]ssmtypes.Parameter{}}
	store := s.newStore(mock)

	err := store.Delete(context.Background(), "OLD_KEY")
	s.Require().NoError(err)
	s.Equal("/staging-my-store/OLD_KEY", mock.lastDeleteName)
}

func (s *ParameterStoreSuite) Test_list_parameters_uses_backend_type() {
	mock := &mockSSMClient{
		pathParams: []ssmtypes.Parameter{
			{Name: aws.String("/staging-my-store/TIMEOUT"), Value: aws.String("5000"), Type: ssmtypes.ParameterTypeString},
			{Name: aws.String("/staging-my-store/API_KEY"), Value: aws.String("secret"), Type: ssmtypes.ParameterTypeSecureString},
			{Name: aws.String("/staging-my-store/LOG_LEVEL"), Value: aws.String("info"), Type: ssmtypes.ParameterTypeString},
			{Name: aws.String("/staging-my-store/CUSTOM"), Value: aws.String("val"), Type: ssmtypes.ParameterTypeString},
		},
	}
	store := s.newStore(mock)

	entries, err := store.List(context.Background())
	s.Require().NoError(err)
	s.Len(entries, 4)

	keyMap := map[string]ConfigEntry{}
	for _, e := range entries {
		keyMap[e.Key] = e
	}
	s.False(keyMap["TIMEOUT"].Sensitive)
	s.True(keyMap["API_KEY"].Sensitive)
	s.False(keyMap["LOG_LEVEL"].Sensitive)
	// CUSTOM is not in the plaintext list but is stored as String, so it must
	// not come back as sensitive.
	s.False(keyMap["CUSTOM"].Sensitive)
}

func (s *ParameterStoreSuite) Test_import_sets_each_entry() {
	mock := &mockSSMClient{params: map[string]ssmtypes.Parameter{}}
	store := s.newStore(mock)

	err := store.Import(context.Background(), []ConfigEntry{
		{Key: "TIMEOUT", Value: "3000", Sensitive: false},
		{Key: "API_KEY", Value: "secret", Sensitive: true},
	})
	s.Require().NoError(err)
	s.Equal(2, mock.putCount)
}

func (s *ParameterStoreSuite) Test_get_error_from_aws() {
	mock := &mockSSMClient{getErr: errMockAWS}
	store := s.newStore(mock)

	_, err := store.Get(context.Background(), "KEY")
	s.Error(err)
	s.Contains(err.Error(), "getting parameter")
}

// mockSSMClient implements ssmAPI for testing.
type mockSSMClient struct {
	params         map[string]ssmtypes.Parameter
	pathParams     []ssmtypes.Parameter
	getErr         error
	putErr         error
	deleteErr      error
	listErr        error
	lastPutType    ssmtypes.ParameterType
	lastDeleteName string
	putCount       int
}

func (m *mockSSMClient) GetParameter(
	ctx context.Context,
	params *ssm.GetParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParameterOutput, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	name := aws.ToString(params.Name)
	p, ok := m.params[name]
	if !ok {
		return nil, fmt.Errorf("parameter %q not found", name)
	}
	return &ssm.GetParameterOutput{Parameter: &p}, nil
}

func (m *mockSSMClient) PutParameter(
	ctx context.Context,
	params *ssm.PutParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.PutParameterOutput, error) {
	if m.putErr != nil {
		return nil, m.putErr
	}
	m.lastPutType = params.Type
	m.putCount++
	name := aws.ToString(params.Name)
	m.params[name] = ssmtypes.Parameter{
		Name:  params.Name,
		Value: params.Value,
	}
	return &ssm.PutParameterOutput{}, nil
}

func (m *mockSSMClient) DeleteParameter(
	ctx context.Context,
	params *ssm.DeleteParameterInput,
	optFns ...func(*ssm.Options),
) (*ssm.DeleteParameterOutput, error) {
	if m.deleteErr != nil {
		return nil, m.deleteErr
	}
	m.lastDeleteName = aws.ToString(params.Name)
	return &ssm.DeleteParameterOutput{}, nil
}

func (m *mockSSMClient) GetParametersByPath(
	ctx context.Context,
	params *ssm.GetParametersByPathInput,
	optFns ...func(*ssm.Options),
) (*ssm.GetParametersByPathOutput, error) {
	if m.listErr != nil {
		return nil, m.listErr
	}

	prefix := aws.ToString(params.Path)
	var filtered []ssmtypes.Parameter
	for _, p := range m.pathParams {
		if strings.HasPrefix(aws.ToString(p.Name), prefix) {
			filtered = append(filtered, p)
		}
	}
	return &ssm.GetParametersByPathOutput{Parameters: filtered}, nil
}
