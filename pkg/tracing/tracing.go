/*
 * Copyright (c) 2017, MegaEase
 * All rights reserved.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package tracing

import (
	"io"
	"time"

	"github.com/megaease/easegress/pkg/util/fasttime"

	zipkingo "github.com/openzipkin/zipkin-go"
	zipkinreporter "github.com/openzipkin/zipkin-go/reporter"
	zipkingohttp "github.com/openzipkin/zipkin-go/reporter/http"
)

type (
	// Spec describes Tracer.
	Spec struct {
		ServiceName string            `json:"serviceName" jsonschema:"required"`
		Tags        map[string]string `json:"tags" jsonschema:"omitempty"`
		Zipkin      *ZipkinSpec       `json:"zipkin" jsonschema:"required"`
	}

	// ZipkinSpec describes Zipkin.
	ZipkinSpec struct {
		Hostport      string  `json:"hostport" jsonschema:"omitempty"`
		ServerURL     string  `json:"serverURL" jsonschema:"required,format=url"`
		DisableReport bool    `json:"disableReport" jsonschema:"omitempty"`
		SampleRate    float64 `json:"sampleRate" jsonschema:"required,minimum=0,maximum=1"`
		SameSpan      bool    `json:"sameSpan" jsonschema:"omitempty"`
		ID128Bit      bool    `json:"id128Bit" jsonschema:"omitempty"`
	}

	// Tracer is the tracer.
	Tracer struct {
		tracer *zipkingo.Tracer
		tags   map[string]string
		closer io.Closer
	}

	noopCloser struct{}
)

// Validate validates Spec.
func (spec *ZipkinSpec) Validate() error {
	if spec.Hostport != "" {
		_, err := zipkingo.NewEndpoint("", spec.Hostport)
		if err != nil {
			return err
		}
	}

	return nil
}

// NoopTracer is the tracer doing nothing.
var NoopTracer *Tracer

func init() {
	tracer, _ := zipkingo.NewTracer(nil)
	NoopTracer = &Tracer{tracer: tracer, closer: nil}
	NoopSpan = &span{tracer: NoopTracer, Span: NoopTracer.tracer.StartSpan("")}
}

// New creates a Tracing.
func New(spec *Spec) (*Tracer, error) {
	if spec == nil {
		return NoopTracer, nil
	}

	endpoint, err := zipkingo.NewEndpoint(spec.ServiceName, spec.Zipkin.Hostport)
	if err != nil {
		return nil, err
	}

	sampler, err := zipkingo.NewBoundarySampler(spec.Zipkin.SampleRate, fasttime.Now().Unix())
	if err != nil {
		return nil, err
	}

	var reporter zipkinreporter.Reporter
	if spec.Zipkin.DisableReport {
		reporter = zipkinreporter.NewNoopReporter()
	} else {
		reporter = zipkingohttp.NewReporter(spec.Zipkin.ServerURL)
	}
	tracer, err := zipkingo.NewTracer(
		reporter,
		zipkingo.WithLocalEndpoint(endpoint),
		zipkingo.WithSharedSpans(spec.Zipkin.SameSpan),
		zipkingo.WithTraceID128Bit(spec.Zipkin.ID128Bit),
		zipkingo.WithSampler(sampler),
		zipkingo.WithTags(spec.Tags),
	)
	if err != nil {
		return nil, err
	}

	return &Tracer{
		tracer: tracer,
		closer: reporter,
	}, nil
}

// IsNoopTracer checks whether tracer is noop tracer.
func (t *Tracer) IsNoopTracer() bool {
	return t == NoopTracer
}

// Close closes Tracing.
func (t *Tracer) Close() error {
	if t.closer != nil {
		return t.closer.Close()
	}

	return nil
}

// NewSpan creates a span.
func (t *Tracer) NewSpan(name string) Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(name, fasttime.Now())
}

// NewSpanWithStart creates a span with specify start time.
func (t *Tracer) NewSpanWithStart(name string, startAt time.Time) Span {
	if t.IsNoopTracer() {
		return NoopSpan
	}
	return t.newSpanWithStart(name, startAt)
}

func (t *Tracer) newSpanWithStart(name string, startAt time.Time) Span {
	s := t.tracer.StartSpan(name, zipkingo.StartTime(startAt))
	return &span{Span: s, tracer: t}
}
