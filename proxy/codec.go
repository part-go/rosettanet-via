package proxy

import (
	"fmt"
	"github.com/bglmmz/grpc"
	"github.com/bglmmz/grpc/encoding"
	"log"
	"reflect"

	"github.com/golang/protobuf/proto"
)

// Codec returns a proxying encoding.Codec with the default protobuf codec as parent.
//
// See CodecWithParent.
// 以protobuf原生codec为默认codec，实现了一个透明的Marshal和Unmarshal
func Codec() grpc.Codec {
	return CodecWithParent(&protoCodec{})
}

// CodecWithParent returns a proxying encoding.Codec with a user provided codec as parent.
//
// This codec is *crucial* to the functioning of the proxy. It allows the proxy server to be oblivious
// to the schema of the forwarded messages. It basically treats a gRPC message frame as raw bytes.
// However, if the server handler, or the client caller are not proxy-internal functions it will fall back
// to trying to decode the message using a fallback codec.
func CodecWithParent(fallback encoding.Codec) grpc.Codec {
	return &rawCodec{fallback}
}

// VIA server收到stream数据后, 入口是 rawCodec.Unmarshal()
// 首先会尝试用自定义 rawCodec 来反序列化成frame结构，
// 如果不可以，再调用父编解码器 protoCodec（这个就是 grpc 内部使用的编解码器）来反序列化成响应消息(本地 grpc 服务支持的消息）。

// 自定义编码器，用来反序列号/序列化 frame结构。由于frame结构就是[]byte，因此实际上rawCodec不对数据做任何处理，目的是用来转发数据
type rawCodec struct {
	parentCodec encoding.Codec
}

type frame struct {
	payload []byte
}

// 序列化函数，
// 消息转为frame类型，并返回frame.payload中的数据;
// 如果消息不能转为frame类型，则调用parentCodec的Marshal来序列化
func (c *rawCodec) Marshal(v interface{}) ([]byte, error) {
	out, ok := v.(*frame)
	if !ok {
		//如果是VIA server服务支持的消息, v就是对应的类型。（如register.RegisterReq消息 和 register.Boolean消息）
		return c.parentCodec.Marshal(v)
	}
	//否则，则认为是需要转发的数据流，把可rawCodec.Unmarshal读出的数据直接返回即可
	//log.Printf("rawCodec.Marshal: just forward stream(out):%v", out.payload)
	return out.payload, nil

}

// 反序列化函数，
// 将消息转为frame类型，并将数据放入frame.payload;
// 如果消息不能转为frame类型，则调用parentCodec的Unmarshal来反序列化
func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	dst, ok := v.(*frame)
	if !ok {
		//如果是VIA server服务支持的消息, v就是对应的类型。（如register.RegisterReq消息 和 register.Boolean消息）
		return c.parentCodec.Unmarshal(data, v)
	}
	//否则，则认为是需要转发的数据流，读出数据即可
	//log.Printf("rawCodec.Unmarshal: just forward stream(in):%v", data)
	dst.payload = data
	return nil
}

//自定义原始编码器名
func (c *rawCodec) Name() string {
	return fmt.Sprintf("custom raw codec")
}

func (c *rawCodec) String() string {
	return fmt.Sprintf("custom raw codec")
}

// protoCodec是protobuf的编码器实现，它是缺省的原生编码器
type protoCodec struct{}

func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	log.Printf("protoCodec.Marshal: v:%v", reflect.TypeOf(v))
	return proto.Marshal(v.(proto.Message))
}

func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	log.Printf("protoCodec.Unmarshal: data:%v, v:%v", data, reflect.TypeOf(v))
	return proto.Unmarshal(data, v.(proto.Message))
}

func (protoCodec) Name() string {
	return "default raw codec"
}

func (protoCodec) String() string {
	return "default raw codec"
}
