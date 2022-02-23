// 用于处理一些用于上传的接口
package handler

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	cmn "fileStore_server/common"
	cfg "fileStore_server/config"
	dblayer "fileStore_server/db"
	"fileStore_server/meta"
	mq "fileStore_server/mq"
	"fileStore_server/store/ceph"
	"fileStore_server/store/oss"
	"fileStore_server/util"

	"github.com/gin-gonic/gin"
)
func init() {
	// 目录已存在
	if _, err := os.Stat(cfg.TempLocalRootDir); err == nil {
		return
	}

	// 尝试创建目录
	err := os.MkdirAll(cfg.TempLocalRootDir, 0744)
	if err != nil {
		log.Println("无法创建临时存储目录，程序将退出")
		os.Exit(1)
	}
}
// UploadHandler : 处理用户注册请求
func UploadHandler(c *gin.Context) {
	data, err := ioutil.ReadFile("./static/view/upload.html")
	if err != nil {
		c.String(404, `网页不存在`)
		return
	}
	c.Header("Content-Type", "text/html; charset=utf-8")
	c.String(200, string(data))
}
// 处理文件上传的post请求
func  DoUploadHandler(c *gin.Context) {
	errCode := 0
	defer func() {
		if errCode < 0 {
			c.JSON(http.StatusOK, gin.H{
				"code": errCode,
				"msg":  "Upload failed",
			})
		}
	}()

	// 接收用户要上传的文件流及存储到本地目录

	// 读取文件内容
	file, head, err := c.Request.FormFile("file")
	if err != nil {
		fmt.Printf("Failed to get data, err : %s", err.Error())
		return
	}
	defer file.Close() // 记得关闭文件句柄

	fileMeta := meta.FileMeta{
		FileName: head.Filename,
		Location: "./tmp/" + head.Filename,
		UploadAt: time.Now().Format("2006-01-02 15:04:05"),
	}

	// 创建本地文件newFile接收内容
	newFile, err := os.Create(fileMeta.Location)
	if err != nil {
		fmt.Printf("Failed to create file, err : %s", err.Error())
		errCode = -2
		return
	}
	defer newFile.Close() // 记得关闭文件句柄

	// 将内存中的文件流的内容拷贝到本地文件的buffer区中
	fileMeta.FileSize, err = io.Copy(newFile, file)
	if err != nil {
		fmt.Printf("Failed to save data into file, err : %s", err.Error())
		errCode = -3
		return
	}
	// 将newFile的句柄位置移到最前面，计算文件的Sha1值
	newFile.Seek(0, 0)
	fileMeta.FileSha1 = util.FileSha1(newFile)

	// 在写入到本地存储之后，并且在更新文件表之前，把文件再写一次到ceph或oss中去
	// 游标重新回到文件头部
	newFile.Seek(0, 0)

	if cfg.CurrentStoreType == cmn.StoreCeph {
		// 文件写入Ceph存储
		data, _ := ioutil.ReadAll(newFile)
		cephPath := "/ceph/" + fileMeta.FileSha1
		_ = ceph.PutObject("userfile", cephPath, data)
		fileMeta.Location = cephPath
	} else if cfg.CurrentStoreType == cmn.StoreOSS {
		// 文件写入OSS存储
		ossPath := "oss/" + fileMeta.FileSha1
		// 判断写入OSS为同步还是异步
		if !cfg.AsyncTransferEnable {
			err = oss.Bucket().PutObject(ossPath, newFile)
			if err != nil {
				fmt.Println(err.Error())
				c.JSON(http.StatusInternalServerError,
					gin.H{
						"code": -2,
						"msg":  "Upload failed!",
					})
				return
			}
			fileMeta.Location = ossPath
		} else {
			// 写入异步转移任务队列
			data := mq.TransferData{
				FileHash:      fileMeta.FileSha1,
				CurLocation:   fileMeta.Location,
				DestLocation:  ossPath,
				DestStoreType: cmn.StoreOSS,
			}
			pubData, _ := json.Marshal(data)
			pubSuc := mq.Publish(
				cfg.TransExchangeName,
				cfg.TransOSSRoutingKey,
				pubData,
			)
			if !pubSuc {
				fmt.Println("send failed")
				// TODO: 当前发送转移信息失败，稍后重试
			}
		}
		//将新的fileMeta加入到数据库的fileMeta集合中去
		_ = meta.UpdateFileMetaDB(fileMeta)

		//更新用户文件表记录
		username := c.Request.FormValue("username")
		suc := dblayer.OnUserFileUploadFinished(username, fileMeta.FileSha1,
			fileMeta.FileName, fileMeta.FileSize)
		if suc {
			c.Redirect(http.StatusFound, "/static/view/home.html")
		} else {
			errCode = -5
		}

	}
	
}
// UploadSucHandler : 上传已完成
func UploadSucHandler(c *gin.Context) {
	c.JSON(http.StatusOK,
		gin.H{
			"code": 0,
			"msg":  "Upload Finish!",
		})
}

// GetFileMetaHandler : 获取文件元信息
func GetFileMetaHandler(c *gin.Context) {
	// 解析请求参数
	// 获取参数第一个值
	filehash := c.Request.Form["filehash"][0]
	//fMeta := meta.GetFileMeta(filehash)
	fMeta, err := meta.GetFileMetaDB(filehash)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{
				"code": -2,
				"msg":  "Upload failed!",
			})
		return
	}

	if fMeta != nil {
		data, err := json.Marshal(fMeta)
		if err != nil {
			c.JSON(http.StatusInternalServerError,
				gin.H{
					"code": -3,
					"msg":  "Upload failed!",
				})
			return
		}
		c.Data(http.StatusOK, "application/json", data)
	} else {
		c.JSON(http.StatusOK,
			gin.H{
				"code": -4,
				"msg":  "No sub file",
			})
	}
}

// FileQueryHandler : 查询批量的文件元信息
func FileQueryHandler(c *gin.Context) {

	limitCnt, _ := strconv.Atoi(c.Request.FormValue("limit"))
	username := c.Request.FormValue("username")
	//fileMetas, _ := meta.GetLastFileMetasDB(limitCnt)
	userFiles, err := dblayer.QueryUserFileMetas(username, limitCnt)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{
				"code": -1,
				"msg":  "Query failed!",
			})
		return
	}

	data, err := json.Marshal(userFiles)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{
				"code": -2,
				"msg":  "Query failed!",
			})
		return
	}
	c.Data(http.StatusOK, "application/json", data)
}

// DownloadHandler 根据 filehash 下载文件
func DownloadHandler(c *gin.Context) {
	fsha1 := c.Request.FormValue("filehash")
	
	// 根据下载参数文件的 hash 值查询文件元信息
	fm, _ := meta.GetFileMetaDB(fsha1)

	c.FileAttachment(fm.Location, fm.FileName)
}

// FileUpdateMetaHandler 修改文件元信息
func FileMetaUpdateHandler(c *gin.Context) {

	opType := c.Request.FormValue("op")       // 操作类型
	fileSha1 := c.Request.FormValue("filehash")   // filehash值，标识更新的具体对象
	newFileName := c.Request.FormValue("filename") // 要更换的目标name
	if opType != "0" {
		c.Status(http.StatusForbidden)
		return
	}
	if c.Request.Method != "POST" {
		c.Status(http.StatusMethodNotAllowed)
		return
	}
	curFileMeta := meta.GetFileMeta(fileSha1) // 得到元信息对象
	curFileMeta.FileName = newFileName
	meta.UpdateFileMeta(curFileMeta)
	// TODO: 更新文件表中的元信息记录
	data, err := json.Marshal(curFileMeta)
	if err != nil {
		c.Status(http.StatusInternalServerError)
		return
	}
	c.JSON(http.StatusOK, data)

}

// FileDeleteHandler : 删除文件及元信息
func FileDeleteHandler(c *gin.Context) {
	fileSha1 := c.Request.FormValue("filehash")

	fMeta := meta.GetFileMeta(fileSha1)
	// 删除文件
	os.Remove(fMeta.Location)
	// 删除文件元信息
	meta.RemoveFileMeta(fileSha1)
	// TODO: 删除表文件信息

	c.Status(http.StatusOK)
}

// TryFastUploadHandler : 尝试秒传接口
func TryFastUploadHandler(c *gin.Context) {

	// 1. 解析请求参数
	username := c.Request.FormValue("username")
	filehash := c.Request.FormValue("filehash")
	filename := c.Request.FormValue("filename")
	filesize, _ := strconv.Atoi(c.Request.FormValue("filesize"))

	// 2. 从文件表中查询相同hash的文件记录
	fileMeta, err := meta.GetFileMetaDB(filehash)
	if err != nil {
		fmt.Println(err.Error())
		c.Status(http.StatusInternalServerError)
		return
	}

	// 3. 查不到记录则返回秒传失败
	if fileMeta == nil {
		resp := util.RespMsg{
			Code: -1,
			Msg:  "秒传失败，请访问普通上传接口",
		}
		c.Data(http.StatusOK, "application/json", resp.JSONBytes())
		return
	}

	// 4. 上传过则将文件信息写入用户文件表， 返回成功
	suc := dblayer.OnUserFileUploadFinished(
		username, filehash, filename, int64(filesize))
	if suc {
		resp := util.RespMsg{
			Code: 0,
			Msg:  "秒传成功",
		}
		c.Data(http.StatusOK, "application/json", resp.JSONBytes())
		return
	}
	resp := util.RespMsg{
		Code: -2,
		Msg:  "秒传失败，请稍后重试",
	}
	c.Data(http.StatusOK, "application/json", resp.JSONBytes())
	return
}


// DownloadURLHandler : 生成文件的下载地址
func DownloadURLHandler(c *gin.Context) {
	filehash := c.Request.FormValue("filehash")
	// 从文件表查找记录
	row, _ := dblayer.GetFileMeta(filehash)

	// TODO: 判断文件存在OSS，还是Ceph，还是在本地
	if strings.HasPrefix(row.FileAddr.String, "/tmp") {
		username := c.Request.FormValue("username")
		token := c.Request.FormValue("token")
		tmpUrl := fmt.Sprintf("http://%s/file/download?filehash=%s&username=%s&token=%s",
			c.Request.Host, filehash, username, token)
		c.Data(http.StatusOK, "octet-stream", []byte(tmpUrl))
	} else if strings.HasPrefix(row.FileAddr.String, "/ceph") {
		// TODO: ceph下载url
	} else if strings.HasPrefix(row.FileAddr.String, "oss/") {
		// oss下载url
		signedURL := oss.DownloadURL(row.FileAddr.String)
		c.Data(http.StatusOK, "octet-stream", []byte(signedURL))
	}
}

