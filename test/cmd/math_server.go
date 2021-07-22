package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/reflection"
	"io"
	"log"
	"math/big"
	"net"
	"via/proxy"

	"time"
	"via/register"
	"via/test"
)

var (
	address          string
	registerAddress  string
	destProxyAddress string
	partner          string
	commands         map[string]Command
)

// A Command is the API for a sub-command
type Command func(*mathServer) error

const DefaultTaskId string = "testTaskId"

func init() {
	flag.StringVar(&partner, "partner", "partner_1", "partner")
	flag.StringVar(&address, "address", ":10040", "listen address")
	flag.StringVar(&registerAddress, "registerAddress", ":10030", "register address")
	flag.StringVar(&destProxyAddress, "destProxyAddress", ":20031", "dest proxy address")
	flag.Parse()

	commands = map[string]Command{
		"unary":           unary,
		"serverStreaming": serverStreaming,
		"clientStreaming": clientStreaming,
		"bidi":            bidi,
	}
}

type mathServer struct {
	ctx    context.Context
	client test.MathServiceClient
}

func (s *mathServer) init() {
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, proxy.MetadataKey, DefaultTaskId)
	s.ctx = ctx
	if conn, err := grpc.Dial(destProxyAddress, grpc.WithInsecure()); err != nil {
		log.Fatalf("did not connect to dest proxy: %v", err)
	} else {
		s.client = test.NewMathServiceClient(conn)
	}
}

func (s *mathServer) Sum_Unary(ctx context.Context, metricList *test.MetricList) (*test.SumResponse, error) {
	log.Printf("服务(unary)：求列表之和：%v", metricList.Metric)
	var sum int64
	for _, metric := range metricList.Metric {
		sum += metric
	}
	log.Printf("计算服务结束(unary)：求列表之和：sum%v=%d", metricList, sum)
	return &test.SumResponse{Count: int32(len(metricList.Metric)), Val: sum}, nil
}

func (s *mathServer) Sum_ServerStreaming(metricList *test.MetricList, stream test.MathService_Sum_ServerStreamingServer) error {
	log.Printf("服务(serverStreaming)：求列表之和：%v", metricList.Metric)
	var sum int64
	for _, metric := range metricList.Metric {
		sum += metric
		log.Printf("服务(serverStreaming)：求列表之和, temp sum：%d", sum)
		if err := stream.Send(&test.SumResponse{Count: int32(len(metricList.Metric)), Val: sum}); err != nil {
			return err
		}
	}
	log.Printf("计算服务结束(serverStreaming)：求列表之和：sum%v=%d", metricList, sum)
	return nil
}

func (s *mathServer) Sum_ClientStreaming(stream test.MathService_Sum_ClientStreamingServer) error {
	var metricList []int64
	var count int32
	var sum int64
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			log.Printf("计算服务结束(clientStreaming)：求列表之和：sum%v=%d", metricList, sum)
			return stream.SendAndClose(&test.SumResponse{Count: count, Val: sum})
		}
		if err != nil {
			return err
		}
		count++
		sum += r.Metric
		metricList = append(metricList, r.Metric)
		log.Printf("服务(clientStreaming)：求列表之和, 收到第 %d 个数字: %d", count, r.Metric)
	}
}

func (s *mathServer) Sum_BidiStreaming(stream test.MathService_Sum_BidiStreamingServer) error {
	var metricList []int64
	var count int32
	var sum int64
	for {
		r, err := stream.Recv()
		if err == io.EOF {
			log.Printf("计算服务结束(bidi)：求列表之和：sum(%v)=%d", metricList, sum)
			return nil
		}
		if err != nil {
			return err
		}

		count++
		sum += r.Metric
		metricList = append(metricList, r.Metric)
		log.Printf("服务(bidi)：求列表之和, 收到第 %d 个数字:%d, sum：%d", count, r.Metric, sum)
		err = stream.Send(&test.SumResponse{Count: count, Val: sum})
		if err != nil {
			log.Fatalf("给对方发送计算结果出错, err: %v", err)
			return err
		}
	}
}

func registerTask() error {
	log.Printf("dial to local register server on %v", registerAddress)
	conn, err := grpc.Dial(registerAddress, grpc.WithInsecure())
	if err != nil {
		log.Fatalf("did not connect to local register server: %v", err)
	}
	defer conn.Close()

	log.Printf("register task to local register server %v", registerAddress)

	c := register.NewRegisterServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r, err := c.Register(ctx, &register.RegisterReq{TaskId: DefaultTaskId, Address: address})
	if err != nil {
		return err
	}

	log.Printf("Register task result: %v", r.Result)
	return nil
}

func randMetricList() []int64 {
	countBigInt, _ := rand.Int(rand.Reader, big.NewInt(20))
	count := int(countBigInt.Int64())
	metricList := make([]int64, count)
	for i := 0; i < count; i++ {
		metric, _ := rand.Int(rand.Reader, big.NewInt(1000))
		metricList[i] = metric.Int64()
	}
	return metricList
}

func unary(s *mathServer) error {
	metricList := randMetricList()
	log.Printf("客户端请求：求列表之和(unary)：%v", metricList)
	resp, err := s.client.Sum_Unary(s.ctx, &test.MetricList{Metric: metricList})
	if err != nil {
		return err
	}
	log.Printf("unary resp, count: %d, sum: %d", resp.Count, resp.Val)
	return nil
}

func serverStreaming(s *mathServer) error {
	metricList := randMetricList()
	log.Printf("客户端请求：求列表之和(serverStreaming)：%v", metricList)
	//发送100个数字
	stream, err := s.client.Sum_ServerStreaming(s.ctx, &test.MetricList{Metric: metricList})
	if err != nil {
		return err
	}
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		log.Printf("serverStreaming resp, count: %d, sum: %d", resp.Count, resp.Val)
	}
	return nil
}

func clientStreaming(s *mathServer) error {
	stream, err := s.client.Sum_ClientStreaming(s.ctx)
	if err != nil {
		return err
	}
	metricList := randMetricList()
	log.Printf("客户端请求：求列表之和(clientStreaming)：%v", metricList)
	for i := 0; i < len(metricList); i++ {
		metric := metricList[i]
		log.Printf("向对方发送第 %d 个数字：%d", i+1, metric)
		err := stream.Send(&test.Metric{Metric: metric})
		if err != nil {
			return err
		}
	}
	resp, err := stream.CloseAndRecv()
	if err != nil {
		return err
	}

	log.Printf("clientStreaming resp, count: %d, sum: %d", resp.Count, resp.Val)
	return nil
}

func bidi(s *mathServer) error {
	stream, err := s.client.Sum_BidiStreaming(s.ctx)
	if err != nil {
		return err
	}

	metricList := randMetricList()
	log.Printf("客户端请求：求列表之和(bidi)：%v", metricList)

	for i := 0; i < len(metricList); i++ {
		metric := metricList[i]
		log.Printf("向对方发送第 %d 个数字：%d", i+1, metric)
		err = stream.Send(&test.Metric{Metric: metric})
		if err != nil {
			log.Fatalf("向对方发送数字出错, %v", err)
			return err
		}

		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			log.Fatalf("接收到对方的计算的中间结果出错, %v", err)
			return err
		}
		log.Printf("接收到对方的计算的中间结果：count: %d, average: %d", resp.Count, resp.Val)
		time.Sleep(time.Duration(100) * time.Millisecond)
	}
	if err := stream.CloseSend(); err != nil {
		log.Fatalf("结束向对方发送数字出错, %v", err)
	}
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

	mathServ := &mathServer{}
	mathServ.init()

	// Register TASK API
	test.RegisterMathServiceServer(s, mathServ)
	reflection.Register(s)

	log.Printf("Listening on %v", address)
	go func() {
		if err := s.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	if err := registerTask(); err != nil {
		log.Fatalf("failed to register task to local register server: %v", err)
	}

	//等待所有的任务都注册完成
	time.Sleep(time.Duration(5) * time.Second)
	var cmdLine string

	for {
		fmt.Println("Please input command：unary|serverStreaming|clientStreaming|bidi|quit")
		fmt.Scanln(&cmdLine)
		if cmdLine == "quit" {
			break
		} else {
			// Execute command
			if cmd, ok := commands[cmdLine]; ok {
				if err := cmd(mathServ); err != nil {
					log.Fatal(err)
				}
			} else {
				log.Printf("invalid command: %s", cmdLine)
			}
		}
	}
}
