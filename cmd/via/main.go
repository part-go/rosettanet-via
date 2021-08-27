package main

import (
	"errors"
	"flag"
	"github.com/bglmmz/grpc"
	"github.com/bglmmz/grpc/credentials"
	"github.com/bglmmz/grpc/peer"
	"golang.org/x/net/context"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"via/conf"
	"via/creds"
	"via/proxy"
	"via/via"
)

var (
	address    string
	sslFile    string
	sslEnabled = false
)

func init() {
	flag.StringVar(&address, "address", ":10031", "VIA service listen address")
	flag.StringVar(&sslFile, "ssl", "", "SSL config file")
	flag.Parse()

	if len(sslFile) > 0 {
		sslEnabled = true
	}
}

type viaServer struct{}

func newVIAServer() *viaServer {
	return &viaServer{}
}

func (t *viaServer) Signup(ctx context.Context, req *via.SignupReq) (*via.Boolean, error) {

	registeredTask := &proxy.SignupTask{TaskId: req.TaskId, PartyId: req.PartyId, ServiceType: req.ServiceType, Address: req.Address}

	log.Printf("收到注册请求：%v", req)

	//得到调用者信息，然后proxy连上它，以便后续转发数据流
	if _, ok := peer.FromContext(ctx); ok {
		//获得conn

		var conn *grpc.ClientConn
		var err error
		if sslEnabled {
			log.Printf("回拨local task server with secure, %s", registeredTask.Address)
			conn, err = grpc.DialContext(ctx, registeredTask.Address, grpc.WithCodec(proxy.Codec()), grpc.WithTransportCredentials(tlsCredentialsAsClient))
		} else {
			log.Printf("回拨local task server with insecure, %s", registeredTask.Address)
			conn, err = grpc.DialContext(ctx, registeredTask.Address, grpc.WithCodec(proxy.Codec()), grpc.WithInsecure())
		}

		if err != nil {
			log.Printf("回拨local task server失败")
			return &via.Boolean{Result: false}, err
		}

		//用taskId_partyId作为请求者的唯一标识，把conn保存到map中。
		registeredTask.Conn = conn
		proxy.RegisteredTaskMap[registeredTask.TaskId+"_"+registeredTask.PartyId] = registeredTask

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
	if sslEnabled {
		log.Printf("starting VIA Server with secure at: %s", address)
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		viaServer = grpc.NewServer(
			grpc.Creds(tlsCredentialsAsServer),
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
		//encoding.RegisterCodec(proxy.Codec())
	} else {
		log.Printf("starting VIA Server with insecure at: %s", address)
		//把所有服务都作为非注册服务，通过TransparentHandler来处理
		viaServer = grpc.NewServer(
			grpc.CustomCodec(proxy.Codec()),
			grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
		)
		//encoding.RegisterCodec(proxy.Codec())
	}

	//注册本身提供的服务
	via.RegisterVIAServiceServer(viaServer, newVIAServer())

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

var sslConfig *conf.Config
var tlsCredentialsAsServer credentials.TransportCredentials
var tlsCredentialsAsClient credentials.TransportCredentials

func init() {
	if !sslEnabled {
		return
	}
	sslConfig = conf.LoadSSLConfig(sslFile)

	createTlsCredentials()
}

func createTlsCredentials() {
	var err error
	if sslConfig.Conf.Cipher == "ssl" {
		if sslConfig.Conf.Mode == "one_way" {
			tlsCredentialsAsServer, err = creds.NewServerTLSOneWay(sslConfig.Conf.SSL.ViaCertFile, sslConfig.Conf.SSL.ViaKeyFile)
			if err != nil {
				panic(err)
			}
			tlsCredentialsAsClient, err = creds.NewClientTLSOneWay(sslConfig.Conf.SSL.CaCertFile)
			if err != nil {
				panic(err)
			}
		} else if sslConfig.Conf.Mode == "two_way" {
			tlsCredentialsAsServer, err = creds.NewServerTLSTwoWay(sslConfig.Conf.SSL.CaCertFile, sslConfig.Conf.SSL.ViaCertFile, sslConfig.Conf.SSL.ViaKeyFile)
			if err != nil {
				panic(err)
			}
			tlsCredentialsAsClient, err = creds.NewClientTLSTwoWay(sslConfig.Conf.SSL.CaCertFile, sslConfig.Conf.SSL.ViaCertFile, sslConfig.Conf.SSL.ViaKeyFile)
			if err != nil {
				panic(err)
			}
		} else {
			log.Fatalf("error mode in ssl-conf.yml, %s", sslConfig.Conf.Mode)
		}
	} else if sslConfig.Conf.Cipher == "gmssl" {
		if sslConfig.Conf.Mode == "one_way" {
			tlsCredentialsAsServer, err = creds.NewServerGMTLSOneWay(
				sslConfig.Conf.GMSSL.ViaSignCertFile, sslConfig.Conf.GMSSL.ViaSignKeyFile,
				sslConfig.Conf.GMSSL.ViaEncryptCertFile, sslConfig.Conf.GMSSL.ViaEncryptKeyFile,
			)
			if err != nil {
				panic(err)
			}

			tlsCredentialsAsClient, err = creds.NewClientGMTLSOneWay(sslConfig.Conf.GMSSL.CaCertFile)
			if err != nil {
				panic(err)
			}

		} else if sslConfig.Conf.Mode == "two_way" {
			tlsCredentialsAsServer, err = creds.NewServerGMTLSTwoWay(
				sslConfig.Conf.GMSSL.CaCertFile,
				sslConfig.Conf.GMSSL.ViaSignCertFile, sslConfig.Conf.GMSSL.ViaSignKeyFile,
				sslConfig.Conf.GMSSL.ViaEncryptCertFile, sslConfig.Conf.GMSSL.ViaEncryptKeyFile,
			)

			if err != nil {
				panic(err)
			}

			tlsCredentialsAsClient, err = creds.NewClientGMTLSTwoWay(
				sslConfig.Conf.GMSSL.CaCertFile,
				sslConfig.Conf.GMSSL.ViaSignCertFile, sslConfig.Conf.GMSSL.ViaSignKeyFile,
				sslConfig.Conf.GMSSL.ViaEncryptCertFile, sslConfig.Conf.GMSSL.ViaEncryptKeyFile,
			)
			if err != nil {
				panic(err)
			}

		} else {
			log.Fatalf("error mode in ssl-conf.yml, %s", sslConfig.Conf.Mode)
		}
	} else {
		log.Fatalf("error cilper in ssl-conf.yml, %s", sslConfig.Conf.Mode)
	}
}
