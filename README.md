### VIA服务说明 

在启动本地task服务进程时，需要到VIA注册task服务的taskId/partyId/serviceType/address信息。

本地task服务进程需要访问远程task服务进程的grpc接口时，需要在metadata中，携带本次任务的taskId,以及远程task服务进程的的partyId。
相应的metadata key定义为：
```
MetadataTaskIdKey = "task_id"
MetadataPartyIdKey = "party_id"
```

#### 注册服务go代码生成：
```
protoc --go_out=plugins=grpc:. register/proto/*.proto
```

#### 测试用task服务go代码生成：
```
protoc --go_out=plugins=grpc:. test/proto/*.proto
```

#### VIA源码的启动方式：
```
go run ./cmd/via_server.go -registerAddress :10030 -proxyAddress :10031
```
其中

registerAddress：
表示VIA服务本身提供的grpc服务地址。VIA服务，本身也是个grpc服务，提供注册任务服务。

proxyAddress：
表示VIA提供的对外代理服务地址。

#### 演示方法：

1. 启动一个VIA服务：
```
go run ./cmd/via_server.go -registerAddress :10030 -proxyAddress :10031
```
2. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math_server.go -partner partner_1 -address :10040 -registerAddress :10030 -destProxyAddress :20031
```
3. 启动另一个VIA服务：
```
go run ./cmd/via_server.go -registerAddress :20030 -proxyAddress :20031
```
4. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math_server.go -partner partner_2 -address :20040 -registerAddress :20030 -destProxyAddress :10031
```

在两个启动的task服务的控制台，按提示输入命令，即可演示各种模式grpc。

命令行提示：

unary|serverStreaming|clientStreaming|bidi

