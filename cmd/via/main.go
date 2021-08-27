package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/peer"
	"io/ioutil"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"via/conf"
	"via/proxy"
	"via/via"
)

var (
	address    string
	tlsFile    string
	tlsEnabled = false
)

func init() {
	flag.StringVar(&address, "address", ":10031", "VIA service listen address")
	flag.StringVar(&tlsFile, "tls", "", "TLS config file")
	flag.Parse()

	if len(tlsFile) > 0 {
		tlsEnabled = true
	}
}

type VIAServer struct{}

func NewVIAServer() *VIAServer {
	return &VIAServer{}
}

func (t *VIAServer) Signup(ctx context.Context, req *via.SignupReq) (*via.Boolean, error) {

	signupTask := &proxy.SignupTask{TaskId: req.TaskId, PartyId: req.PartyId, ServiceType: req.ServiceType, Address: req.Address}

	log.Printf("收到注册请求：%v", req)

	//得到调用者信息，然后proxy连上它，以便后续转发数据流
	if _, ok := peer.FromContext(ctx); ok {
		//获得conn

		var conn *grpc.ClientConn
		var err error
		if tlsEnabled {
			log.Printf("回拨local task server with secure, %s", signupTask.Address)
			conn, err = grpc.DialContext(ctx, signupTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithTransportCredentials(tlsCredentialsAsClient))
		} else {
			log.Printf("回拨local task server with insecure, %s", signupTask.Address)
			conn, err = grpc.DialContext(ctx, signupTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithInsecure())
		}

		if err != nil {
			log.Printf("回拨local task server失败")
			return &via.Boolean{Result: false}, err
		}

		//用taskId_partyId作为请求者的唯一标识，把conn保存到map中。
		signupTask.Conn = conn
		proxy.RegisteredTaskMap[signupTask.TaskId+"_"+signupTask.PartyId] = signupTask

		log.Printf("回拨local task server成功")

		return &via.Boolean{Result: true}, nil
	} else {
		log.Printf("获取local task server节点信息失败")
		return &via.Boolean{Result: false}, errors.New("failed to retrieve the task server peer info")
	}
}

func main() {
	//via提供的代理服务
	viaListener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var viaServer *grpc.Server
	if tlsEnabled {
		log.Printf("starting VIA Server with secure at: %s", address)
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		viaServer = grpc.NewServer(
			grpc.Creds(tlsCredentialsAsServer),
			grpc.ForceServerCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
	} else {
		log.Printf("starting VIA Server with insecure at: %s", address)
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		viaServer = grpc.NewServer(
			grpc.ForceServerCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
	}

	//注册本身提供的服务
	via.RegisterVIAServiceServer(viaServer, NewVIAServer())

	go func() {
		viaServer.Serve(viaListener)
	}()

	waitForGracefulShutdown(viaServer)
}

func waitForGracefulShutdown(viaServer *grpc.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-interruptChan

	_, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	viaServer.GracefulStop()

	log.Println("Shutting down VIA server.")
	os.Exit(0)
}

var tlsCredentialsAsClient credentials.TransportCredentials
var tlsCredentialsAsServer credentials.TransportCredentials
var tlsConfig *conf.TlsConfig

func init() {
	if !tlsEnabled {
		return
	}

	tlsConfig = conf.LoadTlsConfig(tlsFile)

	// Load via's certificate and private key
	viaCert, err := tls.LoadX509KeyPair(tlsConfig.Tls.ViaCertFile, tlsConfig.Tls.ViaKeyFile)
	if err != nil {
		log.Fatalf("failed to load VIA certificate and private key. %v", err)
	}
	//当是SSL，拨号VIA需要携带统一的ca证书库
	caPool := loadCaPool()

	if tlsConfig.Tls.Mode == "one_way" {
		log.Printf("VIA单向SSL")
		serverSSLConfig := &tls.Config{
			Certificates: []tls.Certificate{viaCert},
			ClientAuth:   tls.NoClientCert,
		}
		tlsCredentialsAsServer = credentials.NewTLS(serverSSLConfig)

		clientSSLConfig := &tls.Config{
			RootCAs: caPool,
		}
		tlsCredentialsAsClient = credentials.NewTLS(clientSSLConfig)
	} else if tlsConfig.Tls.Mode == "two_way" {
		log.Printf("VIA双向SSL")
		serverSSLConfig := &tls.Config{
			//InsecureSkipVerify: true, //不校验证书有效性
			Certificates: []tls.Certificate{viaCert},
			ClientAuth:   tls.RequireAndVerifyClientCert,
			ClientCAs:    loadCaPool(),
		}
		tlsCredentialsAsServer = credentials.NewTLS(serverSSLConfig)

		clientSSLConfig := &tls.Config{
			Certificates: []tls.Certificate{viaCert},
			RootCAs:      caPool,
		}
		tlsCredentialsAsClient = credentials.NewTLS(clientSSLConfig)
	} else {
		log.Fatalf("Tls.Mode value error: %s", tlsConfig.Tls.Mode)
	}
}

func loadCaPool() *x509.CertPool {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("cert/ca.crt")
	if err != nil {
		log.Fatalf("failed to read CA cert file. %v", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(pemServerCA) {
		log.Fatalf("failed to add CA cert to cert pool. %v", err)
	}

	return caPool
}
