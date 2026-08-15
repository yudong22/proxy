// Package history maintains an in-memory ring buffer of recent proxy requests.
package history

import "time"

// RequestRecord holds metadata for a single completed proxy request.
type RequestRecord struct {
	ID           string        // unique request ID
	Model        string        // actual upstream model used (e.g. "kimi-k2.6")
	Provider     string        // provider name (e.g. "opencode-go")
	Scenario     string        // routing scenario (e.g. "default", "complex")
	StartTime    time.Time     // when the request started
	Duration     time.Duration // total latency
	InputTokens  int           // input tokens from SSE usage event
	OutputTokens int           // output tokens from SSE usage event
	// CacheReadInputTokens and CacheCreationInputTokens break out the portions
	// of the prompt served from / written to the upstream prompt cache. Per the
	// Anthropic spec, InputTokens excludes these, so a heavily-cached request
	// can report a small (even zero) InputTokens while consuming many total
	// tokens. The GUI sums all three to show the total prompt size.
	CacheReadInputTokens     int    // tokens read from the prompt cache
	CacheCreationInputTokens int    // tokens written to the prompt cache
	Streaming                bool   // whether this was a streaming request
	Success                  bool   // whether it completed successfully
	ErrorMsg                 string // error message if failed
}
