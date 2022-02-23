package handler

import (
	"fmt"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/garyburd/redigo/redis"
	"github.com/gin-gonic/gin"

	rPool "fileStore_server/cache/redis"
	dblayer "fileStore_server/db"
	"fileStore_server/util"
)

// MultipartUploadInfo : 初始化信息
type MultipartUploadInfo struct {
	FileHash   string  
	FileSize   int
	UploadID   string // 标记当前分块上传的唯一id 规则：username+当前时间戳
	ChunkSize  int    // 表示分块的大小
	ChunkCount int    // 表示分块的数量，即文件分成多少块来上传
}

// 每块的大小
var chunkSize = 5 * 1024 * 1024 // 5M
var hSetKeyPrefix = "MP_"       // 前缀

// InitialMultipartUploadHandler : 初始化分块上传
func InitialMultipartUploadHandler(c *gin.Context) {
	// 1. 解析用户请求参数
	username := c.Request.FormValue("username")
	filehash := c.Request.FormValue("filehash")
	filesize, err := strconv.Atoi(c.Request.FormValue("filesize"))
	if err != nil {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": -1,
				"msg":  "params invalid",
			})
		return
	}

	// 2. 获得redis的一个连接
	rConn := rPool.RedisPool().Get()
	defer rConn.Close() // 获得连接后记得最后关闭连接

	// 3. 生成分块上传的初始化信息
	upInfo := MultipartUploadInfo{
		FileHash:   filehash,
		FileSize:   filesize,
		UploadID:   username + fmt.Sprintf("%x", time.Now().UnixNano()), // 用户名加当前时间戳
		ChunkSize:  chunkSize,                                  
		ChunkCount: int(math.Ceil((float64(filesize / chunkSize)))),
	}

	// 4. 将初始化信息写入到redis缓存
	rConn.Do("HSET", "MP_"+upInfo.UploadID, "chunkcount", upInfo.ChunkCount)
	rConn.Do("HSET", "MP_"+upInfo.UploadID, "filehash", upInfo.FileHash)
	rConn.Do("HSET", "MP_"+upInfo.UploadID, "filesize", upInfo.FileSize)

	// 5. 将响应初始化数据返回到客户端
	c.JSON(
		http.StatusOK,
		gin.H{
			"code": 0,
			"msg":  "OK",
			"data": upInfo,
		})
}

// UploadPartHandler : 上传文件分块
func UploadPartHandler(c *gin.Context) {
	// 1. 解析用户请求参数

	//	username := c.Request.FormValue("username")
	uploadID := c.Request.FormValue("uploadid")
	chunkIndex := c.Request.FormValue("index")

	// 2. 获得redis连接池中的一个连接
	rConn := rPool.RedisPool().Get()
	defer rConn.Close()

	// 3. 获得文件句柄，用于存储分块内容
	fpath := util.GetCurrentFielParentPath() + "/tmp/" + uploadID + "/" + chunkIndex
	
	// 数字设定法：：0表示没有权限，1表示可执行权限，2表示可写权限，4表示可读权限，然后将其相加。设置当前用户可读可写可执行权限
	os.MkdirAll(path.Dir(fpath), 0744) // 创建目录，设置权限0744
	fd, err := os.Create(fpath)
	if err != nil {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": 0,
				"msg":  "Upload part failed",
				"data": nil,
			})
		return
	}
	defer fd.Close()

	// 3.读取内存中的分块内容写入到文件中
	buf := make([]byte, 1024*1024)
	for {
		n, err := c.Request.Body.Read(buf)
		fd.Write(buf[:n])
		if err != nil {
			break
		}
	}

	// 4. 更新redis缓存状态
	rConn.Do("HSET", hSetKeyPrefix+uploadID, "chkidx_"+chunkIndex, 1)

	// 5. 返回处理结果到客户端
	c.JSON(
		http.StatusOK,
		gin.H{
			"code": 0,
			"msg":  "OK",
			"data": nil,
		})
}

// CompleteUploadHandler : 通知上传合并
func CompleteUploadHandler(c *gin.Context) {
	// 1. 解析请求参数
	upid := c.Request.FormValue("uploadid")
	username := c.Request.FormValue("username")
	filehash := c.Request.FormValue("filehash")
	filesize := c.Request.FormValue("filesize")
	filename := c.Request.FormValue("filename")

	// 2. 获得redis连接池中的一个连接
	rConn := rPool.RedisPool().Get()
	defer rConn.Close()

	// 3. 通过uploadid查询redis并判断是否所有分块上传完成
	// 查询redis中的hashSet，之前每上传一块都会往redis中写一条记录，所以写入到redis中的分块数量是等于切分的总数量的
	data, err := redis.Values(rConn.Do("HGETALL", hSetKeyPrefix+upid))
	if err != nil {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": -1,
				"msg":  "OK",
				"data": nil,
			})
		return
	}
	totalCount := 0
	chunkCount := 0      // 实际查出来的分块数量
	for i := 0; i < len(data); i += 2 { // 因为通过HGETALL查出来的key和value是在同一个arr中的，所以每次i+2
		k := string(data[i].([]byte))
		v := string(data[i+1].([]byte))
		if k == "chunkcount" { // 如果 k 表示文件总共分成的块的数量
			totalCount, _ = strconv.Atoi(v) // 就将相应的val值赋给totalCount
		} else if strings.HasPrefix(k, "chkidx_") && v == "1" { // 判断key是否以chkidx开头，如果是就说明这一条记录是标志每个块已经完成的
			chunkCount++
		}
	}
	if totalCount != chunkCount {
		c.JSON(
			http.StatusOK,
			gin.H{
				"code": -2,
				"msg":  "OK",
				"data": nil,
			})
		return
	}

	// 4.合并分块
	fpath := util.GetCurrentFielParentPath() + "/tmp/" + upid + "/"

	resultFile := fpath + filename
	fil, err := os.OpenFile(resultFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, os.ModePerm)
	if err != nil {
		panic(err)
		return
	}
	defer fil.Close()

	for i := 1; i <= chunkCount; i++ {
		fname := fpath + strconv.Itoa(i)
		f, err := os.OpenFile(fname, os.O_RDONLY, os.ModePerm)
		if err != nil {
			fmt.Printf("打开文件[%s]失败: %s", fname, err.Error())
		}

		bytes, err := ioutil.ReadAll(f)
		if err != nil {
			fmt.Printf("读取数据失败: %s", err.Error())
		}

		fil.Write(bytes)
		f.Close()
	}

	// 写入完成，删除分块文件
	for i := 1; i <= chunkCount; i++ {
		fname := fpath + strconv.Itoa(i)
		err := os.Remove(fname)
		if err != nil {
			fmt.Printf("分块文件[%s]删除失败,err: %s", fname, err.Error())
		}
	}
	
	// 5. 更新唯一文件表及用户文件表
	fsize, _ := strconv.Atoi(filesize)
	dblayer.OnFileUploadFinished(filehash, filename, int64(fsize), "")
	dblayer.OnUserFileUploadFinished(username, filehash, filename, int64(fsize))

	// 6. 响应处理结果
	c.JSON(
		http.StatusOK,
		gin.H{
			"code": 0,
			"msg":  "OK",
			"data": nil,
		})
}
