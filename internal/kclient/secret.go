package kclient

import (
	"crypto/x509"
	"encoding/pem"
	"time"
)

// SearchExpiries looks for PEM encoded certs and returns the 'not after' times.
func SearchExpiries(data map[string][]byte) (map[string]time.Time, error) {
	results := map[string]time.Time{}
	for f, d := range data {
		block, _ := pem.Decode(d)
		if block == nil {
			// not PEM formatted
			continue
		}

		if cer, err := x509.ParseCertificate(block.Bytes); err == nil {
			results[f] = cer.NotAfter
		}
		// TODO try other formats
	}

	return results, nil
}
