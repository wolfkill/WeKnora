package service

import (
	"context"
	"testing"

	"github.com/Tencent/WeKnora/internal/config"
	"github.com/Tencent/WeKnora/internal/models/chat"
	"github.com/Tencent/WeKnora/internal/types"
)

type wikiIngestOptionCaptureChat struct {
	opts *chat.ChatOptions
}

func (m *wikiIngestOptionCaptureChat) Chat(
	_ context.Context,
	_ []chat.Message,
	opts *chat.ChatOptions,
) (*types.ChatResponse, error) {
	if opts != nil {
		copied := *opts
		m.opts = &copied
	}
	return &types.ChatResponse{Content: "ok"}, nil
}

func (m *wikiIngestOptionCaptureChat) ChatStream(
	context.Context,
	[]chat.Message,
	*chat.ChatOptions,
) (<-chan types.StreamResponse, error) {
	return nil, nil
}

func (m *wikiIngestOptionCaptureChat) GetModelName() string { return "capture" }
func (m *wikiIngestOptionCaptureChat) GetModelID() string   { return "capture" }

func TestGenerateWithTemplateUsesConfiguredTokenLimits(t *testing.T) {
	model := &wikiIngestOptionCaptureChat{}
	svc := &wikiIngestService{
		config: &config.Config{
			Conversation: &config.ConversationConfig{
				Summary: &config.SummaryConfig{
					MaxTokens:           3072,
					MaxCompletionTokens: 4096,
				},
			},
		},
	}

	content, err := svc.generateWithTemplate(
		context.Background(),
		model,
		"hello {{.Name}}",
		map[string]string{"Name": "wiki"},
	)
	if err != nil {
		t.Fatalf("generateWithTemplate returned error: %v", err)
	}
	if content != "ok" {
		t.Fatalf("unexpected content: got %q want %q", content, "ok")
	}
	if model.opts == nil {
		t.Fatal("chat options were not captured")
	}
	if model.opts.MaxTokens != 3072 {
		t.Fatalf("MaxTokens = %d, want 3072", model.opts.MaxTokens)
	}
	if model.opts.MaxCompletionTokens != 4096 {
		t.Fatalf("MaxCompletionTokens = %d, want 4096", model.opts.MaxCompletionTokens)
	}
}

func TestGenerateWithTemplateLeavesTokenLimitsUnsetWithoutConfig(t *testing.T) {
	model := &wikiIngestOptionCaptureChat{}
	svc := &wikiIngestService{}

	_, err := svc.generateWithTemplate(
		context.Background(),
		model,
		"hello",
		nil,
	)
	if err != nil {
		t.Fatalf("generateWithTemplate returned error: %v", err)
	}
	if model.opts == nil {
		t.Fatal("chat options were not captured")
	}
	if model.opts.MaxTokens != 0 {
		t.Fatalf("MaxTokens = %d, want 0", model.opts.MaxTokens)
	}
	if model.opts.MaxCompletionTokens != 0 {
		t.Fatalf("MaxCompletionTokens = %d, want 0", model.opts.MaxCompletionTokens)
	}
}

func TestSlugify(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"Hello World", "hello-world"},
		{"Acme Corp", "acme-corp"},
		{"  spaces  ", "spaces"},
		{"under_score", "under-score"},
		{"Already-Good", "already-good"},
		{"Special!@#Chars", "specialchars"},
		{"CamelCase", "camelcase"},
		{"", ""},
		{"a/b/c", "a/b/c"},               // preserve slashes for hierarchical slugs
		{"中文标题", "中文标题"},                 // preserve CJK
		{"Mix 中英文 Test", "mix-中英文-test"}, // mixed
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := slugify(tt.input)
			if got != tt.want {
				t.Errorf("slugify(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestTruncateString(t *testing.T) {
	tests := []struct {
		input  string
		maxLen int
		want   string
	}{
		{"hello world", 20, "hello world"},
		{"hello world", 5, "hello..."},
		{"", 10, ""},
		{"abc", 3, "abc"},
		{"abcd", 3, "abc..."},
		{"中文测试", 2, "中文..."},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncateString(tt.input, tt.maxLen)
			if got != tt.want {
				t.Errorf("truncateString(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.want)
			}
		})
	}
}

func TestAppendUnique(t *testing.T) {
	arr := types.StringArray{"a", "b"}

	// Add new
	result := appendUnique(arr, "c")
	if len(result) != 3 {
		t.Errorf("Expected 3 items, got %d", len(result))
	}

	// Add duplicate
	result = appendUnique(result, "b")
	if len(result) != 3 {
		t.Errorf("Expected 3 items (no dup), got %d", len(result))
	}
}

func TestReconstructContent(t *testing.T) {
	chunks := []*types.Chunk{
		{ChunkIndex: 2, ChunkType: types.ChunkTypeText, Content: "Third paragraph."},
		{ChunkIndex: 0, ChunkType: types.ChunkTypeText, Content: "First paragraph."},
		{ChunkIndex: 1, ChunkType: types.ChunkTypeText, Content: "Second paragraph."},
		{ChunkIndex: 3, ChunkType: types.ChunkTypeImageOCR, Content: "OCR text should be excluded."},
	}

	content := reconstructContent(chunks)

	// Should be sorted by ChunkIndex and exclude non-text chunks
	if content == "" {
		t.Fatal("reconstructContent should not be empty")
	}

	// Verify order: first, second, third
	if len(content) == 0 {
		t.Fatal("empty content")
	}

	// The first characters should be "First"
	if content[:5] != "First" {
		t.Errorf("Expected content to start with 'First', got: %s", content[:20])
	}
}

func TestReconstructContentEmpty(t *testing.T) {
	content := reconstructContent(nil)
	if content != "" {
		t.Errorf("Empty chunks should produce empty content, got %q", content)
	}
}

func TestStripImageMarkup(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"plain text untouched", "Hello world.", "Hello world."},
		{"single markdown image removed", "![alt](images/page_1.png)", ""},
		{
			"scanned-pdf style page references all stripped",
			"![MX5280_page_1.png](images/MX5280_page_1.png)\n\n![MX5280_page_2.png](images/MX5280_page_2.png)",
			"\n\n",
		},
		{"mixed text and image keeps text", "Intro paragraph.\n![fig](a.png)\nConclusion.", "Intro paragraph.\n\nConclusion."},
		{"html img tag stripped", `Before <img src="x.png" alt="y"/> after`, "Before  after"},
		{
			// Regression guard: an earlier version stripped the WHOLE
			// <image>...</image> block (including <image_ocr> content),
			// silently destroying successful VLM OCR results. The fix must
			// preserve the inner OCR / caption text.
			"enriched <image> block keeps inner OCR + caption text",
			`<image url="images/page_1.png">
<image_original>![p1](images/page_1.png)</image_original>
<image_caption>scanned letter on letterhead</image_caption>
<image_ocr>SEHR GEEHRTER HERR MUSTERMANN, ...</image_ocr>
</image>`,
			"\n\nscanned letter on letterhead\nSEHR GEEHRTER HERR MUSTERMANN, ...\n",
		},
		{
			"empty <image> block (OCR failed) reduces to whitespace",
			`<image url="x"><image_original>![a](x)</image_original></image>`,
			"",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := stripImageMarkup(tt.input)
			if got != tt.want {
				t.Errorf("stripImageMarkup(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestHasSufficientTextContent(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  bool
	}{
		{"empty string", "", false},
		{"only whitespace", "   \n\n\t  ", false},
		{
			"only image references (scanned PDF without OCR)",
			"![MX5280_page_1.png](images/MX5280_page_1.png)\n![MX5280_page_2.png](images/MX5280_page_2.png)",
			false,
		},
		{"too-short text below 10-rune threshold", "hi", false},
		{
			"short legitimate note above threshold",
			"Meeting at 3pm tomorrow.",
			true,
		},
		{
			"image-only with successful VLM OCR (the fix)",
			`<image url="images/p1.png">
<image_original>![p1](images/p1.png)</image_original>
<image_caption>scanned letter</image_caption>
<image_ocr>Sehr geehrter Herr Mustermann, in der Sache 4711/2024 ...</image_ocr>
</image>`,
			true,
		},
		{
			"image-only with failed VLM OCR (still rejected)",
			`<image url="images/p1.png">
<image_original>![p1](images/p1.png)</image_original>
</image>`,
			false,
		},
		{
			"sufficient text mixed with images still passes",
			"![cover](cover.png)\nDie Beklagte hat die Klage anerkannt.\n![sig](sig.png)",
			true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := hasSufficientTextContent(tt.input)
			if got != tt.want {
				t.Errorf("hasSufficientTextContent(%q) = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}
