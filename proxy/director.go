package proxy

import (
	"golang.org/x/net/context"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/status"
	"log"
)

// StreamDirector returns a gRPC ClientConn to be used to forward the call to.
//
// The presence of the `Context` allows for rich filtering, e.g. based on Metadata (headers).
// If no handling is meant to be done, a `codes.NotImplemented` gRPC error should be returned.
//
// The context returned from this function should be the context for the *outgoing* (to backend) call. In case you want
// to forward any Metadata between the inbound request and outbound requests, you should do it manually. However, you
// *must* propagate the cancel function (`context.WithCancel`) of the inbound context to the one returned.
//
// It is worth noting that the StreamDirector will be fired *after* all server-side stream interceptors
// are invoked. So decisions around authorization, monitoring etc. are better to be handled there.
//
// See the rather rich example.
type StreamDirector func(ctx context.Context, fullMethodName string) (context.Context, *grpc.ClientConn, error)

//任务的服务信息
type RegisteredTask struct {
	TaskId      string           //任务id
	ServiceType string           //任务服务类型
	Address     string           //任务服务地址,ip:port
	Conn        *grpc.ClientConn //proxy到任务服务的grpc调用连接，此链接在任务服务到proxy注册后，由proxy建立
}

// 存放注册的任务服务进程信息
var RegisteredTaskMap = make(map[string]*RegisteredTask)

const MetadataKey = "task_id"

func GetDirector() StreamDirector {
	director := func(ctx context.Context, fullName string) (context.Context, *grpc.ClientConn, error) {
		md, ok := metadata.FromIncomingContext(ctx)
		log.Printf("收到的metadata.md: %v", md)
		if ok {
			if taskId, exists := md[MetadataKey]; exists {
				if _, ok := RegisteredTaskMap[taskId[0]]; ok {
					outCtx, _ := context.WithCancel(ctx)

					// Explicitly copy the metadata, otherwise the tests will fail.
					outCtx = metadata.NewOutgoingContext(outCtx, md.Copy())
					return outCtx, RegisteredTaskMap[taskId[0]].Conn, nil
				} else {
					return ctx, nil, status.Errorf(codes.Unknown, "cannot find connection for registered task")
				}
			} else {
				return ctx, nil, status.Errorf(codes.NotFound, "task id not found")
			}
		} else {
			return ctx, nil, status.Errorf(codes.Unknown, "cannot get metadata from incoming context")
		}
	}
	return director
}
