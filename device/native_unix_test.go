package device

import "testing"

func TestShouldAddNativeRoute(t *testing.T) {
	tests := []struct {
		name   string
		addr   string
		subnet string
		want   bool
	}{
		{
			name:   "skip route for connected prefix",
			addr:   "10.200.1.2/24",
			subnet: "10.200.1.0/24",
			want:   false,
		},
		{
			name:   "add route for different subnet",
			addr:   "10.200.1.2/24",
			subnet: "10.200.2.0/24",
			want:   true,
		},
		{
			name:   "add route on invalid addr",
			addr:   "10.200.1.2",
			subnet: "10.200.1.0/24",
			want:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := shouldAddNativeRoute(tt.addr, tt.subnet); got != tt.want {
				t.Fatalf("shouldAddNativeRoute(%q, %q) = %v, want %v", tt.addr, tt.subnet, got, tt.want)
			}
		})
	}
}
