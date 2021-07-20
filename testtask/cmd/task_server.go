package main

import (
	"flag"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"log"
	"net"
	"strconv"
	"time"
	"via/register"
	"via/testtask"
)

var (
	address    string
	registerAddress	string
	destProxyAddress	string
	partner 	string
)


const DefaultTaskId string = "testTaskId"

func init() {
	flag.StringVar(&partner, "partner", "partner_1", "partner")
	flag.StringVar(&address, "address", ":10040", "listen address")
	flag.StringVar(&registerAddress, "registerAddress", ":10030", "register address")
	flag.StringVar(&destProxyAddress, "destProxyAddress", ":20031", "dest proxy address")
	flag.Parse()
}


// echoServer is used to implement a chat.EchoServer
type echoServer struct {
	testtask.UnimplementedEchoServer
}
func (s *echoServer) Replay(ctx context.Context, in *testtask.EchoRequest) (*testtask.EchoReply, error) {
	log.Printf("收到请求: %v", in.GetMessage())
	return &testtask.EchoReply{Message: partner + " 的回复::::::" + in.GetMessage()}, nil
}


func registerTask(conn *grpc.ClientConn, args []string) error {
	c := register.NewRegisterServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	r, err := c.Register(ctx, &register.RegisterReq{TaskId: DefaultTaskId, Address: address})
	if err != nil {
		return err
	}

	log.Printf("Register task result: %v", r.Result)
	return nil
}

func echoLoop() {
	log.Printf("拨号dest proxy, %v", destProxyAddress)
	conn, err := grpc.Dial(destProxyAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect to self proxy: %v", err)
	}
	defer conn.Close()
	for i:=0; i<100; i++{
		echo(conn, []string{strconv.Itoa(i)})
		time.Sleep(time.Duration(2)*time.Second)
	}
}

func echo(conn *grpc.ClientConn, args []string) error {
	c := testtask.NewEchoClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	message :="this is from partner: " + partner + ", biz data: " + args[0]
	//把taskId放入 metadata中，key=task_ID
	log.Printf("发送请求：destProxy: %v, message: %s", destProxyAddress, message)

	ctx = metadata.AppendToOutgoingContext(ctx, "task_ID", DefaultTaskId)
	r, err := c.Replay(ctx, &testtask.EchoRequest{Message: "this is from partner: " + partner + ", biz data: " + args[0]})
	if err != nil {
		return err
	}

	log.Printf("收到响应: %v", r.GetMessage())
	return nil
}


func main() {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var s *grpc.Server

	log.Print("running insecure!")
	s = grpc.NewServer()

	// Register TASK API
	testtask.RegisterEchoServer(s, &echoServer{})
	reflection.Register(s)
	log.Printf("Listening on %v", address)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	//task任务启动后，首先连接本地proxy，并把任务注册到本地proxy
	log.Printf("dial to local register server on %v", registerAddress)
	conn, err := grpc.Dial(registerAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect to local register server: %v", err)
	}
	defer conn.Close()

	log.Printf("register task to local register server %v", registerAddress)
	registerTask(conn, nil)


	echoLoop()

}