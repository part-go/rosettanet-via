
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

组件版本：
- grpc-go v1.15.0
- protoc v3.7.0
- protobuf-go v1.3.0

```
protoc --go_out=plugins=grpc:. register/proto/*.proto
```

- 生成后的via.pb.go，需要修改下Import的包，把grpc "google.golang.org/grpc" 改成 grpc "github.com/bglmmz/grpc"



#### 测试用task服务go代码生成：
```
protoc --go_out=plugins=grpc:. test/proto/*.proto
```
- 生成后的math.pb.go，需要修改下Import的包，把grpc "google.golang.org/grpc" 改成 grpc "github.com/bglmmz/grpc"


#### VIA源码的启动方式

- SSL方式：
```
go run ./cmd/via/main.go -ssl conf/ssl-conf.yml -address 0.0.0.0:10031
```

其中

- ssl:
ssl配置文件，配置VIA代理服务要求的安全模式（SSL模式），以及SSL模式时需要的各种证书。

- address：
表示VIA服务的监听地址


- 非SSL方式：
```
go run ./cmd/via/main.go -address 0.0.0.0:10031
```
其中

- address：
表示VIA服务的监听地址

**注意事项**

- 所有 VIA 的安全模式必须一致
- task 服务实例的安全模式必须和 VIA 保持一致
- 当 VIA 的安全模式是SSL时，VIA、task服务实例所用的证书，必须是证书链中的某一个相同的机构签发。


#### 演示方法：

1. 启动一个VIA服务：
```
go run ./cmd/via/main.go -ssl conf/ssl-conf.yml -address 0.0.0.0:10031
```
2. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math/main.go -ssl conf/ssl-conf.yml -partner partner_1 -address 0.0.0.0:10040 -localVia 0.0.0.0:10031 -destVia 0.0.0.0:20031
```
3. 启动另一个VIA服务：
```
go run ./cmd/via/main.go -ssl conf/ssl-conf.yml -address 0.0.0.0:20031
```
4. 启动这个VIA服务后面的task服务：
```
go run ./test/cmd/math/main.go -ssl conf/ssl-conf.yml -partner partner_2 -address 0.0.0.0:20040 -localVia 0.0.0.0:20031 -destVia 0.0.0.0:10031
```

在两个启动的task服务的控制台，按提示输入命令，即可演示各种模式grpc调用。

命令行提示：

unary|serverStreaming|clientStreaming|bidi

