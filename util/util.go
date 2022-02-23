package util

// 根据传入的文件流/句柄来计算文件的哈希值，包括sha1,md5，文件大小等
import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

var CstZone = time.FixedZone("CST", 8*3600) // 东八

type Sha1Stream struct {
	_sha1 hash.Hash
}

func (obj *Sha1Stream) Update(data []byte) {
	if obj._sha1 == nil {
		obj._sha1 = sha1.New()
	}
	obj._sha1.Write(data)
}

func (obj *Sha1Stream) Sum() string {
	return hex.EncodeToString(obj._sha1.Sum([]byte("")))
}

func Sha1(data []byte) string {
	_sha1 := sha1.New()
	_sha1.Write(data)
	return hex.EncodeToString(_sha1.Sum([]byte("")))
}

func FileSha1(file *os.File) string {
	_sha1 := sha1.New()
	io.Copy(_sha1, file)
	return hex.EncodeToString(_sha1.Sum(nil))
}

func MD5(data []byte) string {
	_md5 := md5.New()
	_md5.Write(data)
	return hex.EncodeToString(_md5.Sum([]byte("")))
}

func FileMD5(file *os.File) string {
	_md5 := md5.New()
	io.Copy(_md5, file)
	return hex.EncodeToString(_md5.Sum(nil))
}

// PathExists 判断文件是否存在
func PathExists(path string) (bool, error) {
	_, e := os.Stat(path)
	if e == nil {
		return true, nil
	}

	if os.IsNotExist(e) {
		return false, nil
	}
	return false, e
}

// GetFileSize 获取文件大小
func GetFileSize(filename string) int64 {
	var result int64

	filepath.Walk(filename, func(path string, info os.FileInfo, err error) error {

		result = info.Size()
		return nil
	})
	return result
}

// GetCurrentFilePath 获取当前执行文件绝对路径
func GetCurrentFilePath() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic(" Can not get current file info")
	}
	lastIndex := strings.LastIndex(file, "/") + 1
	file = file[:lastIndex]
	return file
}

// GetCurrentFielParentPath 获取当前执行文件父类路径
func GetCurrentFielParentPath() string {
	_, file, _, ok := runtime.Caller(1)
	if !ok {
		panic(" Can not get current file info")
	}
	lastIndex := strings.LastIndex(file, "/")
	file = file[:lastIndex]
	lastIndex = strings.LastIndex(file, "/") + 1
	parentPath := file[:lastIndex]
	fmt.Println(parentPath)
	return parentPath
}
