# SigilBridge Example Plugin

Build with:

```sh
go build ./examples/plugin-example
```

Run directly for a local smoke test:

```sh
./plugin-example -listen 127.0.0.1:9099
```

Install the binary and `plugin.yaml` into a SigilBridge plugin directory. The example serves the provider gRPC contract, returns canned responses, supports streaming, and is intended for host/supervisor testing.
