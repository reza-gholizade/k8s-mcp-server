package handlers

import (
	"encoding/json"
	"testing"
)

func TestParseHelmRevision(t *testing.T) {
	tests := []struct {
		name string
		args map[string]interface{}
		want int
		wantErr bool
	}{
		{name: "MCP JSON number", args: map[string]interface{}{"revision": float64(3)}, want: 3},
		{name: "previous revision", args: map[string]interface{}{"revision": float64(0)}, want: 0},
		{name: "integer", args: map[string]interface{}{"revision": 4}, want: 4},
		{name: "json number", args: map[string]interface{}{"revision": json.Number("5")}, want: 5},
		{name: "numeric string compatibility", args: map[string]interface{}{"revision": "6"}, want: 6},
		{name: "missing", args: map[string]interface{}{}, wantErr: true},
		{name: "negative", args: map[string]interface{}{"revision": float64(-1)}, wantErr: true},
		{name: "fractional", args: map[string]interface{}{"revision": float64(1.5)}, wantErr: true},
		{name: "wrong type", args: map[string]interface{}{"revision": true}, wantErr: true},
		{name: "invalid string", args: map[string]interface{}{"revision": "abc"}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseHelmRevision(tt.args)
			if (err != nil) != tt.wantErr {
				t.Fatalf("parseHelmRevision() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Fatalf("parseHelmRevision() = %d, want %d", got, tt.want)
			}
		})
	}
}
