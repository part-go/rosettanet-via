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
	address string
)

func init() {
	flag.StringVar(&address, "address", ":10031", "VIA server listen address")
	flag.Parse()
}

type VIAServer struct{}

func NewVIAServer() *VIAServer {
	return &VIAServer{}
}

func (t *VIAServer) Register(ctx context.Context, req *register.RegisterReq) (*register.Boolean, error) {

	registeredTask := &proxy.RegisteredTask{TaskId: req.TaskId, PartyId: req.PartyId, ServiceType: req.ServiceType, Address: req.Address}

	log.Printf("收到注册请求：%v", req)

	//得到调用者信息，然后proxy连上它，以便后续转发数据流
	if _, ok := peer.FromContext(ctx); ok {
		//获得conn
		log.Printf("回拨local task server, %s", registeredTask.Address)
		conn, err := grpc.DialContext(ctx, registeredTask.Address, grpc.WithDefaultCallOptions(grpc.ForceCodec(proxy.Codec())), grpc.WithInsecure())

		if err != nil {
			log.Printf("回拨local task server失败")
			return &register.Boolean{Result: false}, err
		}
		log.Printf("回拨local task server成功")
		registeredTask.Conn = conn
		proxy.RegisteredTaskMap[registeredTask.TaskId+"_"+registeredTask.PartyId] = registeredTask

		return &register.Boolean{Result: true}, nil
	} else {
		log.Printf("获取local task server地址失败")
		return &register.Boolean{Result: false}, errors.New("cannot retrieve the task info")
	}
}

func main() {
	//via提供的代理服务
	viaListener, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	//把所有服务都作为非注册服务，通过TransparentHandler来处理
	viaServer := grpc.NewServer(
		grpc.ForceServerCodec(proxy.Codec()),
		grpc.UnknownServiceHandler(proxy.TransparentHandler(proxy.GetDirector())),
	)
	//注册本身提供的服务
	register.RegisterRegisterServiceServer(viaServer, NewVIAServer())

	log.Printf("starting VIA Server at: %v", viaListener.Addr().String())
	go func() {
		viaServer.Serve(viaListener)
	}()

	waitForGracefulShutdown(viaServer)
}

func waitForGracefulShutdown(registerListener *grpc.Server) {
	interruptChan := make(chan os.Signal, 1)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	<-interruptChan

	_, cancel := context.WithTimeout(context.Background(), time.Second*10)
	defer cancel()

	registerListener.GracefulStop()

	log.Println("Shutting down Via server.")
	os.Exit(0)
}
