package sqlschema

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	atlasschema "ariga.io/atlas/sql/schema"
	"github.com/stretchr/testify/suite"
)

type ContractsSuite struct {
	suite.Suite
}

func TestContractsSuite(t *testing.T) {
	suite.Run(t, new(ContractsSuite))
}

func (s *ContractsSuite) writeContracts(content string) string {
	dir := s.T().TempDir()
	path := filepath.Join(dir, ContractsFileName)
	s.Require().NoError(os.WriteFile(path, []byte(content), 0o644))
	return path
}

func contractSchemas() map[string]*Schema {
	return map[string]*Schema{
		"ordersDb": {
			Tables: map[string]Table{
				"orders":    {Columns: map[string]Column{"id": {Type: "bigint", PrimaryKey: true}}},
				"customers": {Columns: map[string]Column{"id": {Type: "bigint", PrimaryKey: true}}},
			},
		},
	}
}

func (s *ContractsSuite) Test_load_and_find() {
	path := s.writeContracts(`
contracts:
  - name: revenue-pipeline
    owner: data-team
    dependencies:
      - database: ordersDb
        tables: ["orders"]
        policy: blocking
`)
	found, ok := FindContractsFile(filepath.Dir(path))
	s.Require().True(ok)
	s.Assert().Equal(path, found)

	cf, err := LoadContractsFile(path)
	s.Require().NoError(err)
	s.Require().Len(cf.Contracts, 1)
	s.Assert().Equal("revenue-pipeline", cf.Contracts[0].Name)
	s.Assert().Equal(PolicyBlocking, cf.Contracts[0].Dependencies[0].Policy)
}

func (s *ContractsSuite) Test_validate_ok() {
	cf := &ContractsFile{Contracts: []Contract{{
		Name: "revenue-pipeline",
		Dependencies: []Dependency{
			{Database: "ordersDb", Tables: []string{"orders", "customers"}, Policy: PolicyBlocking},
		},
	}}}
	s.Assert().Empty(ValidateContracts(cf, contractSchemas()))
}

func (s *ContractsSuite) Test_validate_unknown_database_and_table_and_policy() {
	cf := &ContractsFile{Contracts: []Contract{
		{Name: "a", Dependencies: []Dependency{{Database: "ghostDb", Tables: []string{"orders"}, Policy: PolicyNotify}}},
		{Name: "b", Dependencies: []Dependency{{Database: "ordersDb", Tables: []string{"ghost_table"}, Policy: PolicyBlocking}}},
		{Name: "c", Dependencies: []Dependency{{Database: "ordersDb", Tables: []string{"orders"}, Policy: "meh"}}},
		{Name: "d", Dependencies: []Dependency{{Database: "ordersDb", Tables: []string{"orders"}}}},
	}}
	problems := ValidateContracts(cf, contractSchemas())
	joined := strings.Join(problems, "\n")
	s.Assert().Contains(joined, "not a celerity/sqlDatabase resource")
	s.Assert().Contains(joined, "does not exist in its schema")
	s.Assert().Contains(joined, `invalid policy "meh"`)
	s.Assert().Contains(joined, "missing a policy")
}

func (s *ContractsSuite) Test_validate_datastore_dependency_only_checks_policy() {
	cf := &ContractsFile{Contracts: []Contract{{
		Name:         "user-analytics",
		Dependencies: []Dependency{{Datastore: "userStore", Policy: PolicyBlocking}},
	}}}
	// NoSQL datastore dependency is accepted (policy valid) without a schema.
	s.Assert().Empty(ValidateContracts(cf, contractSchemas()))
}

func (s *ContractsSuite) Test_evaluate_matches_blocking_and_notify() {
	cf := &ContractsFile{Contracts: []Contract{
		{Name: "revenue-pipeline", Owner: "data-team", Dependencies: []Dependency{
			{Database: "ordersDb", Tables: []string{"orders"}, Policy: PolicyBlocking},
		}},
		{Name: "customer-analytics", Owner: "data-team", Dependencies: []Dependency{
			{Database: "ordersDb", Tables: []string{"customers", "orders"}, Policy: PolicyNotify},
		}},
		{Name: "unrelated", Dependencies: []Dependency{
			{Database: "ordersDb", Tables: []string{"invoices"}, Policy: PolicyBlocking},
		}},
	}}

	impacts := EvaluateContracts(cf, map[string][]string{"ordersDb": {"orders"}})
	s.Require().Len(impacts, 2)
	s.Assert().True(AnyBlocking(impacts))

	// "unrelated" watches invoices, which did not change, so it is not impacted.
	names := []string{impacts[0].Contract, impacts[1].Contract}
	s.Assert().Contains(names, "revenue-pipeline")
	s.Assert().Contains(names, "customer-analytics")
	s.Assert().NotContains(names, "unrelated")
}

func (s *ContractsSuite) Test_evaluate_no_change_no_impact() {
	cf := &ContractsFile{Contracts: []Contract{{
		Name:         "revenue-pipeline",
		Dependencies: []Dependency{{Database: "ordersDb", Tables: []string{"orders"}, Policy: PolicyBlocking}},
	}}}
	s.Assert().Empty(EvaluateContracts(cf, map[string][]string{"ordersDb": {"customers"}}))
	s.Assert().Empty(EvaluateContracts(nil, map[string][]string{"ordersDb": {"orders"}}))
}

func (s *ContractsSuite) Test_changed_tables_extraction() {
	changes := []atlasschema.Change{
		&atlasschema.AddTable{T: atlasschema.NewTable("orders")},
		&atlasschema.ModifyTable{T: atlasschema.NewTable("customers")},
		&atlasschema.AddTable{T: atlasschema.NewTable(revisionsTable)}, // ignored
	}
	s.Assert().Equal([]string{"customers", "orders"}, changedTables(changes))
}

func (s *ContractsSuite) Test_render_contract_impacts() {
	var b strings.Builder
	RenderContractImpacts(&b, []ContractImpact{
		{Contract: "revenue-pipeline", Owner: "data-team", Policy: PolicyBlocking, Database: "ordersDb", Tables: []string{"orders"}},
		{Contract: "customer-analytics", Owner: "data-team", Policy: PolicyNotify, Database: "ordersDb", Tables: []string{"customers"}},
	})
	out := b.String()
	s.Assert().Contains(out, "Contracts affected:")
	s.Assert().Contains(out, "⛔ revenue-pipeline (data-team) is affected by a change to ordersDb.orders [blocking]")
	s.Assert().Contains(out, "⚠ customer-analytics (data-team) is affected by a change to ordersDb.customers [notify]")
}
