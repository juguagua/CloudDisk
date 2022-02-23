package main

import (
	"bufio"
	"encoding/json"
	"fileStore_server/config"
	dblayer "fileStore_server/db"
	mq "fileStore_server/mq"
	"fileStore_server/store/oss"
	"log"
	"os"
)

// ProcessTransfer : 处理文件转移
func ProcessTransfer(msg []byte) bool {
	log.Println(string(msg))
	// 1.解析msg
	pubData := mq.TransferData{} // 定义消息的结构体
	err := json.Unmarshal(msg, &pubData)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	// 2.根据msg中的信息找到当前文件临时存储的路径，创建文件句柄
	fin, err := os.Open(pubData.CurLocation)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	// 3. 通过文件句柄将文件内容读出来并上传到oss中去
	err = oss.Bucket().PutObject(
		pubData.DestLocation,
		bufio.NewReader(fin))
	if err != nil {
		log.Println(err.Error())
		return false
	}
	// 4.更新文件表信息，将原来的存储路径改为在oss上面的存储路径
	suc := dblayer.UpdateFileLocation( // 更新文件的存储地址
		pubData.FileHash,
		pubData.DestLocation)
	if !suc {
		return false
	}
	return true
}

func main() {
	if !config.AsyncTransferEnable {
		log.Println("异步转移文件功能目前被禁用，请检查相关配置")
		return
	}
	log.Println("文件转移服务启动中，开始监听转移任务队列...")
	mq.StartConsume(
		config.TransOSSQueueName, // 转移队列名
		"transfer_oss",           // 消费者名
		ProcessTransfer)          // callbac
}
