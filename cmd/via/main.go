package main

import (
	"crypto/tls"
	"crypto/x509"
	"errors"
	"flag"
	"fmt"
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
	"via/register"
)

var (
	regAddr   string
	proxyAddr string
	tlsFile   string
)

func init() {
	flag.StringVar(&regAddr, "regAddr", ":10031", "VIA register service listen address")
	flag.StringVar(&proxyAddr, "proxyAddr", ":10032", "VIA proxy service listen address")
	flag.StringVar(&tlsFile, "tls", "../conf/tls.yml", "TLS config file")
	flag.Parse()
}

type RegisterServer struct{}

func NewRegisterServer() *RegisterServer {
	return &RegisterServer{}
}

func (t *RegisterServer) Register(ctx context.Context, req *register.RegisterReq) (*register.Boolean, error) {

	registeredTask := &proxy.RegisteredTask{TaskId: req.TaskId, PartyId: req.PartyId, ServiceType: req.ServiceType, Address: req.Address}

	log.Printf("收到注册请求：%v", req)

	//得到调用者信息，然后proxy连上它，以便后续转发数据流
	if _, ok := peer.FromContext(ctx); ok {
		//获得conn

		var conn *grpc.ClientConn
		var err error
		if tlsCredentialsAsClient == nil {
			log.Printf("回拨local task server with insecure, %s", registeredTask.Address)
			conn, err = grpc.DialContext(ctx, registeredTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithInsecure())
		} else {
			log.Printf("回拨local task server with secure, %s", registeredTask.Address)
			conn, err = grpc.DialContext(ctx, registeredTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithTransportCredentials(tlsCredentialsAsClient))
		}

		if err != nil {
			log.Printf("回拨local task server失败")
			return &register.Boolean{Result: false}, err
		}

		//用taskId_partyId作为请求者的唯一标识，把conn保存到map中。
		registeredTask.Conn = conn
		proxy.RegisteredTaskMap[registeredTask.TaskId+"_"+registeredTask.PartyId] = registeredTask

		log.Printf("回拨local task server成功")

		return &register.Boolean{Result: true}, nil
	} else {
		log.Printf("获取local task server节点信息失败")
		return &register.Boolean{Result: false}, errors.New("failed to retrieve the task server peer info")
	}
}

//启动任务注册服务，调用任务注册服务无需TLS/SSL
func startRegisterWithNoTls() *grpc.Server {
	//via提供的任务注册服务
	registerListener, err := net.Listen("tcp", regAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	registerServer := grpc.NewServer()
	//注册本身提供的任务注册服务
	register.RegisterRegisterServiceServer(registerServer, NewRegisterServer())

	go func() {
		log.Printf("starting VIA register.Server at: %v", regAddr)
		registerServer.Serve(registerListener)
	}()

	return registerServer
}

func startProxy() *grpc.Server {
	//via提供的代理服务
	proxyListener, err := net.Listen("tcp", proxyAddr)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	/*tlsCredentials, err := loadTLSCredentials()
	if err != nil {
		log.Fatal("cannot load TLS credentials: ", err)
	}*/
	var proxyServer *grpc.Server
	if tlsCredentialsAsServer == nil {
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		proxyServer = grpc.NewServer(
			grpc.Creds(tlsCredentialsAsServer),
			grpc.ForceServerCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
	} else {
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		proxyServer = grpc.NewServer(
			grpc.Creds(tlsCredentialsAsServer),
			grpc.ForceServerCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
	}

	go func() {
		log.Printf("starting VIA proxy.Server at: %v", proxyAddr)
		proxyServer.Serve(proxyListener)
	}()

	return proxyServer
}
func main() {
	waitForGracefulShutdown(startRegisterWithNoTls(), startProxy())
}

func waitForGracefulShutdown(registerServer, proxyServer *grpc.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-interruptChan

	_, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	registerServer.GracefulStop()
	proxyServer.GracefulStop()

	log.Println("Shutting down VIA server.")
	os.Exit(0)
}

var tlsCredentialsAsClient credentials.TransportCredentials
var tlsCredentialsAsServer credentials.TransportCredentials
var tlsConfig *conf.TlsConfig

func init() {
	tlsConfig = conf.LoadTlsConfig(tlsFile)

	if tlsConfig.Tls.Secure == "none" {
		return
	}

	// Load via's certificate and private key
	viaCert, err := tls.LoadX509KeyPair(tlsConfig.Tls.ViaCertFile, tlsConfig.Tls.ViaKeyFile)
	if err != nil {
		panic(fmt.Errorf("failed to load VIA certificate and private key. %v", err))
	}
	//当是SSL，拨号VIA需要携带统一的ca证书库
	caPool := loadCaPool()

	if tlsConfig.Tls.Secure == "one_way" {
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
	} else if tlsConfig.Tls.Secure == "two_way" {
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
	}
}

func loadCaPool() *x509.CertPool {
	// Load certificate of the CA who signed server's certificate
	pemServerCA, err := ioutil.ReadFile("cert/ca-cert.pem")
	if err != nil {
		log.Fatalf("failed to read CA cert file. %v", err)
	}

	caPool := x509.NewCertPool()
	if !caPool.AppendCertsFromPEM(pemServerCA) {
		log.Fatalf("failed to add CA cert to cert pool. %v", err)
	}

	return caPool
}
