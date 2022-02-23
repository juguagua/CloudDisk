package main

import (
	"fileStore_server/config"
	"fileStore_server/route"
	"fmt"
)

func main() {
	// gin framework
	router := route.Router()

	// 启动服务并监听端口
	err := router.Run(config.UploadServiceHost)
	if err != nil {
		fmt.Printf("Failed to start server, err:%s\n", err.Error())
	}
}