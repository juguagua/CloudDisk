package config

import (
	cmn "fileStore_server/common"
)

const (
		// TempLocalRootDir : 本地临时存储地址的路径
		TempLocalRootDir = "./tmp/"
		// 设置当前文件的存储类型
		//CurrentStoreType = cmn.StoreLocal
		CurrentStoreType = cmn.StoreOSS
)