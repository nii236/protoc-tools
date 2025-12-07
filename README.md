# protoc-gen-godot

Protocol Buffer compiler plugins for generating both GDScript client code for Godot games and Go stream management utilities for Connect-RPC services.

## Features

### GDScript Client Generator (`protoc-gen-godot`)
- Generates GDScript clients for gRPC/Connect-RPC services in Godot
- Supports both unary and streaming RPC calls
- Automatic JSON serialization/deserialization
- Idempotent method detection for GET requests
- Built-in error handling and response signals

### Go Stream Manager Generator (`protoc-gen-streammanager`)
- Generates type-safe stream managers for Connect-RPC server-side streaming
- Thread-safe stream lifecycle management with session tracking
- Filtered broadcasting to multiple streams based on metadata
- Built-in implementations: full, no-op (testing), and debug variants
- Automatic cleanup on context cancellation

## Installation

1. Build the plugin:

```bash
go build -o protoc-gen-godot ./protoc-gen-godot
```

2. Configure `buf.gen.yaml` to use the binary path (not the `cmd` directory):

```yaml
version: v2
plugins:
  - local: ["./protoc-gen-godot"] # Use binary path, not ./cmd
    out: gen
    strategy: all
managed:
  enabled: true
```

3. Generate the code:

```bash
buf generate
```

## Generated Files

The plugin generates:

- `gen/connectrpc_client.gd` - Base client class with HTTP handling
- `gen/[package]/[service].gd` - Service-specific client classes

## Usage in Godot

### Option 1: Autoload Singleton

1. Add your service script as an autoload in Project Settings
2. Access globally: `MyServiceClient.method_name()`

### Option 2: Scene Node

1. Add a Node to your scene
2. Attach the generated service script
3. Call methods directly on the node

### Option 3: Programmatic

```gdscript
func _ready():
    var client = preload("res://gen/v1/myservice/myservice.gd").new()
    client.set_base_url("https://api.example.com")
    add_child(client)

    # Connect to response signals
    client.my_method_response.connect(_on_response)

    # Make RPC call
    client.my_method("parameter")

func _on_response(response: Dictionary):
    print("Response: ", response)
```

## Configuration

Set the server URL in your Godot script:

```gdscript
client.set_base_url("https://your-api.com")
```

## Requirements

- Go 1.19+
- Buf CLI
- Godot 4.0+
