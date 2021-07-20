package main

import (
	"errors"
	"flag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/peer"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"
	"via/proxy"
	"via/register"
)

var (
	registerAddress    string
	proxyAddress string
)


func init() {
	flag.StringVar(&registerAddress, "registerAddress", ":10031", "register listen address")
	flag.StringVar(&proxyAddress, "proxyAddress", ":10031", "proxy listen address")
	flag.Parse()
}


type ProxyServer struct {}

func NewProxyServer() *ProxyServer {
	return &ProxyServer{}
}

func (t *ProxyServer) Register(ctx context.Context, req *register.RegisterReq) (*register.Boolean, error) {

	registeredTask := &proxy.RegisteredTask{TaskId: req.TaskId, ServiceType: req.ServiceType, Address: req.Address}

	log.Printf("收到注册请求：%v", req)

	//得到调用者信息，然后proxy连上它，以便后续转发数据流
	if _, ok := peer.FromContext(ctx); ok{
		//获得conn
		log.Printf("回拨local task server, %s", registeredTask.Address)
		conn, err := grpc.DialContext(ctx, registeredTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithInsecure())


		if(err!=nil){
			log.Printf("回拨local task server失败")
			return &register.Boolean{Result: false}, err
		}
		log.Printf("回拨local task server成功")
		registeredTask.Conn = conn
		proxy.RegisteredTaskMap[registeredTask.TaskId] = registeredTask

		return &register.Boolean{Result: true}, nil
	}else{
		log.Printf("获取local task server地址失败")
		return &register.Boolean{Result: false}, errors.New ("cannot retrieve the task info")
	}
}

func main(){

	//via本身提供的注册服务
	registerListener, err := net.Listen("tcp", registerAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	registerServer := grpc.NewServer()
	//注册本身提供的服务
	register.RegisterRegisterServiceServer(registerServer, NewProxyServer())

	//via提供的代理服务
	proxyListener, err := net.Listen("tcp", proxyAddress)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	//把所有服务都作为非注册服务，通过TransparentHandler来处理
	proxyServer := grpc.NewServer(
		grpc.ForceServerCodec(proxy.Codec()),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
	)

	log.Printf("starting register.Server at: %v", registerListener.Addr().String())
	go func() {
		registerServer.Serve(registerListener)
	}()

	log.Printf("starting proxy.Server at: %v", proxyListener.Addr().String())
	go func() {
		proxyServer.Serve(proxyListener)
	}()

	waitForGracefulShutdown(registerServer, proxyServer)
}


func waitForGracefulShutdown(registerServer, registerListener *grpc.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-interruptChan

	_, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()
	registerServer.GracefulStop()
	registerListener.GracefulStop()

	log.Println("Shutting down Via server.")
	os.Exit(0)
}