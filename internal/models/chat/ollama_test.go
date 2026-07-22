package chat

import "testing"

func TestOllamaChatBuildRequestUsesDefaultReasoningForQwen3Next(t *testing.T) {
	c := &OllamaChat{modelName: "qwen3-next:8b"}
	disabled := false
	cases := []struct {
		name string
		opts *ChatOptions
	}{
		{name: "nil options", opts: nil},
		{name: "unset thinking", opts: &ChatOptions{}},
		{name: "disabled thinking", opts: &ChatOptions{Thinking: &disabled}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := c.buildChatRequest([]Message{{Role: "user", Content: "hello"}}, tc.opts, true)

			if req.Think == nil {
				t.Fatal("expected qwen3-next request to include think option")
			}
			if req.Think.Value != "medium" {
				t.Fatalf("qwen3-next think value = %#v, want %q", req.Think.Value, "medium")
			}
		})
	}
}
