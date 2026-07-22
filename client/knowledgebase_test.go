package client

import (
	"encoding/json"
	"testing"
)

func TestUpdateKnowledgeBaseRequestEncodesVLMConfig(t *testing.T) {
	body, err := json.Marshal(UpdateKnowledgeBaseRequest{
		Name: "kb",
		VLMConfig: &VLMConfig{
			Enabled: true,
			ModelID: "vlm-model",
		},
	})
	if err != nil {
		t.Fatalf("marshal update request: %v", err)
	}

	var payload map[string]json.RawMessage
	if err := json.Unmarshal(body, &payload); err != nil {
		t.Fatalf("unmarshal encoded request: %v", err)
	}
	if _, ok := payload["vlm_config"]; !ok {
		t.Fatalf("expected vlm_config in update request JSON, got %s", body)
	}
}
