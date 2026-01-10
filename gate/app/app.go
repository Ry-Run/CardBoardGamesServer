package app

import (
	"common/config"
	"common/logs"
	"context"
	"fmt"
	"gate/router"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// Run 启动程序 启动 grpc 服务、启动 http 服务、启动日志、启用数据库
func Run(ctx context.Context) error {
	// 初始化日志
	logs.InitLog(config.Conf.AppName)
	// gin 启动 注册路由
	registerRouter := router.RegisterRouter()
	go func() {
		// 绑定 http 服务器端口
		if err := registerRouter.Run(fmt.Sprintf(":%d", config.Conf.HttpPort)); err != nil {
			logs.Fatal("gate run err: %v", err)
		}
	}()
	// 优雅启停 遇到：中断 退出 中止 挂断信号 先执行清理操作，再退出
	stop := func() {
		// 其他操作
		// 有些操作可能在协程里进行，这里等待 3 秒，尽量让这些操作完成
		time.Sleep(3 * time.Second)
		fmt.Println("stop app finish")
	}
	c := make(chan os.Signal, 1)
	signal.Notify(c, syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT, syscall.SIGHUP)
	for {
		select {
		case <-ctx.Done():
			// timeout
			stop()
		case s := <-c:
			switch s {
			case syscall.SIGTERM, syscall.SIGQUIT, syscall.SIGINT:
				// 中止/退出/中断 信号
				stop()
				logs.Info("user app quit")
				return nil
			case syscall.SIGHUP:
				// 挂断信号 linux 用户注销，该用户的进程需要销毁
				stop()
				logs.Info("hang up, user app quit")
				return nil
			default:
				return nil
			}
		}
	}
}
