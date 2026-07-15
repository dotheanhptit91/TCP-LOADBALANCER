package config

import "testing"

func TestParseMappings(t *testing.T) {
	mappings, err := ParseMappings("1000-1010=worker1@10.10.0.0/24,2000-2010=worker2@10.20.0.0/24")
	if err != nil {
		t.Fatal(err)
	}
	if len(mappings) != 2 || mappings[0].Range.Start != 1000 || mappings[1].WorkerName != "worker2" {
		t.Fatalf("unexpected mappings: %#v", mappings)
	}
}

func TestParseMappingsRejectsOverlap(t *testing.T) {
	if _, err := ParseMappings("1000-1010=a@10.10.0.0/24,1010-1020=b@10.20.0.0/24"); err == nil {
		t.Fatal("expected overlapping ranges to fail")
	}
}
