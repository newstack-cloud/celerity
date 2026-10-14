package build

// NewDefaultRegistry creates a registry pre-populated with the standard
// build strategies and per-runtime components.
// appDir is used to detect the Node.js package manager from the project.
func NewDefaultRegistry(appDir string) *Registry {
	return NewRegistryWithExecutor(&ExecCommandExecutor{}, appDir)
}

// NewRegistryWithExecutor creates a registry using the provided command
// executor. This allows tests to inject a stub executor.
func NewRegistryWithExecutor(exec CommandExecutor, appDir string) *Registry {
	r := NewRegistry()

	r.RegisterStrategy("aws-serverless", &LambdaStrategy{
		Registry: r,
		Bootstraps: map[string]HandlerBootstrap{
			"nodejs": &LambdaNodeBootstrap{AppDir: appDir},
			"python": &LambdaPythonBootstrap{},
		},
	})

	nodePM := DetectNodePackageManager(appDir)
	r.RegisterDependencyInstaller("nodejs", &NodeDependencyInstaller{Exec: exec, PM: nodePM})
	r.RegisterDependencyInstaller("python", &PythonDependencyInstaller{Exec: exec})

	r.RegisterSourceCompiler("nodejs", &NodeSourceCompiler{Exec: exec, PM: nodePM})

	r.RegisterHandlerDependencyReader("nodejs", &NodeHandlerDependencyReader{})
	r.RegisterHandlerDependencyReader("python", &PythonHandlerDependencyReader{})

	return r
}
