# Generated from helloworld.proto
# ConnectRPC client for HelloWorldService service
# Auto-generated - DO NOT EDIT

extends "../../connectrpc_client.gd"
signal say_hello_response(response: Dictionary)
signal stream_hellos_event(event: Dictionary)
signal stream_counter_event(event: Dictionary)

# SayHello - Unary RPC (idempotent - supports GET)
func say_hello(name: String, message: String = "") -> void:
	var request_data = {}
	request_data["name"] = name
	if message != "":
		request_data["message"] = message
	call_unary_get("helloworld.v1.HelloWorldService", "SayHello", request_data, "say_hello_response")

# StreamHellos - Streaming RPC
func stream_hellos(name: String, count: int = 0, interval_seconds: int = 0) -> void:
	var request_data = {}
	request_data["name"] = name
	if count != 0:
		request_data["count"] = count
	if interval_seconds != 0:
		request_data["intervalSeconds"] = interval_seconds
	call_streaming("helloworld.v1.HelloWorldService", "StreamHellos", request_data, "stream_hellos_event")

# StreamCounter - Streaming RPC
func stream_counter(start_value: int, increment: int, max_value: int = 0, interval_ms: int = 0) -> void:
	var request_data = {}
	request_data["startValue"] = start_value
	request_data["increment"] = increment
	if max_value != 0:
		request_data["maxValue"] = max_value
	if interval_ms != 0:
		request_data["intervalMs"] = interval_ms
	call_streaming("helloworld.v1.HelloWorldService", "StreamCounter", request_data, "stream_counter_event")
