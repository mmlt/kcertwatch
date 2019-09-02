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
		//TODO
		//buf := make([]byte, len(d))
		//n, err := base64.StdEncoding.Decode(buf, d)
		//if err != nil {
		//	return nil, fmt.Errorf("field %v: base64 decode failed", f)
		//}

		block, _ := pem.Decode(d) //buf[:n])
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
