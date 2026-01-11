package rpc

import (
	"common/config"
	"common/discovery"
	"common/logs"
	"context"
	"fmt"
	"user/pb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/resolver"
)

var (
	UserClient pb.UserServiceClient
)

func Init() {
	// grpc 需要一个 解析器，在 Dial 时同步调用解析器获取 grpc 服务器地址
	// 创建一个 etcd 解析器
	etcdResolver := discovery.NewResolver(config.Conf.Etcd)
	// grpc 注册 etcdResolver
	resolver.Register(etcdResolver)
	userDomain := config.Conf.Domain["user"]
	initClt(userDomain.Name, userDomain.LoadBalance, &UserClient)
}

func initClt(name string, loadBalance bool, client interface{}) {
	// etcd 上服务的地址
	addr := fmt.Sprintf("etcd:///%s", name)
	// grpc Dial 拨号配置
	opts := []grpc.DialOption{
		grpc.WithTransportCredentials(insecure.NewCredentials())}
	if loadBalance {
		// grpc 添加负载均衡配置 LoadBalancingPolicy，策略 round_robin（默认）
		opts = append(opts, grpc.WithDefaultServiceConfig(fmt.Sprintf(`{"LoadBalancingPolicy": "%s"}`, "round_robin")))
	}
	// grpc Dial 初始化网络模块: addr = resolver.Scheme() + ip + port
	// 1.创建 ClientConn（核心对象）
	// 2.根据 target 的 scheme 选 resolver
	// 3.初始化负载均衡器（balancer）
	// 4.启动 resolver（异步）
	// 5.把地址交给 balancer
	conn, err := grpc.DialContext(context.TODO(), addr, opts...)
	if err != nil {
		logs.Fatal("rpc UserClient connect err: %v", err)
	}
	switch c := client.(type) {
	case *pb.UserServiceClient:
		*c = pb.NewUserServiceClient(conn)
	default:
		logs.Fatal("unsupported client type")
	}
}
