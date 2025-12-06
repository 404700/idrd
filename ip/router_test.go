package ip

import (
	"testing"
)

func TestParseIP_RouterOS(t *testing.T) {
	r := &RouterProvider{Type: "routeros"}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "RouterOS CIDR format",
			input:    "192.168.1.100/24",
			expected: "192.168.1.100",
			wantErr:  false,
		},
		{
			name:     "RouterOS plain IP",
			input:    "10.0.0.1",
			expected: "10.0.0.1",
			wantErr:  false,
		},
		{
			name:     "RouterOS with newlines",
			input:    "\n192.168.1.1/24\n",
			expected: "192.168.1.1",
			wantErr:  false,
		},
		{
			name:     "RouterOS old format with address=",
			input:    "address=172.16.0.1/24 interface=ether1",
			expected: "172.16.0.1",
			wantErr:  false,
		},
		{
			name:     "Empty output",
			input:    "",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Invalid output",
			input:    "no ip found",
			expected: "",
			wantErr:  true,
		},
		{
			name:     "Public IP CIDR",
			input:    "203.0.113.45/32",
			expected: "203.0.113.45",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.parseIP(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestParseIP_OpenWrt(t *testing.T) {
	r := &RouterProvider{Type: "openwrt"}

	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "OpenWrt plain IP",
			input:    "192.168.1.1",
			expected: "192.168.1.1",
			wantErr:  false,
		},
		{
			name:     "OpenWrt with newlines",
			input:    "\n10.0.0.1\n",
			expected: "10.0.0.1",
			wantErr:  false,
		},
		{
			name:     "OpenWrt public IP",
			input:    "203.0.113.86",
			expected: "203.0.113.86",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := r.parseIP(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Errorf("Expected error, but got none")
				}
				return
			}
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestCleanPEMKey(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "PEM with YAML indentation",
			input: `    -----BEGIN OPENSSH PRIVATE KEY-----
    b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
    -----END OPENSSH PRIVATE KEY-----`,
			expected: `-----BEGIN OPENSSH PRIVATE KEY-----
b3BlbnNzaC1rZXktdjEAAAAABG5vbmUAAAAEbm9uZQAAAAAAAAABAAAAMwAAAAtzc2gtZW
-----END OPENSSH PRIVATE KEY-----`,
		},
		{
			name: "Clean PEM no changes needed",
			input: `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA
-----END RSA PRIVATE KEY-----`,
			expected: `-----BEGIN RSA PRIVATE KEY-----
MIIEpAIBAAKCAQEA
-----END RSA PRIVATE KEY-----`,
		},
		{
			name:     "Empty lines removed",
			input:    "-----BEGIN PRIVATE KEY-----\n\n\nbase64data\n\n-----END PRIVATE KEY-----",
			expected: "-----BEGIN PRIVATE KEY-----\nbase64data\n-----END PRIVATE KEY-----",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := cleanPEMKey(tt.input)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestRouterProvider_GetIPv6(t *testing.T) {
	r := &RouterProvider{}

	// IPv6 目前未实现，应返回空
	ip, source, err := r.GetIPv6()
	if err != nil {
		t.Errorf("GetIPv6 should not return error, got: %v", err)
	}
	if ip != "" {
		t.Errorf("GetIPv6 should return empty IP, got: %s", ip)
	}
	if source != "" {
		t.Errorf("GetIPv6 should return empty source, got: %s", source)
	}
}

func TestSTUNProvider_GetIPv6(t *testing.T) {
	s := &STUNProvider{}

	// IPv6 目前未实现，应返回空
	ip, source, err := s.GetIPv6()
	if err != nil {
		t.Errorf("GetIPv6 should not return error, got: %v", err)
	}
	if ip != "" {
		t.Errorf("GetIPv6 should return empty IP, got: %s", ip)
	}
	if source != "" {
		t.Errorf("GetIPv6 should return empty source, got: %s", source)
	}
}
