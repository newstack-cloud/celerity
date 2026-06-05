# Changelog

## [0.7.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.6.0...runtime-core/v0.7.0) (2026-06-05)


### Features

* **lib-rt-blueprint-lang:** add blueprint language implementation ([96715a4](https://github.com/newstack-cloud/celerity/commit/96715a4903e02c684e2d12cdac8f44521555ec85))
* **lib-rt-blueprint-parser:** integrate blueprint lang into config parser ([432b9a1](https://github.com/newstack-cloud/celerity/commit/432b9a1a211493e6b261270d702c8a6a2b4f4207))
* **lib-rt-core:** integrate blueprint language support ([df1ea3d](https://github.com/newstack-cloud/celerity/commit/df1ea3d0057b8aed9a123e09c2d4cbae7be94c14))


### Bug Fixes

* **lib-rt-helpers:** correct match statement to avoid panic ([6b8f944](https://github.com/newstack-cloud/celerity/commit/6b8f94459ce8720ec700f04278ab2c08153cb431))
* **lib-rt-helpers:** correct scanner to use match instead of if ([6ae6056](https://github.com/newstack-cloud/celerity/commit/6ae60563f9ea90af44c0843184e5b6306ed815f3))
* **lib-rt-workflow:** correct attempt number extraction to use match instead of if ([04ac1f1](https://github.com/newstack-cloud/celerity/commit/04ac1f12381eb9a8b5aa90255b5d10d6eca1dff1))
* **lib-rt-workflow:** correct mapping node variant for none ([f6f21ec](https://github.com/newstack-cloud/celerity/commit/f6f21ec1afaed11384db0393ee214f795bf7311e))
* **lib-rt-workflow:** correct match statement to avoid panic ([88a5554](https://github.com/newstack-cloud/celerity/commit/88a555498fd2d83c6231d3a1e0501ecac04d4261))

## [0.6.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.5.0...runtime-core/v0.6.0) (2026-03-23)


### Features

* **lib-rt-core:** add capability message to websocket server ([43e8bb6](https://github.com/newstack-cloud/celerity/commit/43e8bb6540c2752d57c185d3da58a082e66f43fa))

## [0.5.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.4.1...runtime-core/v0.5.0) (2026-03-16)


### Features

* **lib-rt-core:** integrate consumer message body transformation ([c35bdfd](https://github.com/newstack-cloud/celerity/commit/c35bdfdc1af523b08640c96d6592d69cbbb459c0))


### Bug Fixes

* **lib-rt-blueprint-parser:** add missing resource types to yaml parser ([1a51394](https://github.com/newstack-cloud/celerity/commit/1a51394c6ac152b5a17d3d297844dec6495980cb))
* **lib-rt-core:** add fixes for ws auth message strategy and cors ([e57706a](https://github.com/newstack-cloud/celerity/commit/e57706a06c081c3faae74b34d3100a0064bc9f08))

## [0.4.1](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.4.0...runtime-core/v0.4.1) (2026-02-25)


### Bug Fixes

* **lib-rt-core:** add fix to use redis connection per worker ([a7c97ef](https://github.com/newstack-cloud/celerity/commit/a7c97efb7c11da06b3bd099e5f48e186c01a8b92))

## [0.4.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.3.1...runtime-core/v0.4.0) (2026-02-25)


### Features

* **lib-rt-blueprint-parser:** add support for static input for a schedule ([e2476c7](https://github.com/newstack-cloud/celerity/commit/e2476c72df99c7a8c6a90413556dabf5b88582ba))
* **lib-rt-core:** add missing instrumentation and optional metrics ([7456e6d](https://github.com/newstack-cloud/celerity/commit/7456e6d5a2960f2a6ffaf2bab5ab4f18475840ab))
* **lib-rt-core:** complete foundations for v0 implementation ([0a0d70d](https://github.com/newstack-cloud/celerity/commit/0a0d70d078a26810da178768980a924e75c9b588))


### Bug Fixes

* **lib-rt-blueprint-parser:** add missing auth scheme and discovery mode fields ([5ef50dc](https://github.com/newstack-cloud/celerity/commit/5ef50dc05aa4392466afa771f550fa1bacca394b))
* **lib-rt-core:** add fixes for redis consumers and ws auth strategy ([354b678](https://github.com/newstack-cloud/celerity/commit/354b67833833de3d2c30435dce8612682d658c96))

## [0.3.1](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.3.0...runtime-core/v0.3.1) (2026-02-18)


### Bug Fixes

* **lib-rt-blueprint-parser:** correct anotation parsing to be string-based ([cf421cd](https://github.com/newstack-cloud/celerity/commit/cf421cd26e97e7580e8e96ac8f953aa1fde7afe1))
* **lib-rt-core:** update config transformation to handle annotations as strings ([2956cfb](https://github.com/newstack-cloud/celerity/commit/2956cfbe6a7edb5adc52847368230df739070849))

## [0.3.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.2.1...runtime-core/v0.3.0) (2026-02-08)


### Features

* **lib-rt-core:** add support for chaining multiple guards for auth and other checks ([6bd5e98](https://github.com/newstack-cloud/celerity/commit/6bd5e98553e52b792d8ac3c053a96b750ee3714f))

## [0.2.1](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.2.0...runtime-core/v0.2.1) (2026-02-08)


### Dependencies

* **runtime-libs:** replace webpki-roots with native-roots to solve license issues ([8f08ddf](https://github.com/newstack-cloud/celerity/commit/8f08ddf3bc737595d31b4ab5ac909f8c5e8d61ad))
* **runtime-libs:** update reqwest to use rustls instead of native openssl ([26fd2cb](https://github.com/newstack-cloud/celerity/commit/26fd2cbfad22366122dfed386dbf8bd3d63447dc))

## [0.2.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.1.0...runtime-core/v0.2.0) (2026-02-08)


### Features

* **lib-rt-core:** add missing features for production-ready http applications ([3b98f89](https://github.com/newstack-cloud/celerity/commit/3b98f8902b4183ea8eba7641dff011edb99f7a06))


### Dependencies

* **runtime-libs:** update vulnerable dependencies to patched versions ([51a7c63](https://github.com/newstack-cloud/celerity/commit/51a7c63fec2218f26a1a6abe9f2f6cc5c5af9e10))
