package conf

import (
	"testing"
)

func TestProxy(t *testing.T) {
	tlsConfig := LoadTlsConfig("cert/tls.yml")
	t.Log(tlsConfig.Tls.Secure)
}
