package config

import (
	"encoding/json"
	"testing"
)

func TestResponseCompressionConfig(t *testing.T) {
	for _, tc := range []struct {
		name     string
		input    string
		enabled  bool
		minBytes int
	}{
		{name: "defaults", input: "{}", enabled: false, minBytes: 1024},
		{name: "disabled", input: "response-compression:\n  enabled: false\n  min-bytes: 64", minBytes: 64},
		{name: "enabled", input: "response-compression:\n  enabled: true\n  min-bytes: -1", enabled: true, minBytes: 1024},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := ParseConfigBytes([]byte(tc.input))
			if err != nil {
				t.Fatal(err)
			}
			eff := cfg.EffectiveResponseCompression()
			if got := eff.IsEnabled(); got != tc.enabled {
				t.Fatalf("IsEnabled() = %v, want %v", got, tc.enabled)
			}
			if got := eff.EffectiveMinBytes(); got != tc.minBytes {
				t.Fatalf("EffectiveMinBytes() = %d, want %d", got, tc.minBytes)
			}
			encoded, errMarshal := json.Marshal(cfg)
			if errMarshal != nil {
				t.Fatal(errMarshal)
			}
			var roundTrip Config
			if errUnmarshal := json.Unmarshal(encoded, &roundTrip); errUnmarshal != nil {
				t.Fatal(errUnmarshal)
			}
			roundTripEff := roundTrip.EffectiveResponseCompression()
			if roundTripEff.IsEnabled() != tc.enabled || roundTripEff.EffectiveMinBytes() != tc.minBytes {
				t.Fatal("JSON round trip changed compression configuration")
			}
		})
	}
}
