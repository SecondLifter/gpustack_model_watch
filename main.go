package main

import (
	"GPUSTACK_WATCH/services"
	"flag"
	"log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	// 命令行参数
	baseURL := flag.String("url", "http://127.0.0.1", "API基础URL")
	username := flag.String("username", "admin", "API用户名")
	password := flag.String("password", "123422", "API密码")
	flag.Parse()

	if *username == "" || *password == "" {
		log.Fatal("请提供用户名和密码")
	}

	// 创建服务实例
	modelService := services.NewModelService(*baseURL, *username, *password)

	// 尝试首次登录
	if err := modelService.Login(); err != nil {
		log.Fatalf("初始登录失败: %v", err)
	}

	// 创建信号通道
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)

	// 启动监控协程
	go modelService.WatchErrorModels()

	// 等待退出信号
	<-sigChan
	log.Println("收到退出信号，程序退出")
}
