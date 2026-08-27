# Changelog

## [0.8.1](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.8.0...runtime-core/v0.8.1) (2026-08-27)


### Bug Fixes

* **lib-rt-core:** serve the handler stream over loopback where there is no unix socket ([b4efe7f](https://github.com/newstack-cloud/celerity/commit/b4efe7f76073153cd7bcb08b653d1b99e967bd7f))

## [0.8.0](https://github.com/newstack-cloud/celerity/compare/runtime-core/v0.7.0...runtime-core/v0.8.0) (2026-08-27)


### Features

* **lib-rt-core:** act once on a message a client sends more than once ([6fef75f](https://github.com/newstack-cloud/celerity/commit/6fef75ff42e127fb111b35413c68e91d381826f6))
* **lib-rt-core:** add the handler stream dispatcher ([c00a3d3](https://github.com/newstack-cloud/celerity/commit/c00a3d393a04ac3c7208ebbee9ff40ffb154cccc))
* **lib-rt-core:** add the IPC handler protocol contract ([14e4cb2](https://github.com/newstack-cloud/celerity/commit/14e4cb23315c3f626cdba6662856700de3bf26c9))
* **lib-rt-core:** add the IPC handler stream and handshake ([05742a2](https://github.com/newstack-cloud/celerity/commit/05742a24fb7db1f5409f203e60f21e7183f9d581))
* **lib-rt-core:** bound the event queue and enforce handler deadlines ([25d5334](https://github.com/newstack-cloud/celerity/commit/25d533470d183bfaf1fe20c77ef066fddac0895f))
* **lib-rt-core:** carry requests to handlers without losing anything ([1bf6337](https://github.com/newstack-cloud/celerity/commit/1bf633739ff9beb8b8b1abe352d92cb7b629fabd))
* **lib-rt-core:** convert between runtime events and protocol frames ([8aac01c](https://github.com/newstack-cloud/celerity/commit/8aac01c5509ff042297008900ae3e178f3d0aaf4))
* **lib-rt-core:** handle a connection's messages beside each other ([15cf09c](https://github.com/newstack-cloud/celerity/commit/15cf09c8557d02e1713e20d2a09331e62c8f96de))
* **lib-rt-core:** invoke a handler by the name the blueprint publishes it under ([81490a8](https://github.com/newstack-cloud/celerity/commit/81490a8030aa7139e1dc000e2f6017782a730f53))
* **lib-rt-core:** let a websocket api be deployed as more than one node ([c5c8764](https://github.com/newstack-cloud/celerity/commit/c5c8764302c3683126abe2adf97516e59d5aba84))
* **lib-rt-core:** make joining a cluster something a test can run ([7b82051](https://github.com/newstack-cloud/celerity/commit/7b820518e1750bf1fd4af6f41b595b61de263295))
* **lib-rt-core:** recognise a client acknowledging a message ([acba51a](https://github.com/newstack-cloud/celerity/commit/acba51aaedf70fc7c82af057d684f77936e84777))
* **lib-rt-core:** recognise a message a client sent to another node ([ce1e1c4](https://github.com/newstack-cloud/celerity/commit/ce1e1c4f98937e32caed74c341bf8cf3a2ad8c08))
* **lib-rt-core:** report handler attachment in the health check ([151cc50](https://github.com/newstack-cloud/celerity/commit/151cc50ef3b1fd06b3ed6a6c3a8b387cba863bb2))
* **lib-rt-core:** route WebSocket messages in the IPC call mode ([6ed8eb8](https://github.com/newstack-cloud/celerity/commit/6ed8eb80dfd4349bda09cd4498159969cd47b0d2))
* **lib-rt-core:** serve the handler stream over a unix socket ([d7511c8](https://github.com/newstack-cloud/celerity/commit/d7511c896cec4ab1d2f2ba0c6aff53ffbfec9f70))
* **lib-rt-core:** wait for a client to acknowledge what it was sent ([897ca39](https://github.com/newstack-cloud/celerity/commit/897ca39c0dfd057ebc1cb0a6ca994ac9420bd0e0))
* **lib-rt-core:** wire HTTP handlers in the IPC call mode ([4f4d3f7](https://github.com/newstack-cloud/celerity/commit/4f4d3f7af71c6c068eea2f210d8dc2d9e1967d10))
* **lib-rt-helpers:** frame binary messages rather than hoping they arrive ([a900672](https://github.com/newstack-cloud/celerity/commit/a900672d7b8bdfe1db4381f55f852b0f03bde181))
* **lib-rt-helpers:** give the crates that build on this one somewhere to find redis ([04e0fa4](https://github.com/newstack-cloud/celerity/commit/04e0fa43d5f3a68ee9e6245273493931e59c95d3))
* **lib-rt-helpers:** read sets and keys, and give up a subscription ([f57eb78](https://github.com/newstack-cloud/celerity/commit/f57eb788e1b6c9801c2414c9771bcf05886368a3))
* **lib-rt-sdk-node:** give handlers a way to frame a binary message ([986c7b6](https://github.com/newstack-cloud/celerity/commit/986c7b695973b945ae960dbcec4045ac11cb0fa9))
* **lib-rt-ws-redis:** put nodes in groups and route between them ([702b217](https://github.com/newstack-cloud/celerity/commit/702b217c647afbca12891ccb540822d30b5254ce))
* **runtime-core:** ship the image able to cluster ([80da97d](https://github.com/newstack-cloud/celerity/commit/80da97df173780046fc1de58ce9a155ca5021f8f))
* **runtime-libs:** carry websocket sends and handler invocation on the stream ([984d0a2](https://github.com/newstack-cloud/celerity/commit/984d0a22bb8f6cc0372d39855f2df36b1249150a))
* **runtime-libs:** let a handler ask for a client acknowledgement over ipc ([cc2bf0d](https://github.com/newstack-cloud/celerity/commit/cc2bf0d0ec1b78bfdb7c80b5bf83cea0f5b5ffe1))
* **runtime-libs:** negotiate the protocol version at the handshake ([242e2b0](https://github.com/newstack-cloud/celerity/commit/242e2b0902cf6b2130e84b60c3855c1c86e90234))
* **runtime-libs:** remove the local runtime API ([3525a33](https://github.com/newstack-cloud/celerity/commit/3525a3337f7ebfe8448d3b26d9bec2156cf72c4f))
* **runtime-libs:** settle a message held by a node that has gone ([cdfa766](https://github.com/newstack-cloud/celerity/commit/cdfa766d8f2759c51324fc8dcdc4377306e43ef8))
* **runtime-libs:** shed, cancel and drain events on the handler stream ([15f8a9c](https://github.com/newstack-cloud/celerity/commit/15f8a9cb362d8888b398bc26cab0ced025a82391))


### Bug Fixes

* **lib-rt-core:** acknowledge a websocket message when it is taken in ([ef32423](https://github.com/newstack-cloud/celerity/commit/ef324232dce4cdd88d7babbefccae8e3e0d9a6db))
* **lib-rt-core:** act on what a consumer handler reported ([2c2427a](https://github.com/newstack-cloud/celerity/commit/2c2427af86ff7e0255d41bdd1f42cf71b43c82f3))
* **lib-rt-core:** add type fix to act as range check for ports ([56cb812](https://github.com/newstack-cloud/celerity/commit/56cb8121f4e58df5256eef2e65389f9cb40d1382))
* **lib-rt-core:** answer a client that has not authenticated yet ([8770eed](https://github.com/newstack-cloud/celerity/commit/8770eedd7f847f52b99aa6420060570caffc3961))
* **lib-rt-core:** bound the cancellation channel ([34fa4cc](https://github.com/newstack-cloud/celerity/commit/34fa4cc99a9c626e9affe5efd1993d5d435cb172))
* **lib-rt-core:** carry a custom handler's output without altering it ([2fbd185](https://github.com/newstack-cloud/celerity/commit/2fbd1859e7b5b58c3dd739fb9985a04c2b8361ba))
* **lib-rt-core:** carry HTTP request bodies without corrupting them ([6d83098](https://github.com/newstack-cloud/celerity/commit/6d83098d92e010b097b623b98423b47a6eb71930))
* **lib-rt-core:** close a websocket connection out without waiting on its queue ([efdec95](https://github.com/newstack-cloud/celerity/commit/efdec95122ac623399247990edac52e3f4dae0a4))
* **lib-rt-core:** close handler streams that never declare themselves ([bb9d07b](https://github.com/newstack-cloud/celerity/commit/bb9d07b271f997d2bc5f28a127d83dd070e5562f))
* **lib-rt-core:** count the default timeout when deriving how long to drain ([5f8d1d4](https://github.com/newstack-cloud/celerity/commit/5f8d1d426d1570444144306e45deabdf7f6c3503))
* **lib-rt-core:** enable missing net feature in tokio-stream ([36b9f39](https://github.com/newstack-cloud/celerity/commit/36b9f398b04c218d8537f70af0df313e1ad98d64))
* **lib-rt-core:** find a connection auth guard when a client connects ([cd40222](https://github.com/newstack-cloud/celerity/commit/cd402227c0cf9b8c81669b2907221bc5db609a54))
* **lib-rt-core:** keep a cluster's credentials out of what is logged ([68a5ef2](https://github.com/newstack-cloud/celerity/commit/68a5ef27562e3df88775124395645879b8379357))
* **lib-rt-core:** keep a stream that is behind rather than tearing it down ([455edeb](https://github.com/newstack-cloud/celerity/commit/455edeb201f21ea80ccb6740b8831ccc6e587904))
* **lib-rt-core:** keep the reservation window inside a u32 ([4974bfa](https://github.com/newstack-cloud/celerity/commit/4974bfa5314fba9eec32fa7d51b4131932bc9cf0))
* **lib-rt-core:** keep what went wrong inside a handler out of the response ([9557f38](https://github.com/newstack-cloud/celerity/commit/9557f384f1cae653a3718ca9ffa839e18275f621))
* **lib-rt-core:** leave a place for the tags that are holding nothing ([42b3187](https://github.com/newstack-cloud/celerity/commit/42b31879ada77694561be9db96320d6828175208))
* **lib-rt-core:** let a deployment set the acknowledgement timings ([73352f7](https://github.com/newstack-cloud/celerity/commit/73352f74d6682231fa4e82ceeee887cc9e02ef08))
* **lib-rt-core:** let a draining handler finish the work it is holding ([f7ab9ef](https://github.com/newstack-cloud/celerity/commit/f7ab9ef26a458b01f48fb5b72d7706898c55e435))
* **lib-rt-core:** parse a text message once and refuse a timeout of nothing ([7fd96c9](https://github.com/newstack-cloud/celerity/commit/7fd96c92a5fb651aabd2741205d5a61404e7cd78))
* **lib-rt-core:** read an acknowledgement written with escapes ([6474462](https://github.com/newstack-cloud/celerity/commit/6474462b782f0e16a0c87c269b5659561fe51a16))
* **lib-rt-core:** read what a websocket client actually sent ([2f09a20](https://github.com/newstack-cloud/celerity/commit/2f09a2065722a04b1fca388372b10d8f2345a97c))
* **lib-rt-core:** record the stack a handler reports for an unhandled error ([0bf691b](https://github.com/newstack-cloud/celerity/commit/0bf691b8ed28233ea6c2b59b8414b0147694287f))
* **lib-rt-core:** refuse a failure naming records that were not delivered ([9347637](https://github.com/newstack-cloud/celerity/commit/9347637eff8aeb3feec8783c80f5bbc3f86f6b19))
* **lib-rt-core:** refuse a response status that cannot be one ([87b1c2e](https://github.com/newstack-cloud/celerity/commit/87b1c2e49a25a8247a26644ea985435b53b0a653))
* **lib-rt-core:** release callers of events abandoned at the drain deadline ([eac0456](https://github.com/newstack-cloud/celerity/commit/eac0456cb42cb7a0b3262243765d1b18cc21258d))
* **lib-rt-core:** report a handler failure in the shape its caller waits for ([e04c230](https://github.com/newstack-cloud/celerity/commit/e04c23026c7c45fbd9bc7901cc62a313c764c967))
* **lib-rt-core:** report a lost dispatcher as not ready ([ea004cc](https://github.com/newstack-cloud/celerity/commit/ea004cc00fe430332340cc3a3955c3ac77d003a3))
* **lib-rt-core:** resolve timeouts for event handlers ([9ecdab7](https://github.com/newstack-cloud/celerity/commit/9ecdab7dca91e28be4b19af9d5884f33171084f9))
* **lib-rt-core:** restrict perms for directory created for unix socket ([ae220a7](https://github.com/newstack-cloud/celerity/commit/ae220a7fd0b69fe7d5442baba937e29ee425fb62))
* **lib-rt-core:** satisfy the protocol lint on the service name ([b6f195c](https://github.com/newstack-cloud/celerity/commit/b6f195c9ccf4381b83f50fa017ee7400b9f339c7))
* **lib-rt-core:** send a batch of websocket messages per connection ([e29dde4](https://github.com/newstack-cloud/celerity/commit/e29dde4f965ccfe0f79f6ecc5a33bd1222c9203c))
* **lib-rt-core:** settle on one name for a node and keep it ([a6f6cd1](https://github.com/newstack-cloud/celerity/commit/a6f6cd1637d7d914ccc8ae86fc07d5449fc64db1))
* **lib-rt-core:** start the event cleanup task in run rather than setup ([7fa7ce8](https://github.com/newstack-cloud/celerity/commit/7fa7ce8a16af64fb7679224905b097ce330f1dcc))
* **lib-rt-core:** stop a handler that never answers from stalling its stream ([8a45195](https://github.com/newstack-cloud/celerity/commit/8a45195effaef494132cc5b9b377499c87da94d2))
* **lib-rt-core:** stop a websocket connection waiting on a lock to read ([8c2393e](https://github.com/newstack-cloud/celerity/commit/8c2393e97cc054ef8270e379d46659646f34a6c8))
* **lib-rt-core:** stop a websocket handler blocking its connection ([9254608](https://github.com/newstack-cloud/celerity/commit/92546086176c194d340f787d5697c90f42507ea3))
* **lib-rt-core:** stop watching the deadline of a completed event ([e0299e7](https://github.com/newstack-cloud/celerity/commit/e0299e742ee5c0fe428e1ec4e0419c3f124f6cfd))
* **lib-rt-core:** take the auth response off the stack where it helps ([d6af1c8](https://github.com/newstack-cloud/celerity/commit/d6af1c8b962a9ff7a4054e35f15d38ed0d7b46d5))
* **lib-rt-core:** tell a client authenticated during the upgrade that it was ([edf254e](https://github.com/newstack-cloud/celerity/commit/edf254e1314dbcab4696f1c2d54038ecf923b22e))
* **lib-rt-core:** wait for a runtime before starting the worker that waits on clients ([f301606](https://github.com/newstack-cloud/celerity/commit/f301606f9d9252253a1dda83c93fb45513c5feb0))
* **lib-rt-helpers:** refuse a binary message whose route has no length ([93b89da](https://github.com/newstack-cloud/celerity/commit/93b89da72c41b063ff09c96d65073d05d8ae7a12))
* **lib-rt-workflow:** drop local API remnants re-added by the rename merge ([c08a9a2](https://github.com/newstack-cloud/celerity/commit/c08a9a23286cc1adad0b42a1370d7a90d9330962))
* **lib-rt-ws-registry:** take a message as settled only from the client it was sent to ([6714675](https://github.com/newstack-cloud/celerity/commit/671467512308bd2efdca3ec4809f75891c42a960))
* resolve the advisories the runtime dependencies carry ([4a43dd0](https://github.com/newstack-cloud/celerity/commit/4a43dd037c2c96249e392c333e9b29c3e652beae))
* **runtime-libs:** carry the trace context to a handlers executable ([ee1e03b](https://github.com/newstack-cloud/celerity/commit/ee1e03bfc45d7e70f26642939be0135e2f10063d))
* **runtime-libs:** handle an acknowledgement worker that has stopped ([0908438](https://github.com/newstack-cloud/celerity/commit/0908438267ec73c5102e3d3c488ef3e97b1c67ac))
* **runtime-libs:** make the test harness able to run one package ([0bd8746](https://github.com/newstack-cloud/celerity/commit/0bd8746c3bfd20427c8c43ac1a536beb1c90cd89))
* **runtime-libs:** refuse an empty name as a node's identity ([8320c96](https://github.com/newstack-cloud/celerity/commit/8320c96d8f78355bb6c9f963156d30f23a9c6d42))
* **runtime-libs:** release the registry when an application shuts down ([5288f06](https://github.com/newstack-cloud/celerity/commit/5288f061c0ebc4f23523559e136e5c1743164881))
* **runtime-libs:** restrict unix socket permissions and make tcp fallback opt-in ([bf9362e](https://github.com/newstack-cloud/celerity/commit/bf9362e4ffe277b7c6cd67b1f0661270aa3f1ea1))
* **runtime-libs:** stop bringing a shed message straight back on sqs ([56f3df0](https://github.com/newstack-cloud/celerity/commit/56f3df074e7361466fb3debbfebddcf1286ff283))

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
