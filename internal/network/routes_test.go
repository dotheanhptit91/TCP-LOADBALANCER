package network

import "testing"

func TestParseRoutes(t *testing.T) {
	routes, err := ParseRoutes("10.30.0.0/24=10.10.0.2,10.40.0.0/24=10.10.0.3")
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 || routes[0].Gateway.String() != "10.10.0.2" {
		t.Fatalf("unexpected routes: %#v", routes)
	}
}
