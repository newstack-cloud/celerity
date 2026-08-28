# Changelog

## [0.3.1](https://github.com/newstack-cloud/celerity/compare/runtime-consumers/v0.3.0...runtime-consumers/v0.3.1) (2026-08-28)


### Bug Fixes

* **lib-rt-consumer-redis:** advance the offset once the outcome is recorded ([fa1fa1b](https://github.com/newstack-cloud/celerity/commit/fa1fa1b441e74df0bf5d8c971d59386ee58d45c2))
* **lib-rt-consumer-sqs:** keep a shed message where failures are deleted ([6e666e6](https://github.com/newstack-cloud/celerity/commit/6e666e6d97d4cbf325c01292a6ea3d84d0def0b0))
* **lib-rt-consumer-sqs:** quieten the large error lint on the sdk's errors ([ae8ec0b](https://github.com/newstack-cloud/celerity/commit/ae8ec0bea60390182eb884447743c7aa531a47fa))
* **lib-rt-consumer-sqs:** stop deleting messages a handler could not process ([c5232b4](https://github.com/newstack-cloud/celerity/commit/c5232b4d476760ee188bc8b774f1e96f262cf975))
* **runtime-libs:** stop bringing a shed message straight back on sqs ([56f3df0](https://github.com/newstack-cloud/celerity/commit/56f3df074e7361466fb3debbfebddcf1286ff283))

## [0.3.0](https://github.com/newstack-cloud/celerity/compare/runtime-consumers/v0.2.1...runtime-consumers/v0.3.0) (2026-03-16)


### Features

* **lib-rt-consumer-redis:** add additional fields to redis consumer messages ([fa9215f](https://github.com/newstack-cloud/celerity/commit/fa9215f6cfdd2192fe918645fc3389e1593e1697))

## [0.2.1](https://github.com/newstack-cloud/celerity/compare/runtime-consumers/v0.2.0...runtime-consumers/v0.2.1) (2026-02-25)


### Bug Fixes

* **lib-rt-core:** add fix to use redis connection per worker ([a7c97ef](https://github.com/newstack-cloud/celerity/commit/a7c97efb7c11da06b3bd099e5f48e186c01a8b92))

## [0.2.0](https://github.com/newstack-cloud/celerity/compare/runtime-consumers/v0.1.0...runtime-consumers/v0.2.0) (2026-02-25)


### Features

* **lib-rt-consumer-redis:** add telemetry to redis consumer ([a555523](https://github.com/newstack-cloud/celerity/commit/a555523e12af4be88db5e4485bf7081a75c3afdd))
* **lib-rt-consumer-sqs:** add metrics to sqs consumer ([da572ff](https://github.com/newstack-cloud/celerity/commit/da572ff85e70ec6147a1426b97bd9bcd0474561e))
