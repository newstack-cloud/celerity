# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.5.0] - 2025-04-14

### Added

- Adds `ListLinkTypes` method to provider interface to allow tools and applications (e.g. Deploy Engine) that use providers to list the link types implemented by the provider.

## [0.4.1] - 2025-04-14

### Fixed

- Adds missing required field to the `function.ValueTypeDefinitionObject` type that defines an object type used as a function parameter or return type.

## [0.4.0] - 2025-04-14

### Added

- **Breaking change** - Add method to export all provider config variables from link contexts.
- Adds helper functions to create `core.ScalarValue` structs from Go primitive types.
- Adds zap logger adaptor for the `core.Logger` interface. This allows for the use of zap logger implementations with the blueprint framework core logger interface.
- **Breaking change** - Adds example method for resources, data sources and custom variable types.
- Adds functionality to allows link implementations to call back into the blueprint framework host to deploy resources registered with the host.
- Adds stabilisation check method to resource registry to allow the resource registry to fulfil a new `ResourceDeployService` interface introduced to allow link plugins to call back into the host to deploy known resources for intermediary resources managed by a link implementation.
- Adds a convenience error interface that allows the extraction of a list of failure reasons without having to check each possible provider error type.
- Adds a new provider.BadInput error type that can be used to help distinguish between input and unexpected errors when providing feedback to the user. _Bad input errors are not handled in any special way in the blueprint container that manages deployments, it must be wrapped in specialised errors to be handled correctly for deployment actions._
- Adds new public util functions that provide a consistent entry point for generating logical link names and link type identifiers.

### Updated

- Adds a new `Secret` field to the plugin config definitions struct. This allows plugin developers to indicate that a config variable is a secret and should be treated as such by the blueprint framework. This is useful for sensitive information such as passwords or API keys.
- Make plugin function call stack thread-safe to be able to handle calls from multiple goroutines.
- Enhances plugin interfaces with methods to provide rich documentation.
- Simplifies resource registry interface when it comes to waiting for resource stability. This is possible through the introduction of a convenience layer to the resource registry interface and core implementation to allow callers to wait for a resource to stabilise using a single boolean option instead of having to set up a polling loop and call `HasStabilised` on an interval to do so.
- **Breaking change** - Adds update to make creating a new plugin function call context a part of the public interface.
- **Breaking change** - Alters the public interface of existing provider errors to make the error check functions more idiomatic where you write to a target error and check the error type in the same action.
- Updates some previously private protobuf conversion methods to be public so that gRPC services (such as the deploy engine plugin system) can reuse existing conversion behaviour.
- Enhances provider-specific error types with child errors so they can hold more structured information.

### Removed

- Removes the `SetCurrentLocation` from the function call context as it is never used, each context gets its own call context with a current location that does not need to change for the lifetime of the function being called.

### Fixed

- Corrects the name of the transformer context field in the `transform.AbstractResourceGetExamplesInput` struct.
- Corrects behaviour to convert from protobuf to blueprint framework types so that a mapping node type with all fields set to nil is treated correctly when the mapping node being converted is optional.

_Breaking changes will occur in early 0.x releases of this framework._

## [0.3.3] - 2025-02-27

### Fixed

- Adds missing `CurrentResourceSpec` and `CurrentResourceMetadata` fields to the `ResourceGetExternalStateInput` struct that is passed into the `GetExternalState` method of the `provider.Resource` interface. This allows the provider to access the current resource spec and metadata when determining how to locate and present the "live" state of the resource in the external system.

## [0.3.2] - 2025-02-26

### Updated

- **Breaking change** - Renamed the `StabilisedDependencies` method of the `provider.Resource` interface to `GetStabilisedDependencies` to be consistent with the names of other methods to retrieve information about a resource.

_Breaking changes will occur in early 0.x releases of this framework._

## [0.3.1] - 2025-02-25

### Fixed

- Corrects error code used in the helper function to create export not found errors. This also adds a missing switch case for the display text to show for the export not found error code.

## [0.3.0] - 2025-02-25

### Added

- Helper functions to create and check for export not found errors to be used as a part of the state container interface. Implementations of the state container interface should use these functions to ensure that consistent error types are returned when a requested export is not present in a given blueprint instance.

## [0.2.3] - 2025-02-14

### Updated

- **Breaking change** - Renamed the `Remove` method of the children state container to `Detach` to be consistent with the `Attach` method and to highlight that the method does not completely remove the child blueprint state but removes the connection between the parent and child blueprints.

_Breaking changes will occur in early 0.x releases of this framework._

## [0.2.2] - 2025-02-09

### Fixed

- Adds a workaround to ensure Go does not try to zip contents of test data and snapshot directories when importing the package into a project. This workaround includes adding an empty `go.mod` file to every directory that should be ignored. Without this fix, the package could not be imported into packages due to unusual characters in the snapshot file names generated by the cupaloy package used for snapshot testing. It is also good practise to ignore these directories as they are not required for projects to make use of the package.

## [0.2.1] - 2025-02-04

### Updated

- **Breaking change** - Simplifies redundant entity type prefixed field names in state structures. (e.g. `ResourceName` -> `Name`) The exception to this change involves the id fields due to the `ID` name being used for the method that fulfils the Element interface. For this reason ResourceID, InstanceID and LinkID will remain in the Go structs but will be serialised to "id" when marshalling to JSON.'

_Breaking changes will occur in early 0.x releases of this framework._

## [0.2.0] - 2025-01-31

### Updated

- **Breaking change** - Removes redundant instance ID arguments from state container methods when interacting with resources and links by globally unique identifiers.
- Updates the in-memory state container implementation used in automated tests to store resource and link data in a way that it can be directly accessed when given a globally unique identifier.

_Breaking changes will occur in early 0.x releases of this framework._

## [0.1.0] - 2025-01-29

### Added

- Functionality to load, parse and validate blueprints that adheres to the [Blueprint Specification](https://www.celerityframework.com/docs/blueprint/specification).
- Functionality to stage changes for updates and new deployments of a blueprint.
- Functionality to resolve substitutions in all elements of a blueprint.
- Functionality to deploy and destroy blueprint instances based on a set of changes produced during change staging.
- An interface and data types for persisting blueprint state.
- Interfaces for interacting with resource providers that applications can build plugin systems on top of.
- A set of core functions that can be used with substitutions along with tools for creating custom functions through a provider plugin.
- Functionality to check whether the "live" external state of a resource or set of resources in a blueprint matches the current state persisted with the blueprint framework.
