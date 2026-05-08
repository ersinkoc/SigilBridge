package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/sigilbridge/sigilbridge/internal/ir"
	pb "github.com/sigilbridge/sigilbridge/pkg/proto"
	"google.golang.org/grpc"
)

type server struct {
	pb.ProviderPluginServer
}

func (server) Chat(_ context.Context, req *pb.ChatRequest) (*pb.ChatResponse, error) {
	var decoded ir.Request
	if err := json.Unmarshal(req.RequestJson, &decoded); err != nil {
		return nil, err
	}
	text := "example plugin response"
	if decoded.ModelAlias != "" {
		text = fmt.Sprintf("example plugin response for %s", decoded.ModelAlias)
	}
	resp := ir.Response{
		Version:          ir.Version,
		ID:               "plugin_" + time.Now().UTC().Format("20060102150405"),
		UpstreamProvider: req.ProviderId,
		UpstreamModel:    decoded.ModelAlias,
		StopReason:       ir.StopEndTurn,
		Content:          []ir.ContentBlock{{Type: ir.ContentText, Text: text}},
		Usage:            ir.Usage{InputTokens: 8, OutputTokens: 5},
	}
	raw, err := json.Marshal(resp)
	if err != nil {
		return nil, err
	}
	return &pb.ChatResponse{ResponseJson: raw}, nil
}

func (s server) Stream(req *pb.ChatRequest, stream pb.ProviderPlugin_StreamServer) error {
	resp, err := s.Chat(stream.Context(), req)
	if err != nil {
		return err
	}
	var decoded ir.Response
	if err := json.Unmarshal(resp.ResponseJson, &decoded); err != nil {
		return err
	}
	events := []ir.Event{
		{Version: ir.Version, Type: ir.EventStart},
		{Version: ir.Version, Type: ir.EventContentBlockDelta, Index: 0, Delta: &decoded.Content[0]},
		{Version: ir.Version, Type: ir.EventUsage, Usage: &decoded.Usage},
		{Version: ir.Version, Type: ir.EventStop, StopReason: decoded.StopReason},
	}
	for _, event := range events {
		raw, err := json.Marshal(event)
		if err != nil {
			return err
		}
		if err := stream.Send(&pb.StreamEvent{EventJson: raw}); err != nil {
			return err
		}
	}
	return nil
}

func (server) Health(context.Context, *pb.HealthRequest) (*pb.HealthResponse, error) {
	return &pb.HealthResponse{Ok: true, Message: "ok"}, nil
}

func main() {
	listen := flag.String("listen", "127.0.0.1:0", "gRPC listen address")
	flag.Parse()

	lis, err := net.Listen("tcp", *listen)
	if err != nil {
		log.Fatal(err)
	}
	grpcServer := grpc.NewServer()
	pb.RegisterProviderPluginServer(grpcServer, server{})
	log.Printf("sigilbridge example plugin listening on %s", lis.Addr())
	if err := grpcServer.Serve(lis); err != nil {
		log.Fatal(err)
	}
}
