package main

import (
	"log"
	"path/filepath"
	"strings"
	"text/template"

	"google.golang.org/protobuf/compiler/protogen"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/pluginpb"
)

func main() {
	protogen.Options{}.Run(func(gen *protogen.Plugin) error {
		gen.SupportedFeatures = uint64(pluginpb.CodeGeneratorResponse_FEATURE_PROTO3_OPTIONAL)

		for _, f := range gen.Files {
			if !f.Generate {
				continue
			}

			generateFile(gen, f)
		}
		return nil
	})
}

func generateFile(gen *protogen.Plugin, file *protogen.File) {
	if len(file.Services) == 0 {
		return
	}

	// Generate base class in the same package as the service
	generateConnectRPCBaseInPackage(gen, file.GoImportPath)

	filename := file.GeneratedFilenamePrefix + ".gd"
	g := gen.NewGeneratedFile(filename, file.GoImportPath)

	generateGDScript(g, file)
}

func generateGDScript(g *protogen.GeneratedFile, file *protogen.File) {

	// Generate the GDScript client with helper functions
	tmpl := template.Must(template.New("gdscript").Funcs(template.FuncMap{
		"snake_case":    toSnakeCase,
		"default_value": getDefaultValue,
	}).Parse(gdscriptTemplate))

	data := TemplateData{
		FileName:    filepath.Base(string(file.Desc.Path())),
		PackageName: string(file.GoPackageName),
		Services:    make([]ServiceData, 0, len(file.Services)),
	}

	for _, service := range file.Services {
		// Create full service path like "helloworld.v1.HelloWorldService"
		servicePath := string(file.Desc.Package()) + "." + string(service.Desc.Name())

		serviceData := ServiceData{
			Name:        string(service.GoName),
			ServicePath: servicePath,
			Methods:     make([]MethodData, 0, len(service.Methods)),
		}

		for _, method := range service.Methods {
			methodData := MethodData{
				Name:           string(method.GoName),
				InputType:      string(method.Input.GoIdent.GoName),
				OutputType:     string(method.Output.GoIdent.GoName),
				IsStreaming:    method.Desc.IsStreamingServer(),
				IsIdempotent:   isIdempotent(method),
				RequestFields:  extractFields(method.Input),
				ResponseFields: extractFields(method.Output),
			}
			serviceData.Methods = append(serviceData.Methods, methodData)
		}

		data.Services = append(data.Services, serviceData)
	}

	if err := tmpl.Execute(g, data); err != nil {
		log.Fatalf("Failed to execute template: %v", err)
	}
}

func isIdempotent(method *protogen.Method) bool {
	// Check for idempotency_level = NO_SIDE_EFFECTS in the method options
	options := method.Desc.Options()
	if options == nil {
		return false
	}

	// Cast to descriptor options to access idempotency_level
	if methodOptions, ok := options.(*descriptorpb.MethodOptions); ok && methodOptions != nil {
		if methodOptions.IdempotencyLevel != nil {
			return *methodOptions.IdempotencyLevel == descriptorpb.MethodOptions_NO_SIDE_EFFECTS
		}
	}

	return false
}

func extractFields(message *protogen.Message) []FieldData {
	fields := make([]FieldData, 0, len(message.Fields))

	for _, field := range message.Fields {
		fieldData := FieldData{
			Name:       string(field.GoName),
			JsonName:   field.Desc.JSONName(),
			Type:       getGDScriptType(field),
			IsOptional: field.Desc.HasOptionalKeyword(),
			IsRepeated: field.Desc.IsList(),
		}
		fields = append(fields, fieldData)
	}

	return fields
}

func getGDScriptType(field *protogen.Field) string {
	switch field.Desc.Kind().String() {
	case "string":
		return "String"
	case "int32", "int64", "uint32", "uint64":
		return "int"
	case "float", "double":
		return "float"
	case "bool":
		return "bool"
	case "message":
		return "Dictionary"
	default:
		return "Variant"
	}
}

func generateConnectRPCBaseInPackage(gen *protogen.Plugin, goImportPath protogen.GoImportPath) {
	// Generate the ConnectRPC base class in the package directory
	g := gen.NewGeneratedFile("connectrpc_client.gd", goImportPath)
	g.P(connectRPCClientCode)
}

// Helper functions for template
func toSnakeCase(s string) string {
	var result strings.Builder
	for i, r := range s {
		if i > 0 && 'A' <= r && r <= 'Z' {
			result.WriteRune('_')
		}
		if 'A' <= r && r <= 'Z' {
			result.WriteRune(r - 'A' + 'a')
		} else {
			result.WriteRune(r)
		}
	}
	return result.String()
}

func getDefaultValue(gdType string) string {
	switch gdType {
	case "String":
		return "\"\""
	case "int":
		return "0"
	case "float":
		return "0.0"
	case "bool":
		return "false"
	case "Dictionary":
		return "{}"
	default:
		return "null"
	}
}

type TemplateData struct {
	FileName    string
	PackageName string
	Services    []ServiceData
}

type ServiceData struct {
	Name        string
	ServicePath string
	Methods     []MethodData
}

type MethodData struct {
	Name           string
	InputType      string
	OutputType     string
	IsStreaming    bool
	IsIdempotent   bool
	RequestFields  []FieldData
	ResponseFields []FieldData
}

type FieldData struct {
	Name       string
	JsonName   string
	Type       string
	IsOptional bool
	IsRepeated bool
}

const gdscriptTemplate = `# Generated from {{ .FileName }}
# ConnectRPC client for {{ range .Services }}{{ .Name }}{{ end }} service
# Auto-generated - DO NOT EDIT

extends "../connectrpc_client.gd"

{{- range .Services }}
{{- range .Methods }}
{{- if .IsStreaming }}
signal {{ snake_case .Name }}_event(event: Dictionary)
{{- else }}
signal {{ snake_case .Name }}_response(response: Dictionary)
{{- end }}
{{- end }}
{{- end }}

{{- range $service := .Services }}
{{- range .Methods }}

{{ if .IsStreaming -}}
# {{ .Name }} - Streaming RPC
func {{ snake_case .Name }}({{ range $i, $field := .RequestFields }}{{ if $i }}, {{ end }}{{ snake_case $field.Name }}: {{ $field.Type }}{{ if $field.IsOptional }} = {{ default_value $field.Type }}{{ end }}{{ end }}) -> void:
	var request_data = {}
{{- range .RequestFields }}
{{- if .IsOptional }}
	if {{ snake_case .Name }} != {{ default_value .Type }}:
		request_data["{{ .JsonName }}"] = {{ snake_case .Name }}
{{- else }}
	request_data["{{ .JsonName }}"] = {{ snake_case .Name }}
{{- end }}
{{- end }}
	call_streaming("{{ $service.ServicePath }}", "{{ .Name }}", request_data, "{{ snake_case .Name }}_event")
{{- else -}}
# {{ .Name }} - Unary RPC{{ if .IsIdempotent }} (idempotent - supports GET){{ end }}
func {{ snake_case .Name }}({{ range $i, $field := .RequestFields }}{{ if $i }}, {{ end }}{{ snake_case $field.Name }}: {{ $field.Type }}{{ if $field.IsOptional }} = {{ default_value $field.Type }}{{ end }}{{ end }}) -> void:
	var request_data = {}
{{- range .RequestFields }}
{{- if .IsOptional }}
	if {{ snake_case .Name }} != {{ default_value .Type }}:
		request_data["{{ .JsonName }}"] = {{ snake_case .Name }}
{{- else }}
	request_data["{{ .JsonName }}"] = {{ snake_case .Name }}
{{- end }}
{{- end }}
{{- if .IsIdempotent }}
	call_unary_get("{{ $service.ServicePath }}", "{{ .Name }}", request_data, "{{ snake_case .Name }}_response")
{{- else }}
	call_unary_post("{{ $service.ServicePath }}", "{{ .Name }}", request_data, "{{ snake_case .Name }}_response")
{{- end }}
{{- end }}
{{- end }}
{{- end }}
`

const connectRPCClientCode = `# ConnectRPC Client Base Class
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
`
