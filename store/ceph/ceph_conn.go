package ceph

import (
	"gopkg.in/amz.v1/aws"
	"gopkg.in/amz.v1/s3"

	cfg "fileStore_server/config"
)

var cephConn *s3.S3

// GetCephConnection : 获取ceph连接
func GetCephConnection() *s3.S3 {
	if cephConn != nil {
		return cephConn
	}
	// 1. 初始化ceph的一些信息
	// 权限相关的认证，后续通过两个key来进行上传下载等相关操作
	auth := aws.Auth{
		AccessKey: cfg.CephAccessKey,
		SecretKey: cfg.CephSecretKey,
	}
	// 定义s3相关的元素
	curRegion := aws.Region{
		Name:                 "default",
		EC2Endpoint:          cfg.CephGWEndpoint,   // 服务器相关的地址
		S3Endpoint:           cfg.CephGWEndpoint,
		S3BucketEndpoint:     "",
		S3LocationConstraint: false,   // 区域限制
		S3LowercaseBucket:    false,   
		Sign:                 aws.SignV2, // 签名算法
	}

	// 2. 创建S3类型的连接
	return s3.New(auth, curRegion)
}

// GetCephBucket : 获取指定的bucket对象
func GetCephBucket(bucket string) *s3.Bucket {
	conn := GetCephConnection()   // 先获取连接 
	return conn.Bucket(bucket)
}

// PutObject : 上传文件到ceph集群
func PutObject(bucket string, path string, data []byte) error {
	return GetCephBucket(bucket).Put(path, data, "octet-stream", s3.PublicRead)
}