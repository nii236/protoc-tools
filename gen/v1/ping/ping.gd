# Generated from ping.proto
# ConnectRPC client for HelloWorldService service
# Auto-generated - DO NOT EDIT

extends "../../connectrpc_client.gd"
signal ping_response(response: Dictionary)

# Ping - Unary RPC
func ping(timestamp: int) -> void:
	var request_data = {}
	request_data["timestamp"] = timestamp
	call_unary_post("ping.v1.HelloWorldService", "Ping", request_data, "ping_response")
