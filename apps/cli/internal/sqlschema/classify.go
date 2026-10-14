package sqlschema

import (
	"fmt"
	"slices"
	"strings"

	"ariga.io/atlas/sql/schema"
)

// RiskLevel classifies how safe it is to auto-apply a migration against a live
// database.
type RiskLevel string

const (
	// RiskSafe changes are additive and backward-compatible with running code
	// (add table/column-with-default, add non-unique index, drop index/constraint).
	RiskSafe RiskLevel = "safe"
	// RiskRisky changes are data-dependent and can fail or lock depending on
	// existing data (set NOT NULL, add unique index, add foreign key/check,
	// change column type, add NOT NULL column without a default).
	RiskRisky RiskLevel = "risky"
	// RiskBreaking changes are destructive/backward-incompatible with running
	// code (drop table/column/schema).
	RiskBreaking RiskLevel = "breaking"
)

// The token a migration file header records its risk under, as in
// "-- celerity:risk breaking  (drops column customers.legacy_tier)".
const riskMarker = "celerity:risk"

func riskRank(level RiskLevel) int {
	switch level {
	case RiskSafe:
		return 0
	case RiskRisky:
		return 1
	case RiskBreaking:
		return 2
	default:
		return 1 // unknown → treat as risky
	}
}

// Returns the highest risk across the changes, with a short reason naming the
// change that set it.
func classifyChanges(changes []schema.Change) (RiskLevel, string) {
	level, reason := RiskSafe, ""
	for _, c := range changes {
		if l, r := classifyChange(c); riskRank(l) > riskRank(level) {
			level, reason = l, r
		}
	}
	return level, reason
}

func classifyChange(change schema.Change) (RiskLevel, string) {
	switch c := change.(type) {
	case *schema.DropTable:
		return RiskBreaking, fmt.Sprintf("drops table %s", tableName(c.T))
	case *schema.DropSchema:
		return RiskBreaking, "drops a schema"
	case *schema.ModifyTable:
		level, reason := classifyChanges(c.Changes)
		if reason != "" {
			reason = fmt.Sprintf("table %s: %s", tableName(c.T), reason)
		}
		return level, reason
	case *schema.DropColumn:
		return RiskBreaking, fmt.Sprintf("drops column %s", columnName(c.C))
	case *schema.AddColumn:
		if c.C != nil && !columnNullable(c.C) && c.C.Default == nil {
			return RiskRisky, fmt.Sprintf("adds NOT NULL column %s without a default", columnName(c.C))
		}
		return RiskSafe, ""
	case *schema.ModifyColumn:
		if c.Change.Is(schema.ChangeNull) && columnNullable(c.From) && !columnNullable(c.To) {
			return RiskRisky, fmt.Sprintf("makes column %s NOT NULL", columnName(c.To))
		}
		if c.Change.Is(schema.ChangeType) {
			return RiskRisky, fmt.Sprintf("changes the type of column %s", columnName(c.To))
		}
		return RiskSafe, ""
	case *schema.AddIndex:
		if c.I != nil && c.I.Unique {
			return RiskRisky, "adds a unique index (may fail on duplicate data)"
		}
		return RiskSafe, ""
	case *schema.AddForeignKey:
		return RiskRisky, "adds a foreign key (existing rows may violate it)"
	case *schema.AddCheck:
		return RiskRisky, "adds a check constraint (existing rows may violate it)"
	default:
		// Add table/schema/index-drop/constraint-drop and anything else are
		// treated as backward-compatible.
		return RiskSafe, ""
	}
}

// A migration without a recognised header is treated as risky, so hand-written
// or legacy migrations are never auto-applied silently.
func migrationRisk(content []byte) RiskLevel {
	for raw := range strings.SplitSeq(string(content), "\n") {
		line := strings.TrimSpace(raw)
		_, after, ok := strings.Cut(line, riskMarker)
		if !ok {
			continue
		}
		fields := strings.Fields(after)
		if len(fields) == 0 {
			continue
		}
		switch level := RiskLevel(fields[0]); level {
		case RiskSafe, RiskRisky, RiskBreaking:
			return level
		}
	}
	return RiskRisky
}

// Combines a migration's declared header risk with the un-spoofable floor
// derived from its actual SQL statements. Data-destructive SQL wins over the
// header, so a "safe" header on a file that drops data is treated as breaking.
// detail is a non-empty parenthetical when the SQL floor raised the risk above
// the declared header.
func effectiveRisk(content []byte, statements []string) (RiskLevel, string) {
	risk := migrationRisk(content)
	if floor, reason := sqlRiskFloor(statements); riskRank(floor) > riskRank(risk) {
		return floor, fmt.Sprintf(" (its SQL %s, overriding the header)", reason)
	}
	return risk, ""
}

// Scans a migration's actual SQL statements for data-destructive operations and
// returns the minimum risk they imply, independent of any declared header. This
// is the un-spoofable safety floor, so a migration that drops data is treated as
// breaking even if its header claims otherwise, as a generated "safe" file
// hand-edited to add a DROP would. Only drops of tables, schemas, databases and
// columns count, since those are the operations that destroy row data; DROP
// INDEX, CONSTRAINT, DEFAULT and NOT NULL do not.
func sqlRiskFloor(statements []string) (RiskLevel, string) {
	for _, stmt := range statements {
		if reason := destructiveReason(stripSQLComments(stmt)); reason != "" {
			return RiskBreaking, reason
		}
	}
	return RiskSafe, ""
}

// Keywords following DROP that do not destroy row data.
var nonDestructiveDropTargets = map[string]bool{
	"CONSTRAINT": true, "DEFAULT": true, "NOT": true, "INDEX": true,
	"EXPRESSION": true, "IDENTITY": true, "PRIMARY": true, "UNIQUE": true,
	"CHECK": true, "FOREIGN": true, "KEY": true, "VIEW": true,
	"FUNCTION": true, "TRIGGER": true, "TYPE": true, "SEQUENCE": true,
}

func destructiveReason(stmt string) string {
	tokens := tokenizeSQL(stmt)
	for i, tok := range tokens {
		if tok != "DROP" {
			continue
		}
		switch target := dropTarget(tokens, i+1); target {
		case "TABLE", "SCHEMA", "DATABASE", "COLUMN":
			return "drops a " + strings.ToLower(target)
		default:
			if nonDestructiveDropTargets[target] {
				continue
			}
			// A bare `ALTER TABLE ... DROP <name>` (COLUMN keyword omitted) is a
			// column drop.
			if hasToken(tokens[:i], "ALTER") && hasToken(tokens[:i], "TABLE") {
				return "drops a column"
			}
		}
	}
	return ""
}

func dropTarget(tokens []string, from int) string {
	for j := from; j < len(tokens); j++ {
		if tokens[j] == "IF" || tokens[j] == "EXISTS" {
			continue
		}
		return tokens[j]
	}
	return ""
}

func hasToken(tokens []string, want string) bool {
	return slices.Contains(tokens, want)
}

func tokenizeSQL(stmt string) []string {
	fields := strings.Fields(stmt)
	tokens := make([]string, 0, len(fields))
	for _, f := range fields {
		if f = strings.Trim(f, "();,\"'`"); f != "" {
			tokens = append(tokens, strings.ToUpper(f))
		}
	}
	return tokens
}

// Removes -- line comments and /* */ block comments so keyword scanning does not
// match text inside a comment.
func stripSQLComments(stmt string) string {
	for {
		before, afterOpen, found := strings.Cut(stmt, "/*")
		if !found {
			break
		}
		_, afterClose, closed := strings.Cut(afterOpen, "*/")
		if !closed {
			stmt = before
			break
		}
		stmt = before + " " + afterClose
	}
	var b strings.Builder
	for line := range strings.SplitSeq(stmt, "\n") {
		if before, _, found := strings.Cut(line, "--"); found {
			line = before
		}
		b.WriteString(line)
		b.WriteByte(' ')
	}
	return b.String()
}

func riskHeader(level RiskLevel, reason string) string {
	if reason != "" {
		return fmt.Sprintf("-- %s %s  (%s)\n", riskMarker, level, reason)
	}
	return fmt.Sprintf("-- %s %s\n", riskMarker, level)
}

func columnNullable(c *schema.Column) bool {
	return c != nil && c.Type != nil && c.Type.Null
}

func columnName(c *schema.Column) string {
	if c == nil {
		return "?"
	}
	return c.Name
}

func tableName(t *schema.Table) string {
	if t == nil {
		return "?"
	}
	return t.Name
}
