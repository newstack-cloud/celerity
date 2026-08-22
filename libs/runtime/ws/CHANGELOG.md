# Changelog

## [0.2.0](https://github.com/newstack-cloud/celerity/compare/runtime-ws/v0.1.1...runtime-ws/v0.2.0) (2026-08-22)


### Features

* **lib-rt-core:** wait for a client to acknowledge what it was sent ([897ca39](https://github.com/newstack-cloud/celerity/commit/897ca39c0dfd057ebc1cb0a6ca994ac9420bd0e0))
* **lib-rt-helpers:** frame binary messages rather than hoping they arrive ([a900672](https://github.com/newstack-cloud/celerity/commit/a900672d7b8bdfe1db4381f55f852b0f03bde181))
* **lib-rt-ws-redis:** put nodes in groups and route between them ([702b217](https://github.com/newstack-cloud/celerity/commit/702b217c647afbca12891ccb540822d30b5254ce))


### Bug Fixes

* **lib-rt-core:** close a websocket connection out without waiting on its queue ([efdec95](https://github.com/newstack-cloud/celerity/commit/efdec95122ac623399247990edac52e3f4dae0a4))
* **lib-rt-core:** read what a websocket client actually sent ([2f09a20](https://github.com/newstack-cloud/celerity/commit/2f09a2065722a04b1fca388372b10d8f2345a97c))
* **lib-rt-core:** stop a websocket connection waiting on a lock to read ([8c2393e](https://github.com/newstack-cloud/celerity/commit/8c2393e97cc054ef8270e379d46659646f34a6c8))
* **lib-rt-helpers:** refuse a binary message whose route has no length ([93b89da](https://github.com/newstack-cloud/celerity/commit/93b89da72c41b063ff09c96d65073d05d8ae7a12))
* **lib-rt-ws-redis:** keep a connection whose entry would not write ([c5ff006](https://github.com/newstack-cloud/celerity/commit/c5ff00634b7d4cd5e4b5fc3f28b65f4298276cb3))
* **lib-rt-ws-redis:** keep the channels of a group a node came back to ([533f1ee](https://github.com/newstack-cloud/celerity/commit/533f1eeff303830835c96d61c0e8f1fba64a6cb7))
* **lib-rt-ws-redis:** keep trying to follow a node into its new group ([8475add](https://github.com/newstack-cloud/celerity/commit/8475add40916bad467682e6e88cf7c7bdc201cfb))
* **lib-rt-ws-redis:** leave a node group without a joiner losing it ([4970fa3](https://github.com/newstack-cloud/celerity/commit/4970fa3c292d225d584d5daf199affb8976e9e3d))
* **lib-rt-ws-registry:** count a resend to a client as an attempt ([15618c2](https://github.com/newstack-cloud/celerity/commit/15618c2346a87f2c5b59f6660d2a29069619ad86))
* **lib-rt-ws-registry:** hand out the client ack channel before the worker runs ([059df18](https://github.com/newstack-cloud/celerity/commit/059df1854c8b810cfd1268152d81b1ed8d7dc76b))
* **lib-rt-ws-registry:** note a message as waiting before it goes out ([dece678](https://github.com/newstack-cloud/celerity/commit/dece6784a3294f8d31f6da4b56469f00ebf0b375))
* **lib-rt-ws-registry:** recognise a message the cluster has already sent ([9ce582b](https://github.com/newstack-cloud/celerity/commit/9ce582bee009fa8cfb401ad71d0c2d947240a8dd))
* **lib-rt-ws-registry:** send a resend to the connection it was meant for ([f5db986](https://github.com/newstack-cloud/celerity/commit/f5db98651b3ab553fe88c322faf85c25f6f957e4))
* **lib-rt-ws-registry:** take a message as settled only from the client it was sent to ([6714675](https://github.com/newstack-cloud/celerity/commit/671467512308bd2efdca3ec4809f75891c42a960))
* **lib-rt-ws-registry:** tell the other clients when one of them has gone ([233333a](https://github.com/newstack-cloud/celerity/commit/233333a5a2c02015f5413e0a2d93214b3bd9b814))
* **lib-rt-ws-registry:** use the acknowledgement timings the protocol names ([6e8a9dc](https://github.com/newstack-cloud/celerity/commit/6e8a9dc4d66fa528677ae26aef9f5e073f91c0ef))
* **runtime-libs:** give back the record of a message that was not sent ([b9793ce](https://github.com/newstack-cloud/celerity/commit/b9793ce80b1283d194b3304bcc68d174647ebf57))

## [0.1.1](https://github.com/newstack-cloud/celerity/compare/runtime-ws/v0.1.0...runtime-ws/v0.1.1) (2026-02-25)


### Bug Fixes

* **lib-rt-ws-registry:** ensure binary messages are forwarded ([78bdae9](https://github.com/newstack-cloud/celerity/commit/78bdae96a654a50a9eeaf3fa6112cb07bfab7784))
