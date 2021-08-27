package creds

import (
	"github.com/Hyperledger-TWGC/ccs-gm/tls"
	"github.com/Hyperledger-TWGC/ccs-gm/x509"
	"github.com/bglmmz/grpc/credentials"
	"io/ioutil"
	"log"
)

func NewServerTLSOneWay(serverCertFile, serverKeyFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewServerTLSOneWay")
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.NoClientCert,
		//InsecureSkipVerify: true,
	}), nil
}

func NewClientTLSOneWay(serverCaCertFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewClientTLSOneWay")
	return credentials.NewTLS(&tls.Config{
		RootCAs: loadCaPool(serverCaCertFile),
	}), nil
}

func NewServerTLSTwoWay(clientCaCertFile, serverCertFile, serverKeyFile string) (credentials.TransportCredentials, error) {
	serverCert, err := tls.LoadX509KeyPair(serverCertFile, serverKeyFile)
	if err != nil {
		return nil, err
	}

	log.Printf("NewServerGMTLSOneWay GMSupport")
	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverCert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    loadCaPool(clientCaCertFile),
		//InsecureSkipVerify: true,
	}), nil
}

func NewClientTLSTwoWay(serverCaCertFile, clientCertFile, clientKeyFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewClientTLSTwoWay")

	clientCert, err := tls.LoadX509KeyPair(clientCertFile, clientKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientCert},
		RootCAs:      loadCaPool(serverCaCertFile),
		//InsecureSkipVerify: true,
	}), nil
}

func NewServerGMTLSOneWay(serverSignCertFile, serverSignKeyFile, serverCipherCertFile, serverCipherKeyFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewServerGMTLSOneWay")

	serverSignCert, err := tls.LoadX509KeyPair(serverSignCertFile, serverSignKeyFile)
	if err != nil {
		return nil, err
	}
	serverCipherCert, err := tls.LoadX509KeyPair(serverCipherCertFile, serverCipherKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverSignCert, serverCipherCert},
		GMSupport:    &tls.GMSupport{},
		ClientAuth:   tls.NoClientCert,
	}), nil
}

func NewClientGMTLSOneWay(serverCaCertFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewClientGMTLSOneWay")
	return credentials.NewTLS(&tls.Config{
		GMSupport:  &tls.GMSupport{},
		RootCAs:    loadCaPool(serverCaCertFile),
		ClientAuth: tls.NoClientCert,
	}), nil
}

func NewServerGMTLSTwoWay(clientCaCertFile, serverSignCertFile, serverSignKeyFile, serverCipherCertFile, serverCipherKeyFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewServerGMTLSTwoWay")

	serverSignCert, err := tls.LoadX509KeyPair(serverSignKeyFile, serverSignCertFile)
	if err != nil {
		return nil, err
	}

	serverCipherCert, err := tls.LoadX509KeyPair(serverCipherCertFile, serverCipherKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{serverSignCert, serverCipherCert},
		GMSupport:    &tls.GMSupport{},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    loadCaPool(clientCaCertFile),
		//InsecureSkipVerify: true,
	}), nil
}

func NewClientGMTLSTwoWay(serverCaCertFile, clientSignCertFile, clientSignKeyFile, clientCipherCertFile, clientCipherKeyFile string) (credentials.TransportCredentials, error) {
	log.Printf("NewClientTLSTwoWay")

	clientSignCert, err := tls.LoadX509KeyPair(clientSignCertFile, clientSignKeyFile)
	if err != nil {
		return nil, err
	}

	clientCipherCert, err := tls.LoadX509KeyPair(clientCipherCertFile, clientCipherKeyFile)
	if err != nil {
		return nil, err
	}

	return credentials.NewTLS(&tls.Config{
		Certificates: []tls.Certificate{clientSignCert, clientCipherCert},
		GMSupport:    &tls.GMSupport{},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		RootCAs:      loadCaPool(serverCaCertFile),
		//InsecureSkipVerify: true,
	}), nil
}

func loadCaPool(caCertFile string) *x509.CertPool {
	pemServerCA, err := ioutil.ReadFile(caCertFile)
	if err != nil {
		log.Fatalf("failed to read CA cert file. %v", err)
	}

	cert, err := x509.Pem2Cert(pemServerCA)
	if err != nil {
		log.Fatalf("failed to add CA cert to cert pool. %v", err)
		return nil
	}
	cp := x509.NewCertPool()
	cp.AddCert(cert)
	return cp
}
