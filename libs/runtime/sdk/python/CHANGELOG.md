# Changelog

## [0.3.0](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-python/v0.2.2...runtime-sdk-python/v0.3.0) (2026-08-27)


### Features

* **lib-rt-core:** serve the handler stream over a unix socket ([d7511c8](https://github.com/newstack-cloud/celerity/commit/d7511c896cec4ab1d2f2ba0c6aff53ffbfec9f70))
* **lib-rt-sdk-python:** give handlers a way to frame a binary message ([366b966](https://github.com/newstack-cloud/celerity/commit/366b966395007f741cb5e110b2bb6a59be0c9781))
* **lib-rt-sdk-python:** read the cluster settings and build with clustering ([098523a](https://github.com/newstack-cloud/celerity/commit/098523a7fbd198d0fade15eab18456a1211bae89))
* **runtime-libs:** carry websocket sends and handler invocation on the stream ([984d0a2](https://github.com/newstack-cloud/celerity/commit/984d0a22bb8f6cc0372d39855f2df36b1249150a))
* **runtime-libs:** remove the local runtime API ([3525a33](https://github.com/newstack-cloud/celerity/commit/3525a3337f7ebfe8448d3b26d9bec2156cf72c4f))
* **runtime-libs:** shed, cancel and drain events on the handler stream ([15f8a9c](https://github.com/newstack-cloud/celerity/commit/15f8a9cb362d8888b398bc26cab0ced025a82391))


### Bug Fixes

* **lib-rt-sdk-python:** carry the acknowledgement timings through to the runtime ([e6078ef](https://github.com/newstack-cloud/celerity/commit/e6078ef3943470130666e9fc3a68157c5daf93ab))
* **lib-rt-sdk-python:** carry the handler concurrency through to the runtime ([eeaa91b](https://github.com/newstack-cloud/celerity/commit/eeaa91b49589c4fcaafe14dcbbd712089c4fa22b))
* **runtime-libs:** refuse an empty name as a node's identity ([8320c96](https://github.com/newstack-cloud/celerity/commit/8320c96d8f78355bb6c9f963156d30f23a9c6d42))
* **runtime-libs:** restrict unix socket permissions and make tcp fallback opt-in ([bf9362e](https://github.com/newstack-cloud/celerity/commit/bf9362e4ffe277b7c6cd67b1f0661270aa3f1ea1))


### Dependencies

* **runtime-sdk-python:** update core dependencies ([b61504e](https://github.com/newstack-cloud/celerity/commit/b61504e3564acb1deb66262c798c556388a79243))
* **runtime-sdk-python:** update core dependencies ([92cbae9](https://github.com/newstack-cloud/celerity/commit/92cbae91abea4590486356edda8add31e8912f1a))
* **runtime-sdk-python:** update core dependencies ([ba7af12](https://github.com/newstack-cloud/celerity/commit/ba7af1266676b7b9214c131f1fc7601538b473ba))

## [0.2.2](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-python/v0.2.1...runtime-sdk-python/v0.2.2) (2026-06-05)


### Dependencies

* **runtime-sdk-python:** update core dependencies ([c630bd4](https://github.com/newstack-cloud/celerity/commit/c630bd46b44ebb3adc84c98b37388ae9ee50d9e1))

## [0.2.1](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-python/v0.2.0...runtime-sdk-python/v0.2.1) (2026-03-23)


### Bug Fixes

* **lib-rt-sdk-python:** add fixes to websocket types and include all features in build ([39cee06](https://github.com/newstack-cloud/celerity/commit/39cee06918f6bbdd1f020d1a33908db32205ca0a))


### Dependencies

* **runtime-sdk-python:** update core dependencies ([d2be0ed](https://github.com/newstack-cloud/celerity/commit/d2be0edddef9e16d6ec348eb41df5ab8d328501e))

## [0.2.0](https://github.com/newstack-cloud/celerity/compare/runtime-sdk-python/v0.1.0...runtime-sdk-python/v0.2.0) (2026-03-17)


### Features

* **lib-rt-sdk-python:** add custom handler invocation method, missing types and fields ([39a4c7a](https://github.com/newstack-cloud/celerity/commit/39a4c7a48e732f84b11aafc71dba90d11d2075ad))
* **lib-rt-sdk-python:** complete core runtime bindings for python ([e499bda](https://github.com/newstack-cloud/celerity/commit/e499bda47e47f82b91226f417ba621f6e0e9a600))


### Bug Fixes

* **lib-rt-sdk-python:** add missing events config and consumer message fields ([99d3f0f](https://github.com/newstack-cloud/celerity/commit/99d3f0f48d72b0322c34d9b8881e95811cf37db1))
* **lib-rt-sdk-python:** correct auth context naming ([b3bfad3](https://github.com/newstack-cloud/celerity/commit/b3bfad31035f0681165f2da081bdb8bd8a50c711))


### Dependencies

* **runtime-sdk-python:** update core dependencies ([b2a8ac2](https://github.com/newstack-cloud/celerity/commit/b2a8ac2a02fc5ec025502dc8092a333e94f848bc))
* **runtime-sdk-python:** update core dependencies ([aa1302d](https://github.com/newstack-cloud/celerity/commit/aa1302dec7a231301e1751f039012e0f872643a6))
* **runtime-sdk-python:** update core dependencies ([0e0c729](https://github.com/newstack-cloud/celerity/commit/0e0c72939b61fd3d2220895482bdf30506c21cf2))
* **runtime-sdk-python:** update core dependencies ([9afdc3e](https://github.com/newstack-cloud/celerity/commit/9afdc3e008513fb363129150bbf4bfb7f26c220b))
* **runtime-sdk-python:** update core dependencies ([1ce7d30](https://github.com/newstack-cloud/celerity/commit/1ce7d30936a6531c2b39d2394d15e07d83947adf))
* **runtime-sdk-python:** update core dependencies ([40efe0f](https://github.com/newstack-cloud/celerity/commit/40efe0f3fc07b0f6f1f630dc9ba3b866f4ccce05))
* **runtime-sdk-python:** update core dependencies ([d25f487](https://github.com/newstack-cloud/celerity/commit/d25f487c7a97ac9ad273de03863b70c34d2d131a))
* **runtime-sdk-python:** update core dependencies ([5cd1f36](https://github.com/newstack-cloud/celerity/commit/5cd1f3602aff5e3bc94387d67fbb8256a3144a75))
