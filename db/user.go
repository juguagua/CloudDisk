package db

import (
	mydb "fileStore_server/db/mysql"
	"fmt"
)

type User struct {
	Username     string
	Email        string
	Phone        string
	SignupAt     string
	LastActiveAt string
	Status       int
}

// UserSingup : 通过用户名和密码完成user表的注册操作
func UserSignup(username string, password string) bool {
	stmt, err := mydb.DBConn().Prepare("insert ignore into tbl_user(`user_name`, `user_pwd`) values (?, ?)")
	if err != nil {
		fmt.Printf("Failed to insert, err : %s", err.Error())
		return false
	}
	defer stmt.Close()

	ret, err := stmt.Exec(username, password)
	if err != nil {
		fmt.Printf("Failed to insert , err : %s ", err.Error())
		return false
	}
	// 多一步判断是否重复注册
	if rowsAffected, err := ret.RowsAffected(); nil == err && rowsAffected > 0 {
		return true
	}
	return false
}

// UserSignin : 判断密码是否一致进行登录
func UserSignin(username string, encpwd string) bool {
	stmt, err := mydb.DBConn().Prepare("select * from tbl_user where user_name=? limit 1") // 查询操作接口生成
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	defer stmt.Close()

	rows, err := stmt.Query(username)
	if err != nil {
		fmt.Println(err.Error())
		return false
	} else if rows == nil {
		fmt.Println("username not found: " + username)
		return false
	}
	// 如果查询到了用户数据，检查两个password是否一致
	pRows := mydb.ParseRows(rows)
	if len(pRows) > 0 && string(pRows[0]["user_pwd"].([]byte)) == encpwd {
		return true
	}
	return false
}

// UpdateToken : 刷新用户登录的token
func UpdateToken(username string, token string) bool {
	stmt, err := mydb.DBConn().Prepare(
		"replace into tbl_user_token (`user_name`,`user_token`) values (?,?)") // 如果有旧的token可以将之覆盖
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	defer stmt.Close()

	_, err = stmt.Exec(username, token)
	if err != nil {
		fmt.Println(err.Error())
		return false
	}
	return true
}


// GetUserInfo : 查询用户信息
func GetUserInfo(username string) (User, error) {
	user := User{}

	stmt, err := mydb.DBConn().Prepare(
		"select user_name,signup_at from tbl_user where user_name=? limit 1")
	if err != nil {
		fmt.Println(err.Error())
		return user, err
	}
	defer stmt.Close()

	// 执行查询的操作， Scan函数可以将数据库查到的记录转换为外部的一个变量来存储
	err = stmt.QueryRow(username).Scan(&user.Username, &user.SignupAt)
	if err != nil {
		return user, err
	}
	return user, nil
}

// UserExist : 查询用户是否存在
func UserExist(username string) (bool, error) {

	stmt, err := mydb.DBConn().Prepare(
		"select 1 from tbl_user where user_name=? limit 1")
	if err != nil {
		fmt.Println(err.Error())
		return false, err
	}
	defer stmt.Close()

	rows, err := stmt.Query(username)
	if err != nil {
		return false, err
	}
	return rows.Next(), nil
}
