package proto

type ChatRequest struct {
	ProviderId  string `json:"provider_id,omitempty"`
	RequestJson []byte `json:"request_json,omitempty"`
	ConfigJson  []byte `json:"config_json,omitempty"`
}

type ChatResponse struct {
	ResponseJson []byte `json:"response_json,omitempty"`
}

type StreamEvent struct {
	EventJson []byte `json:"event_json,omitempty"`
}

type HealthRequest struct {
	ProviderId string `json:"provider_id,omitempty"`
	ConfigJson []byte `json:"config_json,omitempty"`
}

type HealthResponse struct {
	Ok      bool   `json:"ok,omitempty"`
	Message string `json:"message,omitempty"`
}
