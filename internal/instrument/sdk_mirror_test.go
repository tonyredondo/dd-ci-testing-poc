package instrument

import (
	"errors"
	"go/parser"
	"go/token"
	"strings"
	"testing"
)

const sdkMirrorSource = `package tracer
type SpanContext struct{}
func spanStart(operationName string) *Span {
 span.context = newSpanContext(span, context)
 return span
}
func(s *Span)finish(finishTime int64){
 s.mu.Lock();defer s.mu.Unlock()
 if s.finished{return}
 s.context.finish(s)
}
func ContextWithSpan(ctx context.Context,s *Span) context.Context {
	var snapshot *SpanContext
	if s!=nil {snapshot=s.Context()}
 newCtx := &spanCtx{Context:ctx,span:s}
 return newCtx
}
`

func TestSDKMirrorAPIDrift(t *testing.T) {
	got, hooks, err := TransformSDKMirror("sdk.go", []byte(sdkMirrorSource))
	if err != nil || hooks != 15 {
		t.Fatal(hooks, err)
	}
	if _, err := parser.ParseFile(token.NewFileSet(), "sdk.go", got, 0); err != nil {
		t.Fatal(err)
	}
	for _, mutation := range [][2]string{
		{"type SpanContext struct{}", "type SpanContext interface{}"},
		{"*Span {", "Span {"},
		{"newSpanContext(span, context)", "newSpanContext(span, nil)"},
		{"s.context.finish(s)", "s.context.complete(s)"},
		{"finishTime int64", "finishTime time.Time"},
		{"s *Span) context.Context", "s Span) context.Context"},
		{"newCtx :=", "different :="},
		{"defer s.mu.Unlock()", "s.mu.Unlock()"},
		{"var snapshot *SpanContext", "var snapshot any"},
	} {
		source := strings.Replace(sdkMirrorSource, mutation[0], mutation[1], 1)
		if _, _, err := TransformSDKMirror("sdk.go", []byte(source)); !errors.Is(err, ErrUnsupportedAPI) {
			t.Fatalf("API drift %v was accepted: %v", mutation, err)
		}
	}
	if _, _, err := TransformSDKMirror("sdk.go", append(got, []byte("\n")...)); !errors.Is(err, ErrUnsupportedAPI) {
		t.Fatal("double instrumentation accepted", err)
	}
}
