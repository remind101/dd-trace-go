package opentracing

import (
	"net/http"
	"testing"
	"time"

	ddtrace "github.com/DataDog/dd-trace-go/tracer"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/stretchr/testify/assert"
)

func TestDefaultTracer(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)
	tTracer, ok := tracer.(*Tracer)
	assert.True(ok)

	assert.Equal(tTracer.impl, ddtrace.DefaultTracer)
}

func TestTracerStartSpan(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	span, ok := tracer.StartSpan("web.request").(*Span)
	assert.True(ok)

	assert.NotEqual(uint64(0), span.Span.TraceID)
	assert.NotEqual(uint64(0), span.Span.SpanID)
	assert.Equal(uint64(0), span.Span.ParentID)
	assert.Equal("web.request", span.Span.Name)
	assert.Equal("opentracing.test", span.Span.Service)
	assert.NotNil(span.Span.Tracer())
}

func TestTracerStartChildSpan(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	root := tracer.StartSpan("web.request")
	child := tracer.StartSpan("db.query", opentracing.ChildOf(root.Context()))
	tRoot, ok := root.(*Span)
	assert.True(ok)
	tChild, ok := child.(*Span)
	assert.True(ok)

	assert.NotEqual(uint64(0), tChild.Span.TraceID)
	assert.NotEqual(uint64(0), tChild.Span.SpanID)
	assert.Equal(tRoot.Span.SpanID, tChild.Span.ParentID)
	assert.Equal(tRoot.Span.TraceID, tChild.Span.ParentID)
}

func TestTracerBaggagePropagation(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	root := tracer.StartSpan("web.request")
	root.SetBaggageItem("key", "value")
	child := tracer.StartSpan("db.query", opentracing.ChildOf(root.Context()))
	context, ok := child.Context().(SpanContext)
	assert.True(ok)

	assert.Equal("value", context.baggage["key"])
}

func TestTracerBaggageImmutability(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	root := tracer.StartSpan("web.request")
	root.SetBaggageItem("key", "value")
	child := tracer.StartSpan("db.query", opentracing.ChildOf(root.Context()))
	child.SetBaggageItem("key", "changed!")
	parentContext, ok := root.Context().(SpanContext)
	assert.True(ok)
	childContext, ok := child.Context().(SpanContext)
	assert.True(ok)

	assert.Equal("value", parentContext.baggage["key"])
	assert.Equal("changed!", childContext.baggage["key"])
}

func TestTracerSpanTags(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	tag := opentracing.Tag{Key: "key", Value: "value"}
	span, ok := tracer.StartSpan("web.request", tag).(*Span)
	assert.True(ok)

	assert.Equal("value", span.Span.Meta["key"])
}

func TestTracerSpanStartTime(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	startTime := time.Now().Add(-10 * time.Second)
	span, ok := tracer.StartSpan("web.request", opentracing.StartTime(startTime)).(*Span)
	assert.True(ok)

	assert.Equal(startTime.UnixNano(), span.Span.Start)
}

func TestTracerPropagation(t *testing.T) {
	assert := assert.New(t)

	config := NewConfiguration()
	tracer, _, _ := NewTracer(config)

	root := tracer.StartSpan("web.request")
	ctx := root.Context()
	headers := http.Header{}

	// inject the SpanContext
	carrier := opentracing.HTTPHeadersCarrier(headers)
	err := tracer.Inject(ctx, opentracing.HTTPHeaders, carrier)
	assert.Nil(err)

	// retrieve the SpanContext
	propagated, err := tracer.Extract(opentracing.HTTPHeaders, carrier)
	assert.Nil(err)

	tCtx, ok := ctx.(SpanContext)
	assert.True(ok)
	tPropagated, ok := propagated.(SpanContext)
	assert.True(ok)

	// compare if there is a Context match
	assert.Equal(tCtx.traceID, tPropagated.traceID)
	assert.Equal(tCtx.spanID, tPropagated.spanID)

	// ensure a child can be created
	child := tracer.StartSpan("db.query", opentracing.ChildOf(propagated))
	tRoot, ok := root.(*Span)
	assert.True(ok)
	tChild, ok := child.(*Span)
	assert.True(ok)

	assert.NotEqual(uint64(0), tChild.Span.TraceID)
	assert.NotEqual(uint64(0), tChild.Span.SpanID)
	assert.Equal(tRoot.Span.SpanID, tChild.Span.ParentID)
	assert.Equal(tRoot.Span.TraceID, tChild.Span.ParentID)
}
