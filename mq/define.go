package mq

import (
	cmn "fileStore_server/common"
)

// TransferData : 将要写到rabbitmq的数据的结构体
type TransferData struct {
	FileHash      string   // 将要被转移的文件的hash值
	CurLocation   string   // 存在临时存储里的具体地址
	DestLocation  string   // 要转移的目标地址
	DestStoreType cmn.StoreType  // 文件将要被转移到哪种类型的存储里面
}