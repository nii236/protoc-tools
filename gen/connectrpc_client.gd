# ConnectRPC Client Base Class
# Generic ConnectRPC client that handles protocol details
# Service-specific clients extend this class
# Auto-generated - DO NOT EDIT

extends Node

signal error_occurred(error: String)

var base_url: String = "http://localhost:8080"

func _ready():
	pass

# Generic unary call with GET (for idempotent methods)
func call_unary_get(service_path: String, method: String, request_data: Dictionary, response_signal: String) -> void:
	var url = base_url + "/" + service_path + "/" + method
	var query_url = url + "?encoding=json&message=" + JSON.stringify(request_data).uri_encode()
	
	var http_request = HTTPRequest.new()
	add_child(http_request)
	http_request.request_completed.connect(func(result: int, response_code: int, headers: PackedStringArray, body: PackedByteArray): _on_unary_response(response_signal, result, response_code, headers, body))
	
	var headers_array = ["Accept: application/json"]
	var error = http_request.request(query_url, headers_array, HTTPClient.METHOD_GET)
	
	if error != OK:
		emit_signal("error_occurred", "Failed to make GET request: " + str(error))
		http_request.queue_free()

# Generic unary call with POST
func call_unary_post(service_path: String, method: String, request_data: Dictionary, response_signal: String) -> void:
	var url = base_url + "/" + service_path + "/" + method
	var json_body = JSON.stringify(request_data)
	
	var http_request = HTTPRequest.new()
	add_child(http_request)
	http_request.request_completed.connect(func(result: int, response_code: int, headers: PackedStringArray, body: PackedByteArray): _on_unary_response(response_signal, result, response_code, headers, body))
	
	var headers_array = [
		"Content-Type: application/json",
		"Accept: application/json"
	]
	
	var error = http_request.request(url, headers_array, HTTPClient.METHOD_POST, json_body)
	
	if error != OK:
		emit_signal("error_occurred", "Failed to make POST request: " + str(error))
		http_request.queue_free()

# Generic streaming call
func call_streaming(service_path: String, method: String, request_data: Dictionary, event_signal: String) -> void:
	var url = base_url + "/" + service_path + "/" + method
	var json_body = JSON.stringify(request_data)
	
	# Create ConnectRPC envelope format for streaming request
	var enveloped_body = _create_connectrpc_envelope(json_body)
	
	var http_request = HTTPRequest.new()
	add_child(http_request)
	http_request.request_completed.connect(func(result: int, response_code: int, headers: PackedStringArray, body: PackedByteArray): _on_streaming_response(event_signal, result, response_code, headers, body))
	
	var headers_array = [
		"Content-Type: application/connect+json",
		"Accept: application/connect+json",
		"Connect-Protocol-Version: 1",
		"Cache-Control: no-cache"
	]
	
	var error = http_request.request_raw(url, headers_array, HTTPClient.METHOD_POST, enveloped_body)
	
	if error != OK:
		emit_signal("error_occurred", "Failed to start stream: " + str(error))
		http_request.queue_free()

# Handle unary responses
func _on_unary_response(response_signal: String, _result: int, response_code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	var http_request = get_children().filter(func(child): return child is HTTPRequest).back()
	if http_request:
		http_request.queue_free()
	
	if response_code == 200:
		var json = JSON.new()
		var parse_result = json.parse(body.get_string_from_utf8())
		if parse_result == OK:
			emit_signal(response_signal, json.data)
		else:
			emit_signal("error_occurred", "Failed to parse response JSON")
	else:
		emit_signal("error_occurred", "Request failed with code: " + str(response_code))

# Handle streaming responses
func _on_streaming_response(event_signal: String, _result: int, response_code: int, _headers: PackedStringArray, body: PackedByteArray) -> void:
	var http_request = get_children().filter(func(child): return child is HTTPRequest).back()
	if http_request:
		http_request.queue_free()
	
	if response_code == 200:
		_parse_connectrpc_stream(body, event_signal)
	else:
		emit_signal("error_occurred", "Stream request failed with code: " + str(response_code))

# Create ConnectRPC envelope format for request body
func _create_connectrpc_envelope(json_message: String) -> PackedByteArray:
	var message_bytes = json_message.to_utf8_buffer()
	var message_length = message_bytes.size()
	
	var envelope = PackedByteArray()
	
	# Envelope flags (1 byte): 0 = no compression, not final
	envelope.append(0)
	
	# Message length (4 bytes, big-endian)
	envelope.append((message_length >> 24) & 0xFF)
	envelope.append((message_length >> 16) & 0xFF)
	envelope.append((message_length >> 8) & 0xFF)
	envelope.append(message_length & 0xFF)
	
	# Message payload
	envelope.append_array(message_bytes)
	
	return envelope

# Parse ConnectRPC streaming format (binary enveloped messages)
func _parse_connectrpc_stream(bytes: PackedByteArray, event_signal: String) -> void:
	var offset = 0
	
	while offset < bytes.size():
		# Need at least 5 bytes for envelope header (1 flag + 4 length)
		if offset + 5 > bytes.size():
			break
			
		# Read envelope flags (1 byte)
		var flags = bytes[offset]
		offset += 1
		
		# Read message length (4 bytes, big-endian)
		var length = (bytes[offset] << 24) | (bytes[offset + 1] << 16) | (bytes[offset + 2] << 8) | bytes[offset + 3]
		offset += 4
		
		# Check if we have enough bytes for the message
		if offset + length > bytes.size():
			emit_signal("error_occurred", "Incomplete ConnectRPC message: expected " + str(length) + " bytes, got " + str(bytes.size() - offset))
			break
			
		# Extract message bytes
		var message_bytes = bytes.slice(offset, offset + length)
		offset += length
		
		# Convert to string and parse JSON
		var message_str = message_bytes.get_string_from_utf8()
		
		var json = JSON.new()
		var parse_result = json.parse(message_str)
		
		if parse_result == OK:
			# Check if this is the final message (flags & 2 == 2)
			var is_final = (flags & 2) == 2
			
			# Skip EndStreamResponse messages (they contain error info or are empty)
			if is_final and (json.data.has("error") or json.data == {}):
				break
			
			# Emit the event signal with the parsed data
			emit_signal(event_signal, json.data)
		else:
			emit_signal("error_occurred", "Failed to parse ConnectRPC message JSON: " + message_str)
		
		# If this was the final message, we're done
		if (flags & 2) == 2:
			break

# Utility methods
func set_base_url(url: String) -> void:
	base_url = url

func get_base_url() -> String:
	return base_url

