package proto

import (
	"context"
	"encoding/json"

	"google.golang.org/grpc"
	"google.golang.org/grpc/encoding"
)

const ProviderPlugin_ServiceDescName = "sigilbridge.adapter.v1.ProviderPlugin"

type jsonCodec struct{}

func (jsonCodec) Name() string { return "json" }
func (jsonCodec) Marshal(v any) ([]byte, error) {
	return json.Marshal(v)
}
func (jsonCodec) Unmarshal(data []byte, v any) error {
	return json.Unmarshal(data, v)
}

func init() {
	encoding.RegisterCodec(jsonCodec{})
}

type ProviderPluginClient interface {
	Chat(ctx context.Context, in *ChatRequest, opts ...grpc.CallOption) (*ChatResponse, error)
	Stream(ctx context.Context, in *ChatRequest, opts ...grpc.CallOption) (ProviderPlugin_StreamClient, error)
	Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error)
}

type providerPluginClient struct {
	cc grpc.ClientConnInterface
}

func NewProviderPluginClient(cc grpc.ClientConnInterface) ProviderPluginClient {
	return &providerPluginClient{cc: cc}
}

func (c *providerPluginClient) Chat(ctx context.Context, in *ChatRequest, opts ...grpc.CallOption) (*ChatResponse, error) {
	out := new(ChatResponse)
	opts = append(opts, grpc.ForceCodec(jsonCodec{}))
	err := c.cc.Invoke(ctx, "/"+ProviderPlugin_ServiceDescName+"/Chat", in, out, opts...)
	return out, err
}

func (c *providerPluginClient) Stream(ctx context.Context, in *ChatRequest, opts ...grpc.CallOption) (ProviderPlugin_StreamClient, error) {
	opts = append(opts, grpc.ForceCodec(jsonCodec{}))
	stream, err := c.cc.NewStream(ctx, &ProviderPlugin_ServiceDesc.Streams[0], "/"+ProviderPlugin_ServiceDescName+"/Stream", opts...)
	if err != nil {
		return nil, err
	}
	x := &providerPluginStreamClient{ClientStream: stream}
	if err := x.ClientStream.SendMsg(in); err != nil {
		return nil, err
	}
	if err := x.ClientStream.CloseSend(); err != nil {
		return nil, err
	}
	return x, nil
}

func (c *providerPluginClient) Health(ctx context.Context, in *HealthRequest, opts ...grpc.CallOption) (*HealthResponse, error) {
	out := new(HealthResponse)
	opts = append(opts, grpc.ForceCodec(jsonCodec{}))
	err := c.cc.Invoke(ctx, "/"+ProviderPlugin_ServiceDescName+"/Health", in, out, opts...)
	return out, err
}

type ProviderPlugin_StreamClient interface {
	Recv() (*StreamEvent, error)
	grpc.ClientStream
}

type providerPluginStreamClient struct {
	grpc.ClientStream
}

func (x *providerPluginStreamClient) Recv() (*StreamEvent, error) {
	m := new(StreamEvent)
	if err := x.ClientStream.RecvMsg(m); err != nil {
		return nil, err
	}
	return m, nil
}

type ProviderPluginServer interface {
	Chat(context.Context, *ChatRequest) (*ChatResponse, error)
	Stream(*ChatRequest, ProviderPlugin_StreamServer) error
	Health(context.Context, *HealthRequest) (*HealthResponse, error)
}

type ProviderPlugin_StreamServer interface {
	Send(*StreamEvent) error
	grpc.ServerStream
}

func RegisterProviderPluginServer(s grpc.ServiceRegistrar, srv ProviderPluginServer) {
	s.RegisterService(&ProviderPlugin_ServiceDesc, srv)
}

var ProviderPlugin_ServiceDesc = grpc.ServiceDesc{
	ServiceName: ProviderPlugin_ServiceDescName,
	HandlerType: (*ProviderPluginServer)(nil),
	Methods: []grpc.MethodDesc{
		{MethodName: "Chat", Handler: _ProviderPlugin_Chat_Handler},
		{MethodName: "Health", Handler: _ProviderPlugin_Health_Handler},
	},
	Streams: []grpc.StreamDesc{
		{StreamName: "Stream", Handler: _ProviderPlugin_Stream_Handler, ServerStreams: true},
	},
	Metadata: "adapter.proto",
}

func _ProviderPlugin_Chat_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(ChatRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderPluginServer).Chat(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ProviderPlugin_ServiceDescName + "/Chat"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(ProviderPluginServer).Chat(ctx, req.(*ChatRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderPlugin_Health_Handler(srv any, ctx context.Context, dec func(any) error, interceptor grpc.UnaryServerInterceptor) (any, error) {
	in := new(HealthRequest)
	if err := dec(in); err != nil {
		return nil, err
	}
	if interceptor == nil {
		return srv.(ProviderPluginServer).Health(ctx, in)
	}
	info := &grpc.UnaryServerInfo{Server: srv, FullMethod: "/" + ProviderPlugin_ServiceDescName + "/Health"}
	handler := func(ctx context.Context, req any) (any, error) {
		return srv.(ProviderPluginServer).Health(ctx, req.(*HealthRequest))
	}
	return interceptor(ctx, in, info, handler)
}

func _ProviderPlugin_Stream_Handler(srv any, stream grpc.ServerStream) error {
	m := new(ChatRequest)
	if err := stream.RecvMsg(m); err != nil {
		return err
	}
	return srv.(ProviderPluginServer).Stream(m, &providerPluginStreamServer{ServerStream: stream})
}

type providerPluginStreamServer struct {
	grpc.ServerStream
}

func (x *providerPluginStreamServer) Send(m *StreamEvent) error {
	return x.ServerStream.SendMsg(m)
}
