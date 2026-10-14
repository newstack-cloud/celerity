package sqlschema

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"ariga.io/atlas/sql/schema"
	"gopkg.in/yaml.v3"
)

// ContractsFileName is the name of the file data teams maintain alongside the
// blueprint to declare their schema dependencies.
const ContractsFileName = "schema-contracts.yaml"

// ContractPolicy is how a contract reacts when a watched table changes.
type ContractPolicy string

const (
	// PolicyBlocking fails CI (non-zero exit) when a watched table changes.
	PolicyBlocking ContractPolicy = "blocking"
	// PolicyNotify warns but does not affect the exit code.
	PolicyNotify ContractPolicy = "notify"
)

// ContractsFile is a parsed schema-contracts.yaml.
type ContractsFile struct {
	Contracts []Contract `yaml:"contracts"`
}

// Contract declares a downstream consumer and the schema objects it depends on.
type Contract struct {
	Name         string       `yaml:"name"`
	Owner        string       `yaml:"owner"`
	Dependencies []Dependency `yaml:"dependencies"`
}

// Dependency is a single database (or datastore) dependency within a contract.
type Dependency struct {
	Database string   `yaml:"database"`
	Tables   []string `yaml:"tables"`
	// Datastore/Entities cover NoSQL datastores; SQL schema management ignores
	// them, but they are parsed so a combined contracts file stays valid.
	Datastore string         `yaml:"datastore"`
	Entities  []string       `yaml:"entities"`
	Policy    ContractPolicy `yaml:"policy"`
}

// ContractImpact records that a change to a database's tables touched a contract.
type ContractImpact struct {
	Contract string
	Owner    string
	Policy   ContractPolicy
	Database string
	Tables   []string
}

// FindContractsFile returns the path to a schema-contracts.yaml in dir, and
// whether it exists.
func FindContractsFile(dir string) (string, bool) {
	path := filepath.Join(dir, ContractsFileName)
	if _, err := os.Stat(path); err == nil {
		return path, true
	}
	return "", false
}

// LoadContractsFile reads and parses a schema-contracts.yaml.
func LoadContractsFile(path string) (*ContractsFile, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening contracts file %s: %w", path, err)
	}
	defer f.Close()

	var cf ContractsFile
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(&cf); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", ContractsFileName, err)
	}
	return &cf, nil
}

// ValidateContracts checks a contracts file against the blueprint's SQL database
// resources and their schemas: every SQL dependency must name a known database
// and only tables that exist in it, and every dependency must declare a valid
// policy. It returns a list of human-readable problems (empty when valid).
// Datastore (NoSQL) dependencies are only checked for a valid policy, since NoSQL
// schema management is not yet implemented.
func ValidateContracts(cf *ContractsFile, schemas map[string]*Schema) []string {
	var problems []string
	for _, c := range cf.Contracts {
		if c.Name == "" {
			problems = append(problems, "a contract is missing a name")
		}
		for _, dep := range c.Dependencies {
			problems = append(problems, validateDependency(c.Name, dep, schemas)...)
		}
	}
	return problems
}

func validateDependency(contractName string, dep Dependency, schemas map[string]*Schema) []string {
	label := fmt.Sprintf("contract %q", contractName)

	switch dep.Policy {
	case PolicyBlocking, PolicyNotify:
	case "":
		return []string{fmt.Sprintf("%s: a dependency is missing a policy (blocking or notify)", label)}
	default:
		return []string{fmt.Sprintf("%s: invalid policy %q (must be blocking or notify)", label, dep.Policy)}
	}

	// A NoSQL datastore dependency has nothing further to check here.
	if dep.Database == "" && dep.Datastore != "" {
		return nil
	}
	if dep.Database == "" {
		return []string{fmt.Sprintf("%s: a dependency has neither a database nor a datastore", label)}
	}

	sch, ok := schemas[dep.Database]
	if !ok {
		return []string{fmt.Sprintf(
			"%s: depends on database %q, which is not a celerity/sqlDatabase resource in the blueprint",
			label, dep.Database)}
	}
	var problems []string
	for _, table := range dep.Tables {
		if _, ok := sch.Tables[table]; !ok {
			problems = append(problems, fmt.Sprintf(
				"%s: depends on table %q in database %q, which does not exist in its schema",
				label, table, dep.Database))
		}
	}
	return problems
}

// EvaluateContracts returns the contracts affected by a set of changed tables,
// keyed by database resource name. A contract is affected when any table it
// watches on a database appears in that database's changed set.
func EvaluateContracts(cf *ContractsFile, changed map[string][]string) []ContractImpact {
	if cf == nil {
		return nil
	}
	var impacts []ContractImpact
	for _, c := range cf.Contracts {
		for _, dep := range c.Dependencies {
			if dep.Database == "" {
				continue
			}
			changedSet := changed[dep.Database]
			if len(changedSet) == 0 {
				continue
			}
			var matched []string
			for _, table := range dep.Tables {
				if slices.Contains(changedSet, table) {
					matched = append(matched, table)
				}
			}
			if len(matched) == 0 {
				continue
			}
			sort.Strings(matched)
			impacts = append(impacts, ContractImpact{
				Contract: c.Name,
				Owner:    c.Owner,
				Policy:   dep.Policy,
				Database: dep.Database,
				Tables:   matched,
			})
		}
	}
	return impacts
}

// RenderContractImpacts writes a "Contracts affected" section for the given
// impacts. Blocking impacts are marked ⛔, notify impacts ⚠. It writes nothing
// when there are no impacts.
func RenderContractImpacts(w io.Writer, impacts []ContractImpact) {
	if len(impacts) == 0 {
		return
	}
	fmt.Fprintln(w, "\n  Contracts affected:")
	for _, i := range impacts {
		marker := "⚠"
		if i.Policy == PolicyBlocking {
			marker = "⛔"
		}
		owner := ""
		if i.Owner != "" {
			owner = fmt.Sprintf(" (%s)", i.Owner)
		}
		fmt.Fprintf(w, "    %s %s%s is affected by a change to %s.%s [%s]\n",
			marker, i.Contract, owner, i.Database, strings.Join(i.Tables, ", "), i.Policy)
	}
}

// AnyBlocking reports whether any impact has a blocking policy.
func AnyBlocking(impacts []ContractImpact) bool {
	for _, i := range impacts {
		if i.Policy == PolicyBlocking {
			return true
		}
	}
	return false
}

// Returns the distinct tables added, dropped or modified by a set of Atlas schema
// changes, less the migration-history table, which no contract can watch.
func changedTables(changes []schema.Change) []string {
	seen := map[string]bool{}
	for _, c := range changes {
		var name string
		switch ch := c.(type) {
		case *schema.AddTable:
			name = tableName(ch.T)
		case *schema.DropTable:
			name = tableName(ch.T)
		case *schema.ModifyTable:
			name = tableName(ch.T)
		default:
			continue
		}
		if name != "" && name != revisionsTable {
			seen[name] = true
		}
	}
	tables := make([]string, 0, len(seen))
	for name := range seen {
		tables = append(tables, name)
	}
	sort.Strings(tables)
	return tables
}
