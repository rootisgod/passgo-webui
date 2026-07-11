package main

import "testing"

func TestListenWithPortPreservesConfiguredHost(t *testing.T) {
	tests := []struct {
		name   string
		listen string
		port   int
		want   string
	}{
		{name: "loopback", listen: "127.0.0.1:8080", port: 9090, want: "127.0.0.1:9090"},
		{name: "wildcard", listen: ":8080", port: 9090, want: ":9090"},
		{name: "IPv6", listen: "[::1]:8080", port: 9090, want: "[::1]:9090"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := listenWithPort(tt.listen, tt.port); got != tt.want {
				t.Fatalf("listenWithPort(%q, %d) = %q, want %q", tt.listen, tt.port, got, tt.want)
			}
		})
	}
}
