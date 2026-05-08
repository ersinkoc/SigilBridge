# Plugin Development Guide

## Overview

SigilBridge plugins are external provider adapters. A plugin is a directory containing a `plugin.yaml` manifest and an executable that speaks the SigilBridge provider gRPC contract.

Use a plugin when an adapter is provider-specific, experimental, private to your organization, or too large to belong in the core binary.

## Directory Layout

```text
my-provider/
  plugin.yaml
  my-provider-plugin
```

## Manifest

```yaml
id: my_provider
display_name: My Provider
version: 0.1.0
executable: ./my-provider-plugin
protocol: grpc
capabilities:
  streaming: true
  tool_use: true
  vision: false
```

The `id` is the value used as an upstream `provider` in `pools.yaml`.

## Pool Configuration

```yaml
pools:
  - name: custom
    strategy: priority_first
    upstreams:
      - id: my-provider-primary
        provider: my_provider
        priority: 1
        weight: 1
        config:
          api_key: ${MY_PROVIDER_API_KEY}
          model: custom-chat-latest
```

## gRPC Contract

The canonical contract lives in `pkg/proto/adapter.proto`, with generated Go stubs committed under `pkg/proto`.

A provider plugin should implement:

- Capability discovery.
- Non-streaming completion dispatch.
- Streaming completion dispatch.
- Health or probe behavior if supported.

Requests and responses use SigilBridge's internal representation so plugins do not need to parse both OpenAI and Anthropic ingress formats.

## Reference Plugin

The repository includes `examples/plugin-example`, which returns canned responses and is useful for validating host discovery:

```bash
go build ./examples/plugin-example
```

Install the binary and manifest into your plugin directory, then add an upstream with the plugin provider ID.

## Operational Behavior

- Plugins are discovered from the configured plugin root.
- SigilBridge starts plugin processes and supervises them.
- Crashed plugins restart with exponential backoff.
- Plugin health is reflected in routing decisions.
- Plugin logs should go to stderr so the supervisor can capture diagnostics.

## Development Checklist

- Validate all required manifest fields.
- Keep startup fast; expensive provider probes should be lazy or bounded.
- Return classified provider errors so router fallback works.
- Support context cancellation for streaming requests.
- Avoid writing secrets to logs.
- Include a small mock or in-process test server.

## Compatibility

Plugins should declare their expected SigilBridge protocol version in release notes. If the protobuf contract changes, rebuild plugins against the new `pkg/proto` package and run an end-to-end dispatch test before deploying.
