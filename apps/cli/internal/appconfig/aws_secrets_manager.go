package appconfig

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/secretsmanager"
	"go.uber.org/zap"
)

// secretsManagerAPI is the subset of the Secrets Manager client used by this store.
type secretsManagerAPI interface {
	GetSecretValue(
		ctx context.Context,
		params *secretsmanager.GetSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.GetSecretValueOutput, error)
	PutSecretValue(
		ctx context.Context,
		params *secretsmanager.PutSecretValueInput,
		optFns ...func(*secretsmanager.Options),
	) (*secretsmanager.PutSecretValueOutput, error)
}

type secretsManagerStore struct {
	client   secretsManagerAPI
	secretID string
	logger   *zap.Logger
}

func (s *secretsManagerStore) Get(ctx context.Context, key string) (*ConfigEntry, error) {
	data, err := s.readSecret(ctx)
	if err != nil {
		return nil, err
	}

	value, ok := data[key]
	if !ok {
		return nil, fmt.Errorf("key %q not found in store %q", key, s.secretID)
	}

	return &ConfigEntry{
		Key:       key,
		Value:     value,
		Sensitive: true,
	}, nil
}

func (s *secretsManagerStore) Set(
	ctx context.Context,
	key, value string,
	sensitive bool,
) error {
	data, err := s.readSecret(ctx)
	if err != nil {
		return err
	}

	data[key] = value
	return s.writeSecret(ctx, data)
}

func (s *secretsManagerStore) Delete(ctx context.Context, key string) error {
	data, err := s.readSecret(ctx)
	if err != nil {
		return err
	}

	if _, ok := data[key]; !ok {
		return fmt.Errorf("key %q not found in store %q", key, s.secretID)
	}

	delete(data, key)
	return s.writeSecret(ctx, data)
}

func (s *secretsManagerStore) List(ctx context.Context) ([]ConfigEntry, error) {
	data, err := s.readSecret(ctx)
	if err != nil {
		return nil, err
	}

	entries := make([]ConfigEntry, 0, len(data))
	for k, v := range data {
		entries = append(entries, ConfigEntry{
			Key:       k,
			Value:     v,
			Sensitive: true,
		})
	}
	return entries, nil
}

func (s *secretsManagerStore) Import(
	ctx context.Context,
	entries []ConfigEntry,
) error {
	data, err := s.readSecret(ctx)
	if err != nil {
		return err
	}

	for _, e := range entries {
		data[e.Key] = e.Value
	}
	return s.writeSecret(ctx, data)
}

func (s *secretsManagerStore) readSecret(
	ctx context.Context,
) (map[string]string, error) {
	out, err := s.client.GetSecretValue(ctx, &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(s.secretID),
	})
	if err != nil {
		return nil, fmt.Errorf(
			"reading store %q. Deploy your application first if the store does not exist: %w",
			s.secretID, err,
		)
	}

	if out.SecretString == nil {
		return map[string]string{}, nil
	}

	var data map[string]string
	if err := json.Unmarshal([]byte(*out.SecretString), &data); err != nil {
		return nil, fmt.Errorf("parsing secret JSON for store %q: %w", s.secretID, err)
	}
	return data, nil
}

func (s *secretsManagerStore) writeSecret(
	ctx context.Context,
	data map[string]string,
) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("encoding secret JSON for store %q: %w", s.secretID, err)
	}

	_, err = s.client.PutSecretValue(ctx, &secretsmanager.PutSecretValueInput{
		SecretId:     aws.String(s.secretID),
		SecretString: aws.String(string(encoded)),
	})
	if err != nil {
		return fmt.Errorf("writing store %q: %w", s.secretID, err)
	}
	return nil
}
