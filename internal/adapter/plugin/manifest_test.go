package plugin

import "testing"

func TestParseManifest(t *testing.T) {
	manifest, err := ParseManifest([]byte(`
id: example
name: Example
version: 1.0.0
protocol: grpc
command: example
provider_ids: [example_plugin]
`))
	if err != nil {
		t.Fatalf("ParseManifest() error = %v", err)
	}
	if manifest.ID != "example" || manifest.ProviderIDs[0] != "example_plugin" {
		t.Fatalf("manifest = %#v", manifest)
	}
	if _, err := ParseManifest([]byte(`id: nope`)); err == nil {
		t.Fatalf("expected validation error")
	}
}
