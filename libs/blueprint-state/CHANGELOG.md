# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2025-04-26

### Added

- Adds the `postgres` state container implementation that implements the `state.Container` interface from the blueprint framework along with the `manage.Validation`, `manage.Changesets` and `manage.Events` interfaces that are designed to be used by host applications such as the deploy engine.
- Adds a set of database migrations for the postgres state container implementation to be used with database migration tools or integrated into an installer for a host application.
- Adds the `memfile` state container implementation that implements the `state.Container` interface from the blueprint framework along with the `manage.Validation`, `manage.Changesets` and `manage.Events` interfaces that are designed to be used by host applications such as the deploy engine. This implementation uses an in-memory store for retrievals and persists writes to files on disk.
