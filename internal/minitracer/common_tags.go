//go:build go1.26

package minitracer

import (
	"maps"
	"strings"

	"github.com/tonyredondo/dd-ci-testing-poc/internal/thirdparty/msgp/msgp"
)

// CommonTags owns an immutable CI tag snapshot. Spans keep their own overrides;
// getters and wire projection resolve those overrides before these defaults.
type CommonTags struct {
	values map[string]string
	bytes  int
}

// IsSharedCITag restricts automatic lifting to the CI/Git/system string tags.
// Capabilities, test attributes, IDs and metrics keep their existing wire fields.
func IsSharedCITag(key string) bool {
	return strings.HasPrefix(key, "ci.") || strings.HasPrefix(key, "git.") ||
		strings.HasPrefix(key, "os.") || strings.HasPrefix(key, "runtime.") || key == "_dd.ci.env_vars"
}

// NewCommonTags copies the CI/Git/system values selected by IsSharedCITag.
// Callers may reuse or mutate their input after construction.
func NewCommonTags(values map[string]string) *CommonTags {
	common := &CommonTags{values: maps.Clone(values)}
	for key, value := range common.values {
		common.bytes += 2*msgp.StringPrefixSize + len(key) + len(value)
	}
	return common
}

// Option applies defaults at this point in option order. Later string/numeric
// tags win; an earlier tag of the same name is replaced, as with ordinary Tag.
func (c *CommonTags) Option() StartSpanOption {
	return func(s *Span) {
		if s.common != nil {
			// Multiple common options are uncommon, but retain earlier keys that
			// the new option does not replace, just as individual Tag options do.
			for key, value := range s.common.values {
				if _, text := s.content.Meta[key]; !text {
					if _, numeric := s.content.Metrics[key]; !numeric {
						s.content.Meta[key] = value
					}
				}
			}
		}
		for key := range s.content.Meta {
			if _, replace := c.values[key]; replace {
				delete(s.content.Meta, key)
			}
		}
		for key := range s.content.Metrics {
			if _, replace := c.values[key]; replace {
				delete(s.content.Metrics, key)
			}
		}
		s.common = c
	}
}

// Account for all effective strings, including immutable defaults. Overrides
// replace those strings rather than adding both values to the wire payload.
func eventSize(event *ciEvent) int {
	size := event.Msgsize()
	if event.common != nil {
		// Include a metadata map even when the event has no local strings.
		size += event.common.bytes + msgp.MapHeaderSize
		for key := range event.Content.Meta {
			if value, replaced := event.common.values[key]; replaced {
				size -= 2*msgp.StringPrefixSize + len(key) + len(value)
			}
		}
		for key := range event.Content.Metrics {
			if _, text := event.Content.Meta[key]; text {
				continue
			}
			if value, replaced := event.common.values[key]; replaced {
				size -= 2*msgp.StringPrefixSize + len(key) + len(value)
			}
		}
	}
	return size
}

// prepareCommonMetadata keeps CI/Git/system strings on each event, matching
// the SDK's intake representation. The shared snapshot remains immutable in
// memory; projection owns the copy and never changes a finished span.
func prepareCommonMetadata(base map[string]map[string]string, events ciEvents) (map[string]map[string]string, ciEvents) {
	var projected ciEvents
	for i, event := range events {
		if event.common == nil || len(event.common.values) == 0 {
			continue
		}
		meta := make(map[string]string, len(event.Content.Meta)+len(event.common.values))
		maps.Copy(meta, event.Content.Meta)
		for key, value := range event.common.values {
			if _, local := meta[key]; local {
				continue
			}
			if _, numeric := event.Content.Metrics[key]; numeric {
				continue
			}
			meta[key] = value
		}
		if projected == nil {
			projected = append(ciEvents(nil), events...)
		}
		copy := *event
		copy.Content.Meta = meta
		projected[i] = &copy
	}
	if projected == nil {
		projected = events
	}
	return base, projected
}
