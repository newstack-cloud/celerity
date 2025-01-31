# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

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
