// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package transform

import (
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	"github.com/google/go-cmp/cmp"
	tracepb "github.com/open-telemetry/opentelemetry-proto/gen/go/trace/v1"
	"github.com/stretchr/testify/assert"
	"google.golang.org/grpc/codes"

	"go.opentelemetry.io/otel/api/core"
	apitrace "go.opentelemetry.io/otel/api/trace"
	export "go.opentelemetry.io/otel/sdk/export/trace"
	"go.opentelemetry.io/otel/sdk/resource"
)

func TestSpanKind(t *testing.T) {
	for _, test := range []struct {
		kind     apitrace.SpanKind
		expected tracepb.Span_SpanKind
	}{
		{
			apitrace.SpanKindInternal,
			tracepb.Span_INTERNAL,
		},
		{
			apitrace.SpanKindClient,
			tracepb.Span_CLIENT,
		},
		{
			apitrace.SpanKindServer,
			tracepb.Span_SERVER,
		},
		{
			apitrace.SpanKindProducer,
			tracepb.Span_PRODUCER,
		},
		{
			apitrace.SpanKindConsumer,
			tracepb.Span_CONSUMER,
		},
		{
			apitrace.SpanKind(-1),
			tracepb.Span_SPAN_KIND_UNSPECIFIED,
		},
	} {
		assert.Equal(t, test.expected, spanKind(test.kind))
	}
}

func TestNilSpanEvent(t *testing.T) {
	assert.Nil(t, spanEvents(nil))
}

func TestEmptySpanEvent(t *testing.T) {
	assert.Nil(t, spanEvents([]export.Event{}))
}

func TestSpanEvent(t *testing.T) {
	attrs := []core.KeyValue{core.Key("one").Int(1), core.Key("two").Int(2)}
	now := time.Now()
	got := spanEvents([]export.Event{
		{
			Name:       "test 1",
			Attributes: []core.KeyValue{},
			Time:       now,
		},
		{
			Name:       "test 2",
			Attributes: attrs,
			Time:       now,
		},
	})
	if !assert.Len(t, got, 2) {
		return
	}
	uNow := uint64(now.Nanosecond())
	assert.Equal(t, &tracepb.Span_Event{Name: "test 1", Attributes: nil, TimeUnixNano: uNow}, got[0])
	// Do not test Attributes directly, just that the return value goes to the correct field.
	assert.Equal(t, &tracepb.Span_Event{Name: "test 2", Attributes: Attributes(attrs), TimeUnixNano: uNow}, got[1])
}

func TestExcessiveSpanEvents(t *testing.T) {
	e := make([]export.Event, maxMessageEventsPerSpan+1)
	for i := 0; i < maxMessageEventsPerSpan+1; i++ {
		e[i] = export.Event{Name: strconv.Itoa(i)}
	}
	assert.Len(t, e, maxMessageEventsPerSpan+1)
	got := spanEvents(e)
	assert.Len(t, got, maxMessageEventsPerSpan)
	// Ensure the drop order.
	assert.Equal(t, strconv.Itoa(maxMessageEventsPerSpan-1), got[len(got)-1].Name)
}

func TestNilLinks(t *testing.T) {
	assert.Nil(t, links(nil))
}

func TestEmptyLinks(t *testing.T) {
	assert.Nil(t, links([]apitrace.Link{}))
}

func TestLinks(t *testing.T) {
	attrs := []core.KeyValue{core.Key("one").Int(1), core.Key("two").Int(2)}
	l := []apitrace.Link{
		{},
		{
			SpanContext: core.EmptySpanContext(),
			Attributes:  attrs,
		},
	}
	got := links(l)

	// Make sure we get the same number back first.
	if !assert.Len(t, got, 2) {
		return
	}

	// Empty should be empty.
	expected := &tracepb.Span_Link{
		TraceId: []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
		SpanId:  []uint8{0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0},
	}
	assert.Equal(t, expected, got[0])

	// Do not test Attributes directly, just that the return value goes to the correct field.
	expected.Attributes = Attributes(attrs)
	assert.Equal(t, expected, got[1])

	// Changes to our links should not change the produced links.
	l[1].TraceID[0] = byte(0x1)
	l[1].SpanID[0] = byte(0x1)
	assert.Equal(t, expected, got[1])
}

func TestStatus(t *testing.T) {
	for _, test := range []struct {
		grpcCode   codes.Code
		message    string
		otlpStatus tracepb.Status_StatusCode
	}{
		{
			codes.OK,
			"test OK",
			tracepb.Status_Ok,
		},
		{
			codes.Canceled,
			//nolint
			"test CANCELLED",
			//nolint
			tracepb.Status_Cancelled,
		},
		{
			codes.Unknown,
			"test UNKNOWN",
			tracepb.Status_UnknownError,
		},
		{
			codes.InvalidArgument,
			"test INVALID_ARGUMENT",
			tracepb.Status_InvalidArgument,
		},
		{
			codes.DeadlineExceeded,
			"test DEADLINE_EXCEEDED",
			tracepb.Status_DeadlineExceeded,
		},
		{
			codes.NotFound,
			"test NOT_FOUND",
			tracepb.Status_NotFound,
		},
		{
			codes.AlreadyExists,
			"test ALREADY_EXISTS",
			tracepb.Status_AlreadyExists,
		},
		{
			codes.PermissionDenied,
			"test PERMISSION_DENIED",
			tracepb.Status_PermissionDenied,
		},
		{
			codes.ResourceExhausted,
			"test RESOURCE_EXHAUSTED",
			tracepb.Status_ResourceExhausted,
		},
		{
			codes.FailedPrecondition,
			"test FAILED_PRECONDITION",
			tracepb.Status_FailedPrecondition,
		},
		{
			codes.Aborted,
			"test ABORTED",
			tracepb.Status_Aborted,
		},
		{
			codes.OutOfRange,
			"test OUT_OF_RANGE",
			tracepb.Status_OutOfRange,
		},
		{
			codes.Unimplemented,
			"test UNIMPLEMENTED",
			tracepb.Status_Unimplemented,
		},
		{
			codes.Internal,
			"test INTERNAL",
			tracepb.Status_InternalError,
		},
		{
			codes.Unavailable,
			"test UNAVAILABLE",
			tracepb.Status_Unavailable,
		},
		{
			codes.DataLoss,
			"test DATA_LOSS",
			tracepb.Status_DataLoss,
		},
		{
			codes.Unauthenticated,
			"test UNAUTHENTICATED",
			tracepb.Status_Unauthenticated,
		},
	} {
		expected := &tracepb.Status{Code: test.otlpStatus, Message: test.message}
		assert.Equal(t, expected, status(test.grpcCode, test.message))
	}

}

func TestNilSpan(t *testing.T) {
	assert.Nil(t, span(nil))
}

func TestNilSpanData(t *testing.T) {
	assert.Nil(t, SpanData(nil))
}

func TestEmptySpanData(t *testing.T) {
	assert.Nil(t, SpanData(nil))
}

func TestSpanData(t *testing.T) {
	// Full test of span data transform.

	startTime := time.Now()
	endTime := startTime.Add(10 * time.Second)
	spanData := &export.SpanData{
		SpanContext: core.SpanContext{
			TraceID: core.TraceID{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
			SpanID:  core.SpanID{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8},
		},
		SpanKind:     apitrace.SpanKindServer,
		ParentSpanID: core.SpanID{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		Name:         "span data to span data",
		StartTime:    startTime,
		EndTime:      endTime,
		MessageEvents: []export.Event{
			{Time: startTime,
				Attributes: []core.KeyValue{
					core.Key("CompressedByteSize").Uint64(512),
				},
			},
			{Time: endTime,
				Attributes: []core.KeyValue{
					core.Key("MessageEventType").String("Recv"),
				},
			},
		},
		Links: []apitrace.Link{
			{
				SpanContext: core.SpanContext{
					TraceID:    core.TraceID{0xC0, 0xC1, 0xC2, 0xC3, 0xC4, 0xC5, 0xC6, 0xC7, 0xC8, 0xC9, 0xCA, 0xCB, 0xCC, 0xCD, 0xCE, 0xCF},
					SpanID:     core.SpanID{0xB0, 0xB1, 0xB2, 0xB3, 0xB4, 0xB5, 0xB6, 0xB7},
					TraceFlags: 0,
				},
				Attributes: []core.KeyValue{
					core.Key("LinkType").String("Parent"),
				},
			},
			{
				SpanContext: core.SpanContext{
					TraceID:    core.TraceID{0xE0, 0xE1, 0xE2, 0xE3, 0xE4, 0xE5, 0xE6, 0xE7, 0xE8, 0xE9, 0xEA, 0xEB, 0xEC, 0xED, 0xEE, 0xEF},
					SpanID:     core.SpanID{0xD0, 0xD1, 0xD2, 0xD3, 0xD4, 0xD5, 0xD6, 0xD7},
					TraceFlags: 0,
				},
				Attributes: []core.KeyValue{
					core.Key("LinkType").String("Child"),
				},
			},
		},
		StatusCode:      codes.Internal,
		StatusMessage:   "utterly unrecognized",
		HasRemoteParent: true,
		Attributes: []core.KeyValue{
			core.Key("timeout_ns").Int64(12e9),
		},
		DroppedAttributeCount:    1,
		DroppedMessageEventCount: 2,
		DroppedLinkCount:         3,
		Resource:                 resource.New(core.Key("rk1").String("rv1"), core.Key("rk2").Int64(5)),
	}

	// Not checking resource as the underlying map of our Resource makes
	// ordering impossible to guarantee on the output. The Resource
	// transform function has unit tests that should suffice.
	expectedSpan := &tracepb.Span{
		TraceId:                []byte{0x00, 0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08, 0x09, 0x0A, 0x0B, 0x0C, 0x0D, 0x0E, 0x0F},
		SpanId:                 []byte{0xFF, 0xFE, 0xFD, 0xFC, 0xFB, 0xFA, 0xF9, 0xF8},
		ParentSpanId:           []byte{0xEF, 0xEE, 0xED, 0xEC, 0xEB, 0xEA, 0xE9, 0xE8},
		Name:                   spanData.Name,
		Kind:                   tracepb.Span_SERVER,
		StartTimeUnixNano:      uint64(startTime.Nanosecond()),
		EndTimeUnixNano:        uint64(endTime.Nanosecond()),
		Status:                 status(spanData.StatusCode, spanData.StatusMessage),
		Events:                 spanEvents(spanData.MessageEvents),
		Links:                  links(spanData.Links),
		Attributes:             Attributes(spanData.Attributes),
		DroppedAttributesCount: 1,
		DroppedEventsCount:     2,
		DroppedLinksCount:      3,
	}

	got := SpanData([]*export.SpanData{spanData})
	if !assert.Len(t, got, 1) {
		return
	}

	// Break the span down as large diffs can be hard to read.
	actualSpans := got[0].GetInstrumentationLibrarySpans()
	if !assert.Len(t, actualSpans, 1) && !assert.Len(t, actualSpans[0].Spans, 1) {
		return
	}
	actualSpan := actualSpans[0].Spans[0]

	if diff := cmp.Diff(expectedSpan, actualSpan, cmp.Comparer(proto.Equal)); diff != "" {
		t.Fatalf("transformed span differs %v\n", diff)
	}
}
