package proxy

import (
	"fmt"
	"google.golang.org/grpc/encoding"
	"log"

	"github.com/golang/protobuf/proto"
)

// Codec returns a proxying encoding.Codec with the default protobuf codec as parent.
//
// See CodecWithParent.
// 以protobuf原生codec为默认codec，实现了一个透明的Marshal和Unmarshal
func Codec() encoding.Codec {
	return CodecWithParent(&protoCodec{})
}

// CodecWithParent returns a proxying encoding.Codec with a user provided codec as parent.
//
// This codec is *crucial* to the functioning of the proxy. It allows the proxy server to be oblivious
// to the schema of the forwarded messages. It basically treats a gRPC message frame as raw bytes.
// However, if the server handler, or the client caller are not proxy-internal functions it will fall back
// to trying to decode the message using a fallback codec.
func CodecWithParent(fallback encoding.Codec) encoding.Codec {
	return &rawCodec{fallback}
}

// 自定义原始编码器
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
	log.Printf("rawCodec.Marshal: v:%v", v)
	out, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Marshal(v)
	}
	return out.payload, nil

}

// 反序列化函数，
// 将消息转为frame类型，并将数据放入frame.payload;
// 如果消息不能转为frame类型，则调用parentCodec的Unmarshal来反序列化
func (c *rawCodec) Unmarshal(data []byte, v interface{}) error {
	log.Printf("rawCodec.Unmarshal: data:%v, v:%v", data, v)
	dst, ok := v.(*frame)
	if !ok {
		return c.parentCodec.Unmarshal(data, v)
	}
	dst.payload = data
	return nil
}

//自定义原始编码器名
func (c *rawCodec) Name() string {
	return fmt.Sprintf("custom raw codec")
}

// protoCodec是protobuf的编码器实现，它是缺省的原生编码器
type protoCodec struct{}

func (protoCodec) Marshal(v interface{}) ([]byte, error) {
	log.Printf("protoCodec.Marshal: v:%v", v)
	return proto.Marshal(v.(proto.Message))
}

func (protoCodec) Unmarshal(data []byte, v interface{}) error {
	log.Printf("protoCodec.Unmarshal: data:%v, v:%v", data, v)
	return proto.Unmarshal(data, v.(proto.Message))
}

func (protoCodec) Name() string {
	return "default raw codec"
}
