package pkg

import (
	"testing"

	v0alpha0 "sigs.k8s.io/wg-ai-gateway/prototypes/payload-processor/api/v0alpha0"
)

func TestComposeBodyExpression(t *testing.T) {
	tests := []struct {
		name     string
		in       v0alpha0.InProcessTransform
		expected string
	}{
		{
			name:     "no body fields",
			in:       v0alpha0.InProcessTransform{},
			expected: "",
		},
		{
			name: "set single field",
			in: v0alpha0.InProcessTransform{
				SetBodyFields: []v0alpha0.BodyFieldTransformation{
					{Name: "$.stream", Value: "true"},
				},
			},
			expected: `toJson(json(request.body).merge({"stream": true}))`,
		},
		{
			name: "set multiple fields preserves order",
			in: v0alpha0.InProcessTransform{
				SetBodyFields: []v0alpha0.BodyFieldTransformation{
					{Name: "$.stream", Value: "true"},
					{Name: "$.model", Value: "json(request.body).model"},
				},
			},
			expected: `toJson(json(request.body).merge({"stream": true, "model": json(request.body).model}))`,
		},
		{
			name: "remove single field",
			in: v0alpha0.InProcessTransform{
				RemoveBodyFields: []v0alpha0.BodyFieldRemoval{
					{Name: "$.user_email"},
				},
			},
			expected: `toJson(json(request.body).filterKeys(k, !(k in ["user_email"])))`,
		},
		{
			name: "set and remove combined",
			in: v0alpha0.InProcessTransform{
				SetBodyFields: []v0alpha0.BodyFieldTransformation{
					{Name: "$.stream", Value: "true"},
				},
				RemoveBodyFields: []v0alpha0.BodyFieldRemoval{
					{Name: "$.user_email"},
					{Name: "$.phone_number"},
				},
			},
			expected: `toJson(json(request.body).filterKeys(k, !(k in ["user_email", "phone_number"])).merge({"stream": true}))`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := composeBodyExpression(&tt.in)
			if got != tt.expected {
				t.Errorf("composeBodyExpression() =\n  %q\nwant\n  %q", got, tt.expected)
			}
		})
	}
}

func TestConvertTransform_BodyFields(t *testing.T) {
	in := &v0alpha0.InProcessTransform{
		SetHeaders: []v0alpha0.HeaderTransformation{
			{Name: "X-Gateway-Model-Name", Value: "json(request.body).model"},
		},
		SetBodyFields: []v0alpha0.BodyFieldTransformation{
			{Name: "$.stream", Value: "true"},
		},
		RemoveBodyFields: []v0alpha0.BodyFieldRemoval{
			{Name: "$.user_email"},
		},
	}

	got := convertTransform(in)
	if got == nil {
		t.Fatal("convertTransform() returned nil, want non-nil transform")
	}

	if len(got.Set) != 1 {
		t.Fatalf("expected 1 header set, got %d", len(got.Set))
	}
	if got.Set[0].Name != "X-Gateway-Model-Name" {
		t.Errorf("unexpected header name: %q", got.Set[0].Name)
	}

	if got.Body == nil {
		t.Fatal("expected Body transformation, got nil")
	}
	want := `toJson(json(request.body).filterKeys(k, !(k in ["user_email"])).merge({"stream": true}))`
	if got.Body.Expression != want {
		t.Errorf("Body.Expression =\n  %q\nwant\n  %q", got.Body.Expression, want)
	}
}

func TestConvertTransform_RemoveHeaders(t *testing.T) {
	in := &v0alpha0.InProcessTransform{
		RemoveHeaders: []v0alpha0.HeaderName{"X-Internal-Token"},
	}

	got := convertTransform(in)
	if got == nil {
		t.Fatal("convertTransform() returned nil, want non-nil transform")
	}
	if len(got.Remove) != 1 || got.Remove[0] != "X-Internal-Token" {
		t.Errorf("unexpected remove headers: %+v", got.Remove)
	}
}

func TestConvertTransform_HeadersOnlyNoBody(t *testing.T) {
	in := &v0alpha0.InProcessTransform{
		SetHeaders: []v0alpha0.HeaderTransformation{
			{Name: "X-Gateway-Model-Name", Value: "json(request.body).model"},
		},
	}

	got := convertTransform(in)
	if got == nil {
		t.Fatal("convertTransform() returned nil, want non-nil transform")
	}
	if got.Body != nil {
		t.Errorf("expected no Body transformation, got %+v", got.Body)
	}
}
