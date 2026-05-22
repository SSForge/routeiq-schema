// Package pii_redactor implements an OTel Collector processor that scrubs
// values from configured routeiq.* attribute paths before storage.
package pii_redactor

import (
	"context"
	"strings"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
)

const redactedValue = "[REDACTED]"

// Config lists attribute key prefixes whose values should be redacted.
type Config struct {
	RedactPrefixes []string
}

// DefaultConfig returns the default redaction config covering fields that
// may contain user-supplied free text subject to PII regulations.
func DefaultConfig() Config {
	return Config{
		RedactPrefixes: []string{
			"routeiq.retrieval.query",
			"routeiq.memory.key",
			"routeiq.handoff.context",
			"routeiq.task.input_intent",
			"routeiq.task.actual_outcome",
			"routeiq.tool_call.arguments_summary",
		},
	}
}

// Processor redacts sensitive attribute values.
type Processor struct {
	next   consumer.Traces
	config Config
}

// NewProcessor creates a Processor with the given config.
func NewProcessor(next consumer.Traces, cfg Config) *Processor {
	return &Processor{next: next, config: cfg}
}

// ConsumeTraces redacts configured attribute keys in-place before forwarding.
func (p *Processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			for m := 0; m < spans.Len(); m++ {
				p.redactSpan(spans.At(m))
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

func (p *Processor) redactSpan(span ptrace.Span) {
	span.Attributes().Range(func(k string, _ ptrace.Value) bool {
		for _, prefix := range p.config.RedactPrefixes {
			if strings.HasPrefix(k, prefix) {
				span.Attributes().PutStr(k, redactedValue)
				break
			}
		}
		return true
	})
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error { return nil }
func (p *Processor) Shutdown(_ context.Context) error                 { return nil }
