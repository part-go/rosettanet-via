package conf

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type TlsConfig struct {
	Tls *Tls `yaml:"tls"`
}

type Tls struct {
	Mode        string `yaml:"mode"`
	ViaCertFile string `yaml:"viaCertFile"`
	ViaKeyFile  string `yaml:"viaKeyFile"`
	IoCertFile  string `yaml:"ioCertFile"`
	IoKeyFile   string `yaml:"ioKeyFile"`
	CaCertFile  string `yaml:"caCertFile"`
}

func LoadTlsConfig(configFile string) *TlsConfig {
	buf, err := ioutil.ReadFile(configFile)
	if err != nil {
		panic(fmt.Errorf("load TLS config file error. %v", err))
	}

	c := &TlsConfig{}
	err = yaml.Unmarshal(buf, c)
	if err != nil {
		panic(fmt.Errorf("load TLS config file error. %v", err))
	}
	return c
}
