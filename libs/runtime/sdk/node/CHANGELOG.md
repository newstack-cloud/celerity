# Changelog

## [0.4.2](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.4.1...runtime-sdk-node/v0.4.2) (2026-08-27)


### Dependencies

* **runtime-sdk-node:** update core dependencies ([52c1500](https://github.com/newstack-cloud/celerity/commit/52c15002ee49c472ef9cf89c391f533a07d594a9))

## [0.4.1](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.4.0...runtime-sdk-node/v0.4.1) (2026-08-27)


### docs

* **lib-rt-sdk-node:** note the toolchain pin for local cross builds ([9661a74](https://github.com/newstack-cloud/celerity/commit/9661a74fdc7e572f284244a14696702ebf67e18f))

## [0.4.0](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.5...runtime-sdk-node/v0.4.0) (2026-08-27)


### Features

* **lib-rt-core:** serve the handler stream over a unix socket ([d7511c8](https://github.com/newstack-cloud/celerity/commit/d7511c896cec4ab1d2f2ba0c6aff53ffbfec9f70))
* **lib-rt-sdk-node:** give handlers a way to frame a binary message ([986c7b6](https://github.com/newstack-cloud/celerity/commit/986c7b695973b945ae960dbcec4045ac11cb0fa9))
* **lib-rt-sdk-node:** read the cluster settings and build with clustering ([5dbdfac](https://github.com/newstack-cloud/celerity/commit/5dbdfaca9d98be88665b4f31e8402922ba80ce3a))
* **runtime-libs:** carry websocket sends and handler invocation on the stream ([984d0a2](https://github.com/newstack-cloud/celerity/commit/984d0a22bb8f6cc0372d39855f2df36b1249150a))
* **runtime-libs:** remove the local runtime API ([3525a33](https://github.com/newstack-cloud/celerity/commit/3525a3337f7ebfe8448d3b26d9bec2156cf72c4f))
* **runtime-libs:** shed, cancel and drain events on the handler stream ([15f8a9c](https://github.com/newstack-cloud/celerity/commit/15f8a9cb362d8888b398bc26cab0ced025a82391))


### Bug Fixes

* **lib-rt-sdk-node:** carry the acknowledgement timings through to the runtime ([e64841c](https://github.com/newstack-cloud/celerity/commit/e64841c65f0f1337e3fdffaed235d8ec149c405b))
* **lib-rt-sdk-node:** carry the handler concurrency through to the runtime ([0ee775e](https://github.com/newstack-cloud/celerity/commit/0ee775e0133f0862b2a481fa2f0bb898021926b9))
* **runtime-libs:** refuse an empty name as a node's identity ([8320c96](https://github.com/newstack-cloud/celerity/commit/8320c96d8f78355bb6c9f963156d30f23a9c6d42))
* **runtime-libs:** restrict unix socket permissions and make tcp fallback opt-in ([bf9362e](https://github.com/newstack-cloud/celerity/commit/bf9362e4ffe277b7c6cd67b1f0661270aa3f1ea1))


### Dependencies

* **runtime-sdk-node:** update core dependencies ([ba18f74](https://github.com/newstack-cloud/celerity/commit/ba18f741bf8911e9f84f65e7490835709ddf86e8))
* **runtime-sdk-node:** update core dependencies ([6a369d9](https://github.com/newstack-cloud/celerity/commit/6a369d9c04ac353f2d6702c0b80425757eff891a))

## [0.3.5](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.4...runtime-sdk-node/v0.3.5) (2026-06-05)


### Dependencies

* **runtime-sdk-node:** update core dependencies ([2fed7ad](https://github.com/newstack-cloud/celerity/commit/2fed7adfcc34a892d0c87c177d64e108f159de98))

## [0.3.4](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.3...runtime-sdk-node/v0.3.4) (2026-04-01)


### Dependencies

* **runtime-sdk-node:** update core dependencies ([03df80c](https://github.com/newstack-cloud/celerity/commit/03df80c095ec63697bbe8c258d6d475142dfa58a))

## [0.3.3](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.2...runtime-sdk-node/v0.3.3) (2026-03-18)


### Dependencies

* **lib-rt-sdk-node:** upgrade node sdk bindings to target node 24 ([94836f1](https://github.com/newstack-cloud/celerity/commit/94836f13a44c01111ad8666304512336ec8cebdc))

## [0.3.2](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.1...runtime-sdk-node/v0.3.2) (2026-03-16)


### Bug Fixes

* **lib-rt-sdk-node:** add missing events config and consumer message fields ([c2d4ec4](https://github.com/newstack-cloud/celerity/commit/c2d4ec48bed61ed98728f517cfb6455f053da3c9))


### Dependencies

* **runtime-sdk-node:** update core dependencies ([55b2c23](https://github.com/newstack-cloud/celerity/commit/55b2c23b184054edade20ec3e19dd7e9ed340575))

## [0.3.1](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.3.0...runtime-sdk-node/v0.3.1) (2026-02-25)


### Dependencies

* **runtime-sdk-node:** update core dependencies ([e0e5ec2](https://github.com/newstack-cloud/celerity/commit/e0e5ec2f1b9c1e57bff273048427f9267f2ea441))
* **runtime-sdk-node:** update core dependencies ([0122e5c](https://github.com/newstack-cloud/celerity/commit/0122e5c4656260e048becec071166fe4235c2757))

## [0.3.0](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.2.3...runtime-sdk-node/v0.3.0) (2026-02-25)


### Features

* **lib-rt-sdk-node:** add support for guards ([96c09b1](https://github.com/newstack-cloud/celerity/commit/96c09b1b143982da374276fc502031be15585e36))
* **lib-rt-sdk-node:** complete core runtime bindings for nodejs ([37c1e43](https://github.com/newstack-cloud/celerity/commit/37c1e43a1a5d80dc62b7e5a9f1c98778a4827d67))


### Dependencies

* **runtime-sdk-node:** update core dependencies ([c5bc93d](https://github.com/newstack-cloud/celerity/commit/c5bc93d3672458ab7ec9b6e4d1fd4fc059f19846))
* **runtime-sdk-node:** update core dependencies ([88d1f9c](https://github.com/newstack-cloud/celerity/commit/88d1f9cf23a5ec0e9ad8e987830d8277d758ea3b))
* **runtime-sdk-node:** update core dependencies ([8318570](https://github.com/newstack-cloud/celerity/commit/83185708be263d3c55bde273af15941be77aabe2))

## [0.2.3](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.2.2...runtime-sdk-node/v0.2.3) (2026-02-18)


### Dependencies

* **runtime-sdk-node:** update core dependencies ([9836166](https://github.com/newstack-cloud/celerity/commit/983616638b04bbb91c208c036671d22f069ad4c6))

## [0.2.2](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.2.1...runtime-sdk-node/v0.2.2) (2026-02-09)


### Bug Fixes

* **lib-rt-sdk-node:** add missing implementation for runtime config ([9abcb0a](https://github.com/newstack-cloud/celerity/commit/9abcb0a78c160d104c51b20d3036b19fffd930bc))

## [0.2.1](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.2.0...runtime-sdk-node/v0.2.1) (2026-02-08)


### Bug Fixes

* **lib-rt-sdk-node:** correct auth context naming ([04dab48](https://github.com/newstack-cloud/celerity/commit/04dab48a49a79256cae97bc2e35783cda0c5e33f))


### Dependencies

* **lib-rt-sdk-node:** move dotenvx to dev dependencies ([d9d6314](https://github.com/newstack-cloud/celerity/commit/d9d6314ea2ff9299727b4549f877f5372b4ac1e5))

## [0.2.0](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-node/v0.1.0...runtime-sdk-node/v0.2.0) (2026-02-08)


### Features

* **lib-rt-sdk-node:** add full support for http features ([6536d24](https://github.com/newstack-cloud/celerity/commit/6536d2495ba614178b661578a9f1f0fd15e899e2))

## 1.0.0 (2026-02-08)


### Features

* **lib-rt-sdk-node:** add full support for http features ([6536d24](https://github.com/newstack-cloud/celerity/commit/6536d2495ba614178b661578a9f1f0fd15e899e2))
