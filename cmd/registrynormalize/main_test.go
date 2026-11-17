package main

import "testing"

func TestNormalizeDropsRuntimeMetricsAndUnusedAttributes(t *testing.T) {
	doc := registry{
		Attributes: []attribute{{Key: "status"}, {Key: "url.scheme"}, {Key: "endpoint"}},
		Metrics: []metric{
			{Name: "up"},
			{Name: "parcels_total"},
			{Name: "http_requests_total", Attributes: []attributeRef{{Ref: "status"}, {Ref: "endpoint"}}},
		},
	}

	got := normalize(doc)

	if len(got.Metrics) != 2 || got.Metrics[0].Name != "http_requests_total" || got.Metrics[1].Name != "parcels_total" {
		t.Fatalf("metrics = %+v", got.Metrics)
	}
	if got.Metrics[0].Attributes[0].Ref != "endpoint" {
		t.Fatalf("attribute refs not sorted: %+v", got.Metrics[0].Attributes)
	}
	if len(got.Attributes) != 2 || got.Attributes[0].Key != "endpoint" || got.Attributes[1].Key != "status" {
		t.Fatalf("attributes = %+v", got.Attributes)
	}
}
