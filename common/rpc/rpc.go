package rpc

import (
	"common/config"
	"common/logs"
	"context"
	"fmt"
	"user/pb"

	"google.golang.org/grpc"
)

var (
	UserClient pb.UserServiceClient
)

func Init() {
	// etcd 解析器

	userDomain := config.Conf.Domain["user"]
	initClt(userDomain.Name, userDomain.LoadBalance, &UserClient)
}

func initClt(name string, loadBalance bool, client interface{}) {
	// etcd 上服务的地址
	addr := fmt.Sprintf("etcd:///%s", name)
	// 建立连接
	conn, err := grpc.DialContext(context.TODO(), addr)
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
