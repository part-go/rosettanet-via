### VIA服务说明 

VIA服务，既提供注册服务，以便本地task服务注册任务信息；也为远程task服务提供代理服务，以便访问本地task服务。

因此，在本地的**每一个参与方**的task服务进程启动时，首先需要到VIA注册task服务的taskId/partyId/serviceType/address等信息。

本地task服务进程需要访问远程task的某个参与方时，是直接访问远程VIA服务，同时在metadata中，携带任务的taskId,以及远程task服务的参与方的partyId。
相应的metadata key定义为：
```
MetadataTaskIdKey = "task_id"
MetadataPartyIdKey = "party_id"
```

#### VIA注册服务go代码生成：
```
protoc --go_out=plugins=grpc:. register/proto/*.proto
```

#### 测试用task服务go代码生成：
```
protoc --go_out=plugins=grpc:. test/proto/*.proto
```

#### VIA源码的启动方式：
```
go run ./cmd/via_server.go -address :10031
```
其中

address：
表示VIA的服务地址。

#### 演示方法：

1. 启动一个VIA服务：
```
go run ./cmd/via_server.go -address :10031
```
2. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math_server.go -partner partner_1 -address :10040 -localVia :10031 -destVia :20031
```
3. 启动另一个VIA服务：
```
go run ./cmd/via_server.go -address :20031
```
4. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math_server.go -partner partner_2 -address :20040 -localVia :20031 -destVia :10031
```

在两个启动的task服务的控制台，按提示输入命令，即可演示各种模式grpc。

命令行提示：

unary|serverStreaming|clientStreaming|bidi

