package testutils

import (
	"context"
)

// MockNoSQLSeeder records all PutItem calls for assertion.
type MockNoSQLSeeder struct {
	PutItemCalls []PutItemCall
	PutItemErr   error
}

// PutItemCall records a single PutItem invocation.
type PutItemCall struct {
	TableName string
	ItemJSON  []byte
}

func (m *MockNoSQLSeeder) PutItem(_ context.Context, tableName string, itemJSON []byte) error {
	m.PutItemCalls = append(m.PutItemCalls, PutItemCall{TableName: tableName, ItemJSON: itemJSON})
	return m.PutItemErr
}

// MockStorageUploader records all Upload calls for assertion.
type MockStorageUploader struct {
	UploadCalls []UploadCall
	UploadErr   error
}

// UploadCall records a single Upload invocation.
type UploadCall struct {
	BucketName string
	ObjectKey  string
	Data       []byte
}

func (m *MockStorageUploader) Upload(_ context.Context, bucketName string, objectKey string, data []byte) error {
	m.UploadCalls = append(m.UploadCalls, UploadCall{BucketName: bucketName, ObjectKey: objectKey, Data: data})
	return m.UploadErr
}

// MockSQLSeeder records all ExecSQL calls for assertion.
type MockSQLSeeder struct {
	ExecSQLCalls []ExecSQLCall
	ExecSQLErr   error
}

// ExecSQLCall records a single ExecSQL invocation.
type ExecSQLCall struct {
	DatabaseName string
	SQL          string
}

func (m *MockSQLSeeder) ExecSQL(_ context.Context, databaseName string, sql string) error {
	m.ExecSQLCalls = append(m.ExecSQLCalls, ExecSQLCall{DatabaseName: databaseName, SQL: sql})
	return m.ExecSQLErr
}
