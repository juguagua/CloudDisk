package handler

import (
	dblayer "fileStore_server/db"
	"fileStore_server/util"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	// 用于加密的盐值(自定义)
	pwdSalt = "*#890"
	tokenSalt = "_tokensalt"
)

// SignupHandler : 响应注册页面
func SignupHandler(c *gin.Context) {
	c.Redirect(http.StatusFound, "http://"+c.Request.Host+"/static/view/signup.html")
}

// 处理用户注册的post请求
func DoSignupHandler(c *gin.Context) {
	username := c.Request.FormValue("username")
	passwd := c.Request.FormValue("password")

	if len(username) < 3 || len(passwd) < 5 {
		c.JSON(http.StatusOK,
			gin.H{
				"msg": "Invalid parameter",
			})
		return
	}

	// 对密码进行加盐及取Sha1值加密
	encPasswd := util.Sha1([]byte(passwd + pwdSalt))
	// 将用户信息注册到用户表中
	suc := dblayer.UserSignup(username, encPasswd)
	if suc {
		c.JSON(http.StatusOK,
			gin.H{
				"code":    0,
				"msg":     "注册成功",
				"data":    nil,
				"forward": "/user/signin",
			})
	} else {
		c.JSON(http.StatusOK,
			gin.H{
				"code": 0,
				"msg":  "注册失败",
				"data": nil,
			})
	}
}

// SigninHandler : 响应登录页面
func SigninHandler(c *gin.Context) {
	c.Redirect(http.StatusFound, "http://"+c.Request.Host+"/static/view/signin.html")
}

// SignInHandler : 登录接口
func DoSignInHandler(c *gin.Context) {
	username := c.Request.FormValue("username")
	password := c.Request.FormValue("password")

	encPasswd := util.Sha1([]byte(password + pwdSalt))

	// 1. 校验用户名及密码
	pwdChecked := dblayer.UserSignin(username, encPasswd)
	if !pwdChecked {
		c.JSON(http.StatusOK,
			gin.H{
				"code": 0,
				"msg":  "密码校验失败",
				"data": nil,
			})
		return
	}

	// 生成访问凭证（一般两种方式：1.token；2.cookies/session 浏览器端比较常见）这里选第一种
	token := GenToken(username)
	upRes := dblayer.UpdateToken(username, token)
	if !upRes {
		c.JSON(http.StatusOK,
			gin.H{
				"code": 0,
				"msg":  "登录失败",
				"data": nil,
			})
		return
	}

	
	// 登录成功后重定向到首页 并组装返回 username, token 重定向 url 等信息
	resp := util.RespMsg{
		Code: 0,
		Msg:  "OK",
		Data: struct {
			Location string
			Username string
			Token    string
		}{
			Location: "http://" + c.Request.Host + "/static/view/home.html",
			Username: username,
			Token:    token,
		},
	}
	c.Data(http.StatusOK, "octet-stream", resp.JSONBytes())
}

// UserInfoHandler ： 查询用户信息
func UserInfoHandler(c *gin.Context) {
	// 1. 解析请求参数
	username := c.Request.FormValue("username")
	//	token := c.Request.FormValue("token")

	// // 2. 验证token是否有效 校验逻辑使用同一拦截器 HTTPInterceptor 处理
	// isValidToken := IsTokenValid(token)
	// if !isValidToken {
	// 	w.WriteHeader(http.StatusForbidden)
	// 	return
	// }

	// 3. 查询用户信息
	user, err := dblayer.GetUserInfo(username)
	if err != nil {
		c.JSON(http.StatusForbidden,
			gin.H{})
		return
	}

	// 4. 组装并且响应用户数据
	resp := util.RespMsg{
		Code: 0,
		Msg:  "OK",
		Data: user,
	}
	c.Data(http.StatusOK, "octet-stream", resp.JSONBytes())
}

// GenToken : 生成token
func GenToken(username string) string {
	//token(40位字符 md5 后得到的32位字符再加上截图时间戳前8位）生成规则：md5(username+timestamp+tokenSalt)+timestamp[:8]
	ts := fmt.Sprintf("%x", time.Now().Unix())
	tokenPrefix := util.MD5([]byte(username + ts + "_tokensalt"))
	return tokenPrefix + ts[:8]
}

// IsTokenValid : token是否有效
func IsTokenValid(token string) bool {
	if len(token) != 40 {
		return false
	}
	// TODO: 判断token的时效性，是否过期
	// TODO: 从数据库表tbl_user_token查询username对应的token信息
	// TODO: 对比两个token是否一致
	return true
}

// UserExistsHandler ： 查询用户是否存在
func UserExistsHandler(c *gin.Context) {
	// 1. 解析请求参数
	username := c.Request.FormValue("username")

	// 3. 查询用户信息
	exists, err := dblayer.UserExist(username)
	if err != nil {
		c.JSON(http.StatusInternalServerError,
			gin.H{
				"msg": "server error",
			})
	} else {
		c.JSON(http.StatusOK,
			gin.H{
				"msg":    "ok",
				"exists": exists,
			})
	}
}
