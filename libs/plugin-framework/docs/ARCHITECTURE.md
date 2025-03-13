# Architecture

The plugin framework provides a gRPC inter-process plugin system built on top of the `provider.Provider` and `transform.SpecTransformer` interfaces to allow for plugins that can be dynamically loaded at runtime without the security risks of loading arbitrary code into the deploy engine process.
