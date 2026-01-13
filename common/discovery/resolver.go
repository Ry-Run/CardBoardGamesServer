package discovery

import (
	"common/config"
	"common/logs"
	"context"
	"time"

	clientv3 "go.etcd.io/etcd/client/v3"
	"google.golang.org/grpc/attributes"
	"google.golang.org/grpc/resolver"
)

type Resolver struct {
	conf        config.EtcdConf
	etcdClt     *clientv3.Client // etcd 连接
	DialTimeout int              // 超时时间
	closeCh     chan struct{}
	key         string
	cc          resolver.ClientConn
	serverAddrs []resolver.Address
	watchCh     clientv3.WatchChan
}

func NewResolver(conf config.EtcdConf) *Resolver {
	return &Resolver{
		conf:        conf,
		DialTimeout: conf.DialTimeout,
	}
}

// grpc 在 Dial 时同步调用 Build，创建 Resolver
func (r *Resolver) Build(target resolver.Target, cc resolver.ClientConn, opts resolver.BuildOptions) (resolver.Resolver, error) {
	// resolver.ClientConn 把解析到的地址列表推给 gRPC
	r.cc = cc
	// 1. 连接 etcd
	var err error
	r.etcdClt, err = clientv3.New(clientv3.Config{
		Endpoints:   r.conf.Addrs,
		DialTimeout: time.Duration(r.DialTimeout) * time.Second,
	})
	if err != nil {
		logs.Fatal("grpc client connect etcd, err=%v", err)
	}
	r.closeCh = make(chan struct{})
	// 2.根据 key 获取 value, 相当于 user/v1
	// target.URL.Path 相当于 grpc.DialContext() 传入的 target 去掉 Scheme 前缀
	r.key = target.URL.Path
	err = r.sync()
	if err != nil {
		return nil, err
	}
	// 3.如果节点有变动 实时更新信息
	go r.watch()
	return nil, nil
}

// Scheme 返回解析器的名字，Dial 时根据指定解析器调用 Build
func (r *Resolver) Scheme() string {
	return "etcd"
}

// sync 从 etcd 获取服务器列表，并更新到解析器
func (r *Resolver) sync() error {
	// 带读写超时时间的 context，避免同步阻塞代码累积大量线程
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(r.conf.RWTimeout)*time.Second)
	defer cancel()
	// 根据前缀获取，例如 /user/v1/*
	values, err := r.etcdClt.Get(ctx, r.key, clientv3.WithPrefix())
	if err != nil {
		logs.Error("grpc client get etcd failed: key=%v, err=%v", r.key, err)
		return err
	}
	r.serverAddrs = []resolver.Address{}
	for k, v := range values.Kvs {
		server, err := ParseValue(v.Value)
		if err != nil {
			logs.Error("grpc client parse etcd value failed: key=%v, err=%v", k, err)
			continue
		}
		r.serverAddrs = append(r.serverAddrs, resolver.Address{
			Addr:       server.Addr,
			Attributes: attributes.New("weight", server.Weight),
		})
	}
	if len(r.serverAddrs) == 0 {
		// 服务器可能还在启动未注册到 etcd，或者其他原因
		logs.Error("no services found")
		return nil
	}
	// 更新 grpc 地址
	err = r.cc.UpdateState(resolver.State{
		Addresses: r.serverAddrs,
	})
	if err != nil {
		logs.Error("grpc client UpdateState failed: key=%v, err=%v", r.key, err)
		return err
	}

	return nil
}

func (r *Resolver) watch() {
	// 1.定时 1 分钟同步一次数据
	ticker := time.NewTicker(time.Minute)
	// 2.监听节点的事件 从而触发不同的操作
	r.watchCh = r.etcdClt.Watch(context.Background(), r.key, clientv3.WithPrefix())
	// 3.监听 close 事件，关闭 etcd 客户端
	select {
	case <-ticker.C:
		if err := r.sync(); err != nil {
			logs.Error("watch sync failed, err=%v", err)
		}
	case resp, ok := <-r.watchCh:
		if ok {
			r.update(resp.Events)
		}
	case <-r.closeCh:
		r.close()
	}
}

func (r *Resolver) update(events []*clientv3.Event) {
	for _, event := range events {
		switch event.Type {
		case clientv3.EventTypePut:
			server, err := ParseKey(string(event.Kv.Key))
			if err != nil {
				logs.Error("grpc client update(EventTypePut) parse etcd value failed: key=%v, err=%v", r.key, err)
				continue
			}
			// 不能重复添加，否则影响负载均衡
			if Exist(r.serverAddrs, server.Addr) {
				r.serverAddrs = append(r.serverAddrs, resolver.Address{
					Addr:       server.Addr,
					Attributes: attributes.New("weight", server.Weight),
				})
				err = r.cc.UpdateState(resolver.State{
					Addresses: r.serverAddrs,
				})
				if err != nil {
					logs.Error("grpc client update(EventTypePut) UpdateState failed: key=%v, err=%v", r.key, err)
				}
			}
		case clientv3.EventTypeDelete:
			server, err := ParseKey(string(event.Kv.Key))
			if err != nil {
				logs.Error("grpc client update(EventTypeDelete) parse etcd value failed: key=%v, err=%v", r.key, err)
				continue
			}
			if list, ok := Remove(r.serverAddrs, server.Addr); ok {
				r.serverAddrs = list
				err = r.cc.UpdateState(resolver.State{
					Addresses: r.serverAddrs,
				})
				if err != nil {
					logs.Error("grpc client update(EventTypeDelete) UpdateState failed: key=%v, err=%v", r.key, err)
				}
			}
		}
	}
}

func (r *Resolver) close() {
	if r.etcdClt != nil {
		err := r.etcdClt.Close()
		if err != nil {
			logs.Error("Resolver close etcd client err=%v", err)
		}
	}
}

func Exist(addrs []resolver.Address, addr string) bool {
	for i := range addrs {
		if addrs[i].Addr == addr {
			return false
		}
	}
	return true
}

func Remove(addrs []resolver.Address, addr string) ([]resolver.Address, bool) {
	for i := range addrs {
		if addrs[i].Addr == addr {
			addrs[i] = addrs[len(addrs)-1]
			return addrs[:len(addrs)-1], true
		}
	}
	return nil, false
}
