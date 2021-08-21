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
go run ./cmd/via/main.go -tls conf/tls.yml -regAddr 0.0.0.0:10031 -proxyAddr 0.0.0.0:10032
```
其中
tls:
tls配置文件，配置VIA代理服务要求的安全模式（SSL模式），以及SSL模式时需要的各种证书。

regAddr：
表示VIA的任务注册服务地址（此服务是非SSL的）。

proxyAddr：
表示VIA的代理服务地址，此服务根据tls配置，决定是否是SSL，或更进一步是否是双向SSL

#### 演示方法：

1. 启动一个VIA服务：
```
go run ./cmd/via/main.go -tls conf/tls.yml -regAddr 0.0.0.0:10031 -proxyAddr 0.0.0.0:10032
```
2. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math/main.go -tls conf/tls.yml -partner partner_1 -address 0.0.0.0:10040 -localViaRegAddr 0.0.0
.0:10031 -destViaProxyAddr 0.0.0.0:20032

```
3. 启动另一个VIA服务：
```
go run ./cmd/via/main.go -tls conf/tls.yml -regAddr 0.0.0.0:20031 -proxyAddr 0.0.0.0:20032
```
4. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math/main.go -tls conf/tls.yml -partner partner_2 -address 0.0.0.0:20040 -localViaRegAddr 0.0.0
.0:20031 -destViaProxyAddr 0.0.0.0:10032
```

在两个启动的task服务的控制台，按提示输入命令，即可演示各种模式grpc。

命令行提示：

unary|serverStreaming|clientStreaming|bidi

