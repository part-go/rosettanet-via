package conf

import (
	"testing"
)

func TestProxy(t *testing.T) {
	tlsConfig := LoadTlsConfig("./tls.yml")
	t.Log(tlsConfig.Tls.Mode)
}
