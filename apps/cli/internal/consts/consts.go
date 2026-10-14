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
	//
	// This is a scaffolding language rather than a runtime prefix. Go handlers
	// run on an OS-only runtime, so no runtime identifier starts with "go" and
	// this must never be registered as a key in a runtime-prefix map.
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

const (
	// RuntimeOSOnly is the prefix shared by every OS-only runtime identifier.
	//
	// An OS-only runtime carries no language interpreter. The handlers
	// executable is compiled ahead of time and the core runtime starts it as a
	// separate process, so the identifier names the base OS rather than a
	// language. Which language built the executable is resolved from the
	// project, not from this string.
	RuntimeOSOnly = "os-only"

	// RuntimeOSOnlyDebian13 is the OS-only runtime built on Debian 13 (trixie).
	RuntimeOSOnlyDebian13 = "os-only.debian13"
)

const (
	BlueprintSourceFile      = "file"
	BlueprintSourceS3        = "s3"
	BlueprintSourceGCS       = "gcs"
	BlueprintSourceAzureBlob = "azureblob"
	BlueprintSourceHTTPS     = "https"
)
