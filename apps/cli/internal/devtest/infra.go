package devtest

// InfraLevel describes the infrastructure needed for a set of test suites.
type InfraLevel int

const (
	// InfraLevelNone means no Docker infrastructure is needed (unit tests only).
	InfraLevelNone InfraLevel = iota

	// InfraLevelCompose starts compose dependencies (databases, caches, etc.)
	// but not the app container. Used for integration tests.
	InfraLevelCompose

	// InfraLevelFull starts compose dependencies AND the app container.
	// Used for API tests.
	InfraLevelFull
)

// InfraLevelForSuites determines the highest infrastructure level needed
// for the given set of test suites.
func InfraLevelForSuites(suites []TestSuite) InfraLevel {
	level := InfraLevelNone
	for _, s := range suites {
		switch s {
		case SuiteAPI:
			return InfraLevelFull
		case SuiteIntegration:
			level = InfraLevelCompose
		}
	}
	return level
}
