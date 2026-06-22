package main

import (
	"testing"
)

func TestIsTimeInWindow(t *testing.T) {
	tests := []struct {
		name     string
		timeStr  string
		start    string
		end      string
		expected bool
	}{
		{
			name:     "Standard daytime window - inside",
			timeStr:  "12:00",
			start:    "06:00",
			end:      "22:00",
			expected: true,
		},
		{
			name:     "Standard daytime window - exactly start",
			timeStr:  "06:00",
			start:    "06:00",
			end:      "22:00",
			expected: true,
		},
		{
			name:     "Standard daytime window - exactly end",
			timeStr:  "22:00",
			start:    "06:00",
			end:      "22:00",
			expected: true,
		},
		{
			name:     "Standard daytime window - outside",
			timeStr:  "02:00",
			start:    "06:00",
			end:      "22:00",
			expected: false,
		},
		{
			name:     "Cross-midnight window - inside late",
			timeStr:  "23:00",
			start:    "22:00",
			end:      "06:00",
			expected: true,
		},
		{
			name:     "Cross-midnight window - inside early",
			timeStr:  "01:00",
			start:    "22:00",
			end:      "06:00",
			expected: true,
		},
		{
			name:     "Cross-midnight window - exactly start",
			timeStr:  "22:00",
			start:    "22:00",
			end:      "06:00",
			expected: true,
		},
		{
			name:     "Cross-midnight window - exactly end",
			timeStr:  "06:00",
			start:    "22:00",
			end:      "06:00",
			expected: true,
		},
		{
			name:     "Cross-midnight window - outside",
			timeStr:  "12:00",
			start:    "22:00",
			end:      "06:00",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isTimeInWindow(tt.timeStr, tt.start, tt.end)
			if result != tt.expected {
				t.Errorf("isTimeInWindow(%q, %q, %q) = %v; want %v", tt.timeStr, tt.start, tt.end, result, tt.expected)
			}
		})
	}
}
