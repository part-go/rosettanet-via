package main

import (
	"crypto/rand"
	"flag"
	"fmt"
	"github.com/bglmmz/grpc"
	"github.com/bglmmz/grpc/credentials"
	"github.com/bglmmz/grpc/metadata"
	"github.com/bglmmz/grpc/reflection"
	"golang.org/x/net/context"
	"io"
	"log"
	"math/big"
	"net"
	"via/conf"
	"via/creds"
	"via/proxy"

	"time"
	"via/register"
	"via/test"
)

var (
	address    string
	localVia   string
	destVia    string
	partner    string
	sslFile    string
	sslEnabled = false
	commands   map[string]Command
)

// A Command is the API for a sub-command
type Command func(*mathServer) error

const DefaultTaskId string = "testTaskId"
const DefaultPartyId string = "testPartyId"

func init() {
	flag.StringVar(&partner, "partner", "partner_1", "partner")
	flag.StringVar(&address, "address", ":10040", "Math server listen address")
	flag.StringVar(&localVia, "localVia", ":10031", "local VIA address")
	flag.StringVar(&destVia, "destVia", ":20031", "dest VIA address")
	flag.StringVar(&sslFile, "ssl", "", "SSL config file")
	flag.Parse()

	if len(sslFile) > 0 {
		sslEnabled = true
	}
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

func (s *mathServer) dialDestVIA() {
	ctx := context.Background()
	ctx = metadata.AppendToOutgoingContext(ctx, proxy.MetadataTaskIdKey, DefaultTaskId)
	ctx = metadata.AppendToOutgoingContext(ctx, proxy.MetadataPartyIdKey, DefaultPartyId)
	s.ctx = ctx

	if sslEnabled {
		if conn, err := grpc.Dial(destVia, grpc.WithTransportCredentials(tlsCredentialsAsClient)); err != nil {
			log.Fatalf("did not connect to dest VIA server: %v", err)
		} else {
			log.Printf("Success to connect to dest VIA server with secure: %v", destVia)
			s.client = test.NewMathServiceClient(conn)
		}
	} else {
		if conn, err := grpc.Dial(destVia, grpc.WithInsecure()); err != nil {
			log.Fatalf("did not connect to dest VIA server: %v", err)
		} else {
			log.Printf("Success to connect to dest VIA server with insecure: %v", destVia)
			s.client = test.NewMathServiceClient(conn)
		}
	}
}

func (s *mathServer) Sum_Unary(ctx context.Context, metricList *test.MetricList) (*test.SumResponse, error) {
	log.Printf("服务(unary)：求列表之和：%v", metricList.Metric)
	var sum int64
	for _, metric := range metricList.Metric {
		sum += metric
	}
	log.Printf("计算服务结束(unary)：求列表之和：%v sum=%d", metricList, sum)
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
	log.Printf("dial to local VIA server on %v", localVia)

	var conn *grpc.ClientConn
	var err error

	if sslEnabled {
		log.Printf("dail up to VIA with secure")
		conn, err = grpc.Dial(localVia, grpc.WithTransportCredentials(tlsCredentialsAsClient))
	} else {
		log.Printf("dail up to VIA with insecure")
		conn, err = grpc.Dial(localVia, grpc.WithInsecure())
	}

	if err != nil {
		log.Fatalf("did not connect to local register server: %v", err)
	}
	defer conn.Close()

	log.Printf("register task to local VIA server %v", localVia)

	c := register.NewRegisterServiceClient(conn)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	r, err := c.Register(ctx, &register.RegisterReq{TaskId: DefaultTaskId, PartyId: DefaultPartyId, Address: address})
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
	/*if err := stream.CloseSend(); err != nil {
		log.Fatalf("结束向对方发送数字出错, %v", err)
	}*/
	return nil
}

func main() {
	lis, err := net.Listen("tcp", address)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}

	var grpcServer *grpc.Server
	grpcServer = grpc.NewServer()
	if sslEnabled {
		log.Print("running math server with secure!")
		grpcServer = grpc.NewServer(grpc.Creds(tlsCredentialsAsServer))
	} else {
		log.Print("running math server with insecure!")
		grpcServer = grpc.NewServer()
	}

	mathServ := &mathServer{}

	mathServ.dialDestVIA()

	// Register a non-ssl server for local VIA
	test.RegisterMathServiceServer(grpcServer, mathServ)
	reflection.Register(grpcServer)

	log.Printf("Listening on %v", address)
	go func() {
		if err := grpcServer.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()

	// Register TASK
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
			tlsCredentialsAsServer, err = creds.NewServerTLSOneWay(sslConfig.Conf.SSL.IoCertFile, sslConfig.Conf.SSL.IoKeyFile)
			if err != nil {
				panic(err)
			}
			tlsCredentialsAsClient, err = creds.NewClientTLSOneWay(sslConfig.Conf.SSL.CaCertFile)
			if err != nil {
				panic(err)
			}
		} else if sslConfig.Conf.Mode == "two_way" {
			tlsCredentialsAsServer, err = creds.NewServerTLSTwoWay(sslConfig.Conf.SSL.CaCertFile, sslConfig.Conf.SSL.IoCertFile, sslConfig.Conf.SSL.IoKeyFile)
			if err != nil {
				panic(err)
			}
			tlsCredentialsAsClient, err = creds.NewClientTLSTwoWay(sslConfig.Conf.SSL.CaCertFile, sslConfig.Conf.SSL.IoCertFile, sslConfig.Conf.SSL.IoKeyFile)
			if err != nil {
				panic(err)
			}
		} else {
			log.Fatalf("error mode in ssl-conf.yml, %s", sslConfig.Conf.Mode)
		}
	} else if sslConfig.Conf.Cipher == "gmssl" {
		if sslConfig.Conf.Mode == "one_way" {
			tlsCredentialsAsServer, err = creds.NewServerGMTLSOneWay(
				sslConfig.Conf.GMSSL.IoSignCertFile, sslConfig.Conf.GMSSL.IoSignKeyFile,
				sslConfig.Conf.GMSSL.IoEncryptCertFile, sslConfig.Conf.GMSSL.IoEncryptKeyFile,
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
				sslConfig.Conf.GMSSL.IoSignCertFile, sslConfig.Conf.GMSSL.IoSignKeyFile,
				sslConfig.Conf.GMSSL.IoEncryptCertFile, sslConfig.Conf.GMSSL.IoEncryptKeyFile,
			)

			if err != nil {
				panic(err)
			}

			tlsCredentialsAsClient, err = creds.NewClientGMTLSTwoWay(
				sslConfig.Conf.GMSSL.CaCertFile,
				sslConfig.Conf.GMSSL.IoSignCertFile, sslConfig.Conf.GMSSL.IoSignKeyFile,
				sslConfig.Conf.GMSSL.IoEncryptCertFile, sslConfig.Conf.GMSSL.IoEncryptKeyFile,
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
