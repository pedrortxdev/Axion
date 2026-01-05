package axhv

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"aexon/internal/provider/axhv/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn    *grpc.ClientConn
	service pb.VmServiceClient
}

// NewClient creates a new gRPC client connection to the AxHV daemon.
// It supports both Unix Socket (default) and TCP (if address is provided).
func NewClient(socketPath string, tcpAddress string, token string) (*Client, error) {
	var opts []grpc.DialOption
	opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))

	var target string

	if tcpAddress != "" {
		// TCP Mode (Cluster)
		target = tcpAddress
		if token != "" {
			opts = append(opts, grpc.WithPerRPCCredentials(&tokenAuth{token: token}))
		}
		log.Printf("[AxHV Client] Connecting via TCP to %s", target)
	} else {
		// Unix Socket Mode (Local)
		if socketPath == "" {
			socketPath = "/tmp/axhv.sock"
		}
		target = "unix://" + socketPath

		// Custom dialer for Unix socket
		dialer := func(ctx context.Context, addr string) (net.Conn, error) {
			return net.Dial("unix", socketPath) // Ignore addr in dialer, use socketPath directly
		}
		opts = append(opts, grpc.WithContextDialer(dialer))
		log.Printf("[AxHV Client] Connecting via Unix Socket to %s", socketPath)
	}

	// Connect with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	conn, err := grpc.DialContext(ctx, target, opts...)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to AxHV daemon: %w", err)
	}

	client := pb.NewVmServiceClient(conn)

	return &Client{
		conn:    conn,
		service: client,
	}, nil
}

func (c *Client) Close() error {
	return c.conn.Close()
}

// Token Auth implementation for gRPC
type tokenAuth struct {
	token string
}

func (t *tokenAuth) GetRequestMetadata(ctx context.Context, uri ...string) (map[string]string, error) {
	return map[string]string{
		"authorization": "Bearer " + t.token,
	}, nil
}

func (t *tokenAuth) RequireTransportSecurity() bool {
	return false
}

// Wrapper methods for VmService

func (c *Client) CreateVm(ctx context.Context, req *pb.CreateVmRequest) (*pb.VmResponse, error) {
	return c.service.CreateVm(ctx, req)
}

func (c *Client) StartVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.StartVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) StopVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.StopVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) DeleteVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.DeleteVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) ListVms(ctx context.Context) (*pb.ListVmsResponse, error) {
	return c.service.ListVms(ctx, &pb.Empty{})
}

func (c *Client) GetVmStats(ctx context.Context, id string) (*pb.VmStatsResponse, error) {
	return c.service.GetVmStats(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) RebootVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.RebootVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) PauseVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.PauseVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) ResumeVm(ctx context.Context, id string) (*pb.VmResponse, error) {
	return c.service.ResumeVm(ctx, &pb.VmIdRequest{Id: id})
}

func (c *Client) AddPort(ctx context.Context, id string, hostPort int, containerPort int, proto string) error {
	// Not implemented in AxHV API currently as a separate call,
	// it's part of Update/Create config usually, but for now we might need to rely on initial config
	// or return not implemented.
	// Based on API.md, port mapping is part of CreateVm.
	// There is no AddPort RPC.
	return fmt.Errorf("dynamic port addition not supported in AxHV v2")
}
