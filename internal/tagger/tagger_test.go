package tagger

import "testing"

func TestApply(t *testing.T) {
	tags := New().Apply("Running Apache Flink on Google Cloud", "A data pipeline with BigQuery", []string{"official"})
	want := map[string]bool{"official": true, "google-cloud": true, "data-engineering": true}
	for _, tag := range tags {
		delete(want, tag)
	}
	if len(want) != 0 {
		t.Fatalf("missing tags: %#v (got %#v)", want, tags)
	}
}
