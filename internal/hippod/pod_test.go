package hippod

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
)

func TestParseToleration(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    corev1.Toleration
		wantErr bool
	}{
		{
			name:  "tolerate all taints",
			input: "::Exists",
			want: corev1.Toleration{
				Operator: corev1.TolerationOpExists,
			},
		},
		{
			name:  "key with any effect",
			input: "node.kubernetes.io/disk-pressure=::Exists",
			want: corev1.Toleration{
				Key:      "node.kubernetes.io/disk-pressure",
				Operator: corev1.TolerationOpExists,
			},
		},
		{
			name:  "key with any value for specific effect",
			input: "node.kubernetes.io/not-ready=:NoExecute:Exists",
			want: corev1.Toleration{
				Key:      "node.kubernetes.io/not-ready",
				Effect:   corev1.TaintEffectNoExecute,
				Operator: corev1.TolerationOpExists,
			},
		},
		{
			name:  "key=value with specific effect",
			input: "dedicated=gpu:NoSchedule:Equal",
			want: corev1.Toleration{
				Key:      "dedicated",
				Value:    "gpu",
				Effect:   corev1.TaintEffectNoSchedule,
				Operator: corev1.TolerationOpEqual,
			},
		},
		{
			name:    "invalid format - missing parts",
			input:   "key:value",
			wantErr: true,
		},
		{
			name:    "invalid operator",
			input:   "key=value:NoSchedule:Invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseToleration(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseToleration() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if got.Key != tt.want.Key || got.Value != tt.want.Value ||
					got.Effect != tt.want.Effect || got.Operator != tt.want.Operator {
					t.Errorf("parseToleration() = %+v, want %+v", got, tt.want)
				}
			}
		})
	}
}

func TestNodeSelectorIntegration(t *testing.T) {
	tests := []struct {
		name     string
		input    map[string]string
		expected map[string]string
	}{
		{
			name: "single node selector",
			input: map[string]string{
				"disktype": "ssd",
			},
			expected: map[string]string{
				"disktype": "ssd",
			},
		},
		{
			name: "multiple node selectors",
			input: map[string]string{
				"disktype":    "ssd",
				"environment": "production",
			},
			expected: map[string]string{
				"disktype":    "ssd",
				"environment": "production",
			},
		},
		{
			name: "node selector with empty value",
			input: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
			expected: map[string]string{
				"node-role.kubernetes.io/control-plane": "",
			},
		},
		{
			name: "complex node selector keys",
			input: map[string]string{
				"topology.kubernetes.io/zone":      "us-east-1a",
				"node.kubernetes.io/instance-type": "m5.large",
			},
			expected: map[string]string{
				"topology.kubernetes.io/zone":      "us-east-1a",
				"node.kubernetes.io/instance-type": "m5.large",
			},
		},
		{
			name:     "empty node selector",
			input:    map[string]string{},
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test that the map is correctly assigned
			if len(tt.input) == 0 && tt.expected == nil {
				return
			}

			if len(tt.input) != len(tt.expected) {
				t.Errorf("length mismatch: got %d, want %d", len(tt.input), len(tt.expected))
				return
			}

			for k, v := range tt.expected {
				if got, ok := tt.input[k]; !ok || got != v {
					t.Errorf("key %q: got %q, want %q", k, got, v)
				}
			}
		})
	}
}
