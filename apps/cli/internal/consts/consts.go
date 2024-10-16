package consts

const (
	// LanguageNodeJS is the Node.js language constant
	// to be used for initialising and managing node.js projects.
	LanguageNodeJS = "nodejs"

	// LanguagePython is the Python language constant
	// to be used for initialising and managing python projects.
	LanguagePython = "python"

	// LanguageJava is the Java language constant
	// to be used for initialising and managing java projects.
	LanguageJava = "java"

	// LanguageDotNet is the .NET language constant
	// to be used for initialising and managing .NET projects.
	LanguageDotNet = "dotnet"

	// LanguageGo is the Go language constant
	// to be used for initialising and managing go projects.
	LanguageGo = "go"

	// LanguageRust is the Rust language constant
	// to be used for initialising and managing rust projects.
	LanguageRust = "rust"
)

var (
	// SupportedLanguages is a list of all supported languages
	// that can be used for Celerity projects.
	SupportedLanguages = []string{
		LanguageNodeJS,
		LanguagePython,
		LanguageJava,
		LanguageDotNet,
		LanguageGo,
		LanguageRust,
	}
)
