package managementasset

import "embed"

//go:embed all:dist
var embeddedDistFS embed.FS

// EmbeddedHTML returns the single-file management control panel HTML embedded
// into the binary at build time, or nil when built without a frontend bundle.
func EmbeddedHTML() []byte {
	data, err := embeddedDistFS.ReadFile("dist/" + managementAssetName)
	if err != nil || len(data) == 0 {
		return nil
	}
	return data
}
