package internal

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
