package discovery

import (
	"common/config"
	"common/logs"
	"context"
	"encoding/json"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
)

// Register
// @Description: grpc 服务注册到 etcd。
// 在etcd创建一个租约，将 grpc 服务注册到 etcd，绑定该租约。
// 过了租约时间，etcd 会删除 grpc 服务信息。
// 实现心跳续租，没有就新注册
type Register struct {
	etcdClt     *clientv3.Client                        // etcd 连接
	leaseId     clientv3.LeaseID                        // 租约 Id
	DialTimeout int                                     // 超时时间
	ttl         int                                     // 租约时间
	keepAliveCh <-chan *clientv3.LeaseKeepAliveResponse // 心跳
	info        Server                                  // 注册的 Server 信息
	closeCh     chan struct{}
}

func NewRegister() *Register {
	return &Register{
		DialTimeout: 3,
	}
}

func (r *Register) Register(conf config.EtcdConf) error {
	// 注册信息
	info := Server{
		Name:    conf.Register.Name,
		Addr:    conf.Register.Addr,
		Weight:  conf.Register.Weight,
		Version: conf.Register.Version,
		Ttl:     conf.Register.Ttl,
	}
	// 建立 etcd 连接
	var err error
	r.etcdClt, err = clientv3.New(clientv3.Config{
		Endpoints:   conf.Addrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		return err
	}
	r.info = info
	// 连接建立成功，开始注册
	if err = r.register(); err != nil {
		return err
	}
	r.closeCh = make(chan struct{})
	// 在协程中，根据心跳的结果，做相应的操作
	go r.watcher()
	return nil
}

func (r *Register) register() error {
	// 1.创建租约
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.DialTimeout)*time.Second)
	defer cancel()
	var err error
	if err := r.createLease(ctx, r.info.Ttl); err != nil {
		return err
	}
	// 2.心跳检测
	if r.keepAliveCh, err = r.keepAlive(); err != nil {
		return err
	}
	// 3.绑定租约
	data, _ := json.Marshal(r.info)
	return r.bindLease(ctx, r.info.BuildRegisterKey(), string(data))
}

// createLease
//
//	@Description: 创建租约
//	@receiver register
//	@param ctx
//	@param ttl 租约时间单位秒
//	@return error
func (r *Register) createLease(ctx context.Context, ttl int64) error {
	grant, err := r.etcdClt.Grant(ctx, ttl)
	if err != nil {
		logs.Error("createLease failed, err: %v", err)
		return err
	}
	r.leaseId = grant.ID
	return nil
}

// bindLease
//
//	@Description: 绑定租约
//	@receiver register
//	@param ctx
//	@param key 存在 etcd 的 key
//	@param value etcd 对应 key 的 value
//	@return error
func (r *Register) bindLease(ctx context.Context, key, value string) error {
	// Put 操作，携带租约 id
	_, err := r.etcdClt.Put(ctx, key, value, clientv3.WithLease(r.leaseId))
	if err != nil {
		logs.Error("bindLease Put failed, err: %v", err)
		return err
	}
	logs.Info("register service success, key: %v", key)
	return nil
}

// keepAlive
//
//	@Description: 创建心跳检测
//	@receiver register
//	@param ctx
//	@return error
func (r *Register) keepAlive() (<-chan *clientv3.LeaseKeepAliveResponse, error) {
	// KeepAlive 是一个长连接，如果 ctx 做了超时，长连接就断掉了
	// 一直发心跳给 etcd，保持 lease
	keepAliveResp, err := r.etcdClt.KeepAlive(context.Background(), r.leaseId)
	if err != nil {
		logs.Error("keepAlive failed, err: %v", err)
		return keepAliveResp, err
	}
	return keepAliveResp, nil
}

// watcher
//
//	@Description: 监听心跳检测响应，有租约就续约，没有就新注册，close 注销注册
func (r *Register) watcher() {
	// 租约到期 检查是否需要自动注册
	ticker := time.NewTicker(time.Duration(r.info.Ttl) * time.Second)
	for {
		select {
		case <-r.closeCh:
			if err := r.unregister(); err != nil {
				logs.Error("close and unregister failed, err: %v", err)
			}
			logs.Info("unregister etcd")
			if r.etcdClt != nil {
				r.etcdClt.Close()
			}
		case res, ok := <-r.keepAliveCh:
			// etcd 重启/网络中断等导致 keepalive 的 gRPC stream 失败时，clientv3 会 close keepAliveCh；
			// 从已关闭的 chan *LeaseKeepAliveResponse 接收会得到 nil（或 ok=false），需要重新注册/重建 keepalive。
			if !ok || res == nil {
				if err := r.register(); err != nil {
					logs.Error("ticker register failed, err: %v", err)
				}
				logs.Info("续约重新注册成功, res=%v", res)
			}
		case <-ticker.C:
			if r.keepAliveCh == nil {
				if err := r.register(); err != nil {
					logs.Error("ticker register failed, err: %v", err)
				}
			}
		}
	}

}

// unregister
//
//	@Description: 注销 etcd 注册，1.删除 key；2.撤销租约
func (r *Register) unregister() error {
	// 删除
	if _, err := r.etcdClt.Delete(context.Background(), r.info.BuildRegisterKey()); err != nil {
		logs.Error("unregister and Delete etcd key failed, err: %v", err)
		return err
	}
	// 租约撤销
	if _, err := r.etcdClt.Revoke(context.Background(), r.leaseId); err != nil {
		logs.Error("unregister and Revoke lease failed, err: %v", err)
		return err
	}
	return nil
}

// Close
//
//	@Description: 关闭 Register
func (r *Register) Close() {
	r.closeCh <- struct{}{}
}
