package plugin

import (
	"context"
	"encoding/json"
	"net"
	"testing"

	"github.com/sigilbridge/sigilbridge/internal/adapter"
	"github.com/sigilbridge/sigilbridge/internal/ir"
	pb "github.com/sigilbridge/sigilbridge/pkg/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/test/bufconn"
)

func TestGRPCAdapter(t *testing.T) {
	listener := bufconn.Listen(1024 * 1024)
	server := grpc.NewServer()
	pb.RegisterProviderPluginServer(server, pluginStub{})
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Stop()
	conn, err := grpc.DialContext(context.Background(), "bufnet", grpc.WithContextDialer(func(context.Context, string) (net.Conn, error) {
		return listener.Dial()
	}), grpc.WithInsecure())
	if err != nil {
		t.Fatalf("DialContext() error = %v", err)
	}
	defer conn.Close()
	provider := NewGRPCAdapter("example_plugin", pb.NewProviderPluginClient(conn))
	resp, err := provider.Chat(context.Background(), ir.Request{Version: ir.Version, ModelAlias: "m"}, adapter.ProviderConfig{})
	if err != nil {
		t.Fatalf("Chat() error = %v", err)
	}
	if resp.Content[0].Text != "plugin response" {
		t.Fatalf("response = %#v", resp)
	}
	if err := provider.HealthCheck(context.Background(), adapter.ProviderConfig{}); err != nil {
		t.Fatalf("HealthCheck() error = %v", err)
	}
}

type pluginStub struct{}

func (pluginStub) Chat(context.Context, *pb.ChatRequest) (*pb.ChatResponse, error) {
	raw, _ := json.Marshal(ir.Response{Version: ir.Version, UpstreamProvider: "example_plugin", StopReason: ir.StopEndTurn, Content: []ir.ContentBlock{{Type: ir.ContentText, Text: "plugin response"}}})
	return &pb.ChatResponse{ResponseJson: raw}, nil
}

func (pluginStub) Stream(*pb.ChatRequest, pb.ProviderPlugin_StreamServer) error {
	return nil
}

func (pluginStub) Health(context.Context, *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{Ok: true}, nil
}
