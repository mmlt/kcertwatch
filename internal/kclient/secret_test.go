package kclient

import (
	"github.com/mmlt/kcertwatch/internal/testdata"
	"github.com/stretchr/testify/assert"
	"testing"
	"time"
)

func Test_SearchExpiries(t *testing.T) {
	type args struct {
		data map[string][]byte
	}
	tests := []struct {
		name string
		args args
		want map[string]time.Time
	}{
		{"", args{data: testdata.Certs}, testdata.CertsNotAfterTimes},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := SearchExpiries(tt.args.data)
			if err != nil {
				t.Errorf("SearchExpiries() error = %v", err)
				return
			}
			assert.Equal(t, tt.want, got)
		})
	}
}
