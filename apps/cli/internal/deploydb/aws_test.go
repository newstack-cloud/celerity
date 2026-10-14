package deploydb

import (
	"testing"

	"github.com/newstack-cloud/bluelink/libs/blueprint/core"
	"github.com/newstack-cloud/bluelink/libs/blueprint/state"
	"github.com/stretchr/testify/suite"
)

type AWSResolveSuite struct {
	suite.Suite
}

func TestAWSResolveSuite(t *testing.T) {
	suite.Run(t, new(AWSResolveSuite))
}

func scalar(s string) *core.MappingNode {
	return &core.MappingNode{Scalar: core.ScalarFromString(s)}
}

func obj(fields map[string]*core.MappingNode) *core.MappingNode {
	return &core.MappingNode{Fields: fields}
}

func resource(typ, abstractName string, spec *core.MappingNode) *state.ResourceState {
	return &state.ResourceState{
		Type:     typ,
		SpecData: spec,
		Metadata: &state.ResourceMetadataState{
			Annotations: map[string]*core.MappingNode{
				abstractNameAnnotation: scalar(abstractName),
			},
		},
	}
}

func instance(resources ...*state.ResourceState) *state.InstanceState {
	m := map[string]*state.ResourceState{}
	for i, r := range resources {
		m[string(rune('a'+i))] = r
	}
	return &state.InstanceState{Resources: m}
}

func (s *AWSResolveSuite) Test_db_instance_with_custom_secret() {
	inst := instance(
		resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
			"endpoint": obj(map[string]*core.MappingNode{"address": scalar("orders.abc.rds.amazonaws.com"), "port": scalar("5432")}),
			"dbName":   scalar("orders"),
		})),
		resource(typeSecret, "ordersDb", obj(map[string]*core.MappingNode{
			"id": scalar("arn:aws:secretsmanager:eu-west-2:123:secret:orders-x"),
		})),
	)

	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("orders.abc.rds.amazonaws.com", target.Host)
	s.Assert().Equal("5432", target.Port)
	s.Assert().Equal("orders", target.Database)
	s.Assert().Equal("arn:aws:secretsmanager:eu-west-2:123:secret:orders-x", target.SecretARN)
	s.Assert().False(target.IAMAuth)
}

func (s *AWSResolveSuite) Test_cluster_with_proxy_prefers_proxy_host() {
	inst := instance(
		resource(typeDBCluster, "ordersDb", obj(map[string]*core.MappingNode{
			"endpoint":     obj(map[string]*core.MappingNode{"address": scalar("cluster.internal"), "port": scalar("5432")}),
			"databaseName": scalar("orders"),
		})),
		resource(typeDBProxy, "ordersDb", obj(map[string]*core.MappingNode{
			"endpoint": scalar("proxy.internal"), // flat string
		})),
		resource(typeSecret, "ordersDb", obj(map[string]*core.MappingNode{"id": scalar("arn:secret")})),
	)

	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("proxy.internal", target.Host, "proxy endpoint is preferred")
	s.Assert().Equal("5432", target.Port, "port comes from the cluster")
	s.Assert().Equal("orders", target.Database)
	s.Assert().Equal("arn:secret", target.SecretARN)
}

func (s *AWSResolveSuite) Test_iam_auth_detected() {
	inst := instance(resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
		"endpoint":                        obj(map[string]*core.MappingNode{"address": scalar("h"), "port": scalar("5432")}),
		"enableIAMDatabaseAuthentication": scalar("true"),
	})))

	target, err := resolveAWSTarget(inst, "ordersDb", "orders")
	s.Require().NoError(err)
	s.Assert().True(target.IAMAuth)
	s.Assert().Empty(target.SecretARN)
	s.Assert().Equal("orders", target.Database, "falls back to provided database name")
}

func (s *AWSResolveSuite) Test_iam_auth_extracts_user_and_region() {
	inst := instance(resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
		"endpoint":                        obj(map[string]*core.MappingNode{"address": scalar("h"), "port": scalar("5432")}),
		"enableIAMDatabaseAuthentication": scalar("true"),
		"masterUsername":                  scalar("app_user"),
		"arn":                             scalar("arn:aws:rds:eu-west-2:123:db:orders"),
	})))

	target, err := resolveAWSTarget(inst, "ordersDb", "orders")
	s.Require().NoError(err)
	s.Assert().True(target.IAMAuth)
	s.Assert().Equal("app_user", target.DBUser, "IAM auth needs the database user")
	s.Assert().Equal("eu-west-2", target.Region, "region derived from the RDS ARN")
}

func (s *AWSResolveSuite) Test_rds_managed_secret_fallback() {
	inst := instance(resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
		"endpoint":         obj(map[string]*core.MappingNode{"address": scalar("h"), "port": scalar("5432")}),
		"dbName":           scalar("orders"),
		"masterUserSecret": obj(map[string]*core.MappingNode{"secretArn": scalar("arn:managed")}),
	})))

	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("arn:managed", target.SecretARN)
}

func (s *AWSResolveSuite) Test_no_matching_resources_errors() {
	inst := instance(resource(typeDBInstance, "otherDb", obj(map[string]*core.MappingNode{
		"endpoint": obj(map[string]*core.MappingNode{"address": scalar("h")}),
	})))
	_, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Assert().ErrorContains(err, "no deployed RDS database")
}

func (s *AWSResolveSuite) Test_defaults_port_and_database() {
	inst := instance(resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
		"endpoint": obj(map[string]*core.MappingNode{"address": scalar("h")}),
	})))
	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("5432", target.Port)
	s.Assert().Equal("postgres", target.Database)
}

// roleResource is a concrete resource carrying the secret-role annotation as
// well as the abstract name.
func roleResource(typ, abstractName, role string, spec *core.MappingNode) *state.ResourceState {
	r := resource(typ, abstractName, spec)
	r.Metadata.Annotations[secretRoleAnnotation] = scalar(role)
	return r
}

// A database now emits two secrets under the same abstract name, and migrations
// run as the owner. The application user cannot create or alter anything, and
// until provisioning has run it does not exist in the database at all, so
// picking it here fails with a permissions error that points nowhere near the
// cause.
func (s *AWSResolveSuite) Test_app_user_secret_is_never_the_migration_credential() {
	inst := instance(
		resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
			"endpoint":         obj(map[string]*core.MappingNode{"address": scalar("h"), "port": scalar("5432")}),
			"dbName":           scalar("orders"),
			"masterUserSecret": obj(map[string]*core.MappingNode{"secretArn": scalar("arn:master")}),
		})),
		roleResource(typeSecret, "ordersDb", secretRoleAppUser, obj(map[string]*core.MappingNode{
			"id": scalar("arn:app-user"),
		})),
	)

	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("arn:master", target.SecretARN)
}

// A secret from a deploy that predates the application user carries no role
// annotation, and every one of those is a master credential.
func (s *AWSResolveSuite) Test_an_unmarked_secret_is_still_the_migration_credential() {
	inst := instance(
		resource(typeDBInstance, "ordersDb", obj(map[string]*core.MappingNode{
			"endpoint": obj(map[string]*core.MappingNode{"address": scalar("h"), "port": scalar("5432")}),
		})),
		resource(typeSecret, "ordersDb", obj(map[string]*core.MappingNode{"id": scalar("arn:legacy")})),
	)

	target, err := resolveAWSTarget(inst, "ordersDb", "")
	s.Require().NoError(err)
	s.Assert().Equal("arn:legacy", target.SecretARN)
}
