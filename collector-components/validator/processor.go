// Package validator implements an OTel Collector processor that validates
// incoming spans against the routeiq semantic convention registry
// (conventions/telemetry.yaml). Spans missing required attributes are dropped.
package validator

import (
	"context"
	"fmt"
	"os"

	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/pdata/ptrace"
	"gopkg.in/yaml.v3"
)

// Convention mirrors one entry in conventions/telemetry.yaml.
type Convention struct {
	ProtoField string `yaml:"proto_field"`
	OtelKey    string `yaml:"otel_attribute_key"`
	Required   bool   `yaml:"required"`
}

type conventionFile struct {
	Conventions []Convention `yaml:"conventions"`
}

// Processor validates routeiq.* span attributes against the convention registry.
type Processor struct {
	next        consumer.Traces
	conventions []Convention
}

// NewProcessor creates a Processor loaded with conventions from conventionsPath.
func NewProcessor(next consumer.Traces, conventionsPath string) (*Processor, error) {
	data, err := os.ReadFile(conventionsPath)
	if err != nil {
		return nil, fmt.Errorf("read conventions: %w", err)
	}
	var cf conventionFile
	if err := yaml.Unmarshal(data, &cf); err != nil {
		return nil, fmt.Errorf("parse conventions: %w", err)
	}
	return &Processor{next: next, conventions: cf.Conventions}, nil
}

// ConsumeTraces validates each span and drops spans missing required attributes.
func (p *Processor) ConsumeTraces(ctx context.Context, td ptrace.Traces) error {
	rss := td.ResourceSpans()
	for i := 0; i < rss.Len(); i++ {
		sss := rss.At(i).ScopeSpans()
		for j := 0; j < sss.Len(); j++ {
			spans := sss.At(j).Spans()
			k := 0
			for m := 0; m < spans.Len(); m++ {
				if p.isValid(spans.At(m)) {
					if k != m {
						spans.At(m).CopyTo(spans.At(k))
					}
					k++
				}
			}
		}
	}
	return p.next.ConsumeTraces(ctx, td)
}

func (p *Processor) isValid(span ptrace.Span) bool {
	attrs := span.Attributes()
	for _, conv := range p.conventions {
		if !conv.Required {
			continue
		}
		if _, ok := attrs.Get(conv.OtelKey); !ok {
			return false
		}
	}
	return true
}

func (p *Processor) Capabilities() consumer.Capabilities {
	return consumer.Capabilities{MutatesData: true}
}

func (p *Processor) Start(_ context.Context, _ component.Host) error { return nil }
func (p *Processor) Shutdown(_ context.Context) error                 { return nil }
