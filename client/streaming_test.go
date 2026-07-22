package client

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
)

type sseRoundTripper struct {
	body string
}

func (rt sseRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(rt.body)),
		Request:    req,
	}, nil
}

const multilineStreamEvent = "data: {\"response_type\":\"answer\",\n" +
	"data: \"content\":\"hello\",\"done\":false}\n\n"

func TestProcessAgentSSEStreamMultilineDataFrame(t *testing.T) {
	c := NewClient("http://example.test")
	var got *AgentStreamResponse

	err := c.processAgentSSEStream(strings.NewReader(multilineStreamEvent), func(resp *AgentStreamResponse) error {
		got = resp
		return nil
	})
	if err != nil {
		t.Fatalf("multiline SSE data frame failed: %v", err)
	}
	if got == nil {
		t.Fatal("callback was not invoked")
	}
	if got.ResponseType != "answer" || got.Content != "hello" || got.Done {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestKnowledgeQAStreamMultilineDataFrame(t *testing.T) {
	c := NewClient("http://example.test", WithTransport(sseRoundTripper{body: multilineStreamEvent}))
	var got *StreamResponse

	err := c.KnowledgeQAStream(context.Background(), "session-1", &KnowledgeQARequest{Query: "hi"}, func(resp *StreamResponse) error {
		got = resp
		return nil
	})
	if err != nil {
		t.Fatalf("multiline knowledge SSE data frame failed: %v", err)
	}
	if got == nil {
		t.Fatal("callback was not invoked")
	}
	if got.ResponseType != "answer" || got.Content != "hello" || got.Done {
		t.Fatalf("unexpected response: %#v", got)
	}
}

func TestContinueStreamMultilineDataFrame(t *testing.T) {
	body := "event:message\n" + multilineStreamEvent
	c := NewClient("http://example.test", WithTransport(sseRoundTripper{body: body}))
	var got *StreamResponse

	err := c.ContinueStream(context.Background(), "session-1", "message-1", func(resp *StreamResponse) error {
		got = resp
		return nil
	})
	if err != nil {
		t.Fatalf("multiline continue SSE data frame failed: %v", err)
	}
	if got == nil {
		t.Fatal("callback was not invoked")
	}
	if got.ResponseType != "answer" || got.Content != "hello" || got.Done {
		t.Fatalf("unexpected response: %#v", got)
	}
}
