package integration

import (
	"context"
	"fmt"
	"net"
	"sync/atomic"
	"testing"
	"time"

	helloworldv1 "gitee.com/flycash/notification-platform/api/proto/gen/helloworld/v1"
	clientpkg "gitee.com/flycash/notification-platform/internal/pkg/client"
	"gitee.com/flycash/notification-platform/internal/pkg/registry"
	"gitee.com/flycash/notification-platform/internal/pkg/registry/etcd"
	testioc "gitee.com/flycash/notification-platform/internal/test/ioc"
	"github.com/ego-component/eetcd"
	"github.com/stretchr/testify/suite"
	"google.golang.org/grpc"
	"google.golang.org/grpc/balancer"
	"google.golang.org/grpc/balancer/base"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

func TestRegistrySuite(t *testing.T) {
	t.Parallel()
	suite.Run(t, new(RegistryTestSuite))
}

type RegistryTestSuite struct {
	suite.Suite
	etcdClient *eetcd.Component
}

func (s *RegistryTestSuite) SetupSuite() {
	s.etcdClient = testioc.InitEtcdClient()
}

// TestGroup 测试基于分组的服务发现和请求路由功能
// 本测试验证：
// 1. 启动4个服务器实例：3个group=core，1个group=""
// 2. 客户端通过在Context中设置group="core"标签
// 3. 验证所有请求都路由到了group=core的服务器
func (s *RegistryTestSuite) TestGroup() {
	t := s.T()

	// 创建etcd注册中心实例
	etcdRegistry, err := etcd.NewRegistry(s.etcdClient.Client)
	s.Require().NoError(err)
	defer etcdRegistry.Close()

	// 定义超时时间
	timeout := time.Second * 10

	// 创建4个服务器实例，3个设置group=core，1个设置group=""
	var servers []*Server
	var coreServers []*greeterServer
	var defaultServers []*greeterServer

	for i := 0; i < 3; i++ {
		gs := &greeterServer{group: "core"}
		srv := NewServer("greeter",
			ServerWithRegistry(etcdRegistry),
			ServerWithTimeout(timeout),
			ServerWithGroup("core"))

		// 注册greeter服务
		helloworldv1.RegisterGreeterServiceServer(srv.Server, gs)

		// 使用随机端口启动服务
		go func() {
			_ = srv.Start("127.0.0.1:0")
		}()

		// 等待服务开始监听
		time.Sleep(time.Millisecond * 200)

		// 确保listener已经创建
		for srv.listener == nil {
			time.Sleep(time.Millisecond * 50)
		}

		gs.addr = srv.listener.Addr().String()
		servers = append(servers, srv)
		coreServers = append(coreServers, gs)
	}

	// 启动1个group=""的默认服务
	gs := &greeterServer{group: ""}
	srv := NewServer("greeter",
		ServerWithRegistry(etcdRegistry),
		ServerWithTimeout(timeout),
		ServerWithGroup(""))

	helloworldv1.RegisterGreeterServiceServer(srv.Server, gs)

	go func() {
		err := srv.Start("127.0.0.1:0")
		if err != nil {
			t.Logf("启动服务失败: %v", err)
		}
	}()

	// 等待服务开始监听
	time.Sleep(time.Millisecond * 200)

	// 确保listener已经创建
	for srv.listener == nil {
		time.Sleep(time.Millisecond * 50)
	}

	gs.addr = srv.listener.Addr().String()
	servers = append(servers, srv)
	defaultServers = append(defaultServers, gs)

	// 确保所有服务器在测试结束后关闭
	defer func() {
		for _, s := range servers {
			s.Close()
		}
	}()

	// 等待服务注册完成
	time.Sleep(time.Second * 3)

	// 列出所有服务，确认注册成功
	services, err := etcdRegistry.ListServices(context.Background(), "greeter")
	s.Require().NoError(err)
	s.Require().Equal(4, len(services), "应该有4个服务实例注册成功")

	// 创建客户端
	c := NewClient(
		ClientWithRegistry(etcdRegistry, timeout),
		ClientWithInsecure(),
		ClientWithPickerBuilder("weight", &clientpkg.WeightBalancerBuilder{}),
	)

	// 连接服务
	conn, err := c.Dial("greeter")
	s.Require().NoError(err)
	defer conn.Close()

	// 创建客户端
	client := helloworldv1.NewGreeterServiceClient(conn)

	// 稍等片刻，确保连接建立
	time.Sleep(time.Second)

	// 发起5次调用，验证所有请求都路由到了group=core的服务器
	for i := 0; i < 5; i++ {
		reqCtx, reqCancel := context.WithTimeout(context.Background(), timeout)

		// 在context中设置group="core"，使请求被路由到core组的服务器
		reqCtx = clientpkg.WithGroup(reqCtx, "core")

		// 发送请求
		resp, err := client.SayHello(
			reqCtx,
			&helloworldv1.SayHelloRequest{Name: fmt.Sprintf("test-%d", i)},
		)
		reqCancel()
		s.Require().NoError(err)

		// 验证响应包含了服务器地址和组信息
		s.Contains(resp.Message, "group=core", "响应应该来自core组服务器")

		// 短暂等待，避免请求过于密集
		time.Sleep(time.Millisecond * 100)
	}

	// 等待一会确保所有请求都已完成处理
	time.Sleep(time.Second)

	// 验证请求分布情况
	var coreTotal int32
	for _, s := range coreServers {
		coreTotal += s.reqCnt.Load()
	}

	var defaultTotal int32
	for _, s := range defaultServers {
		defaultTotal += s.reqCnt.Load()
	}

	s.Equal(int32(5), coreTotal, "所有请求应该被发送到group=core的服务器")
	s.Equal(int32(0), defaultTotal, "group=''的服务器不应该收到请求")

	// 验证请求在core组服务器之间有分布
	var nonZeroServers int
	for _, server := range coreServers {
		if server.reqCnt.Load() > 0 {
			nonZeroServers++
		}
	}
	s.Greater(nonZeroServers, 0, "至少有一个core组服务器应该收到请求")
}

// 创建Greeter服务的实现
type greeterServer struct {
	helloworldv1.UnimplementedGreeterServiceServer
	addr   string
	group  string
	reqCnt atomic.Int32
}

// 实现SayHello方法
func (g *greeterServer) SayHello(_ context.Context, req *helloworldv1.SayHelloRequest) (*helloworldv1.SayHelloResponse, error) {
	g.reqCnt.Add(1)
	return &helloworldv1.SayHelloResponse{
		Message: fmt.Sprintf("Hello %s from server %s in group=%s", req.Name, g.addr, g.group),
	}, nil
}

type Server struct {
	name     string
	listener net.Listener

	si       registry.ServiceInstance
	registry registry.Registry
	// 单个操作的超时时间，一般用于和注册中心打交道
	timeout time.Duration
	*grpc.Server

	readWeight  int32
	writeWeight int32
	group       string
}

func NewServer(name string, opts ...ServerOption) *Server {
	res := &Server{
		name:        name,
		Server:      grpc.NewServer(),
		readWeight:  1,
		writeWeight: 1,
	}
	for _, opt := range opts {
		opt(res)
	}
	return res
}

func ServerWithGroup(group string) ServerOption {
	return func(server *Server) {
		server.group = group
	}
}

func ServerWithRegistry(r registry.Registry) ServerOption {
	return func(server *Server) {
		server.registry = r
	}
}

func ServerWithTimeout(timeout time.Duration) ServerOption {
	return func(server *Server) {
		server.timeout = timeout
	}
}

func ServerWithReadWeight(readWeight int32) ServerOption {
	return func(server *Server) {
		server.readWeight = readWeight
	}
}

func ServerWithWriteWeight(writeWeight int32) ServerOption {
	return func(server *Server) {
		server.writeWeight = writeWeight
	}
}

type ServerOption func(server *Server)

func (s *Server) Start(addr string) error {
	listener, err := net.Listen("tcp", addr)
	if err != nil {
		return err
	}
	s.listener = listener
	// 用户决定使用注册中心
	if s.registry != nil {
		s.si = registry.ServiceInstance{
			Name:        s.name,
			Address:     listener.Addr().String(),
			ReadWeight:  s.readWeight,
			WriteWeight: s.writeWeight,
			Group:       s.group,
		}
		ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
		err = s.registry.Register(ctx, s.si)
		cancel()
		if err != nil {
			return err
		}
	}
	return s.Server.Serve(listener)
}

func (s *Server) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), s.timeout)
	defer cancel()
	if s.registry != nil {
		if err := s.registry.UnRegister(ctx, s.si); err != nil {
			return err
		}
	}
	if s.listener != nil {
		return s.listener.Close()
	}
	return nil
}

type ClientOption func(client *Client)

type Client struct {
	rb       resolver.Builder
	insecure bool
	balancer balancer.Builder
}

func NewClient(opts ...ClientOption) *Client {
	client := &Client{}
	for _, opt := range opts {
		opt(client)
	}
	return client
}

func (c *Client) Dial(service string) (*grpc.ClientConn, error) {
	opts := []grpc.DialOption{grpc.WithResolvers(c.rb), grpc.WithNoProxy()}
	address := fmt.Sprintf("registry:///%s", service)
	if c.insecure {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if c.balancer != nil {
		opts = append(opts, grpc.WithDefaultServiceConfig(
			fmt.Sprintf(`{"loadBalancingPolicy": %q}`,
				c.balancer.Name())))
	}
	return grpc.NewClient(address, opts...)
}

func ClientWithRegistry(r registry.Registry, timeout time.Duration) ClientOption {
	return func(client *Client) {
		client.rb = clientpkg.NewResolverBuilder(r, timeout)
	}
}

func ClientWithInsecure() ClientOption {
	return func(client *Client) {
		client.insecure = true
	}
}

func ClientWithPickerBuilder(name string, b base.PickerBuilder) ClientOption {
	return func(client *Client) {
		builder := base.NewBalancerBuilder(name, b, base.Config{HealthCheck: true})
		balancer.Register(builder)
		client.balancer = builder
	}
}
