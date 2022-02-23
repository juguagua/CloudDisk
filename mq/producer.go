package mq

import (
	"fileStore_server/config"
	"log"

	"github.com/streadway/amqp"
)

var conn *amqp.Connection  // rabbitMQ的一个连接对象
var channel *amqp.Channel  // 通过channel来进行消息的发布与接收

// 如果异常关闭，会接收通知
var notifyClose chan *amqp.Error

func init() {
	// 是否开启异步转移功能，开启时才初始化rabbitMQ连接
	if !config.AsyncTransferEnable {
		return
	}
	if initChannel() {
		channel.NotifyClose(notifyClose)
	}
	// 断线自动重连
	go func() {
		for {
			select {
			case msg := <-notifyClose:
				conn = nil
				channel = nil
				log.Printf("onNotifyChannelClosed: %+v\n", msg)
				initChannel()
			}
		}
	}()
}
// 创建新channel的方法
func initChannel() bool {
	// 1.判断channel是否已经创建过
	if channel != nil {
		return true
	}
	// 2.获得rabbitMQ的一个连接
	conn, err := amqp.Dial(config.RabbitURL)
	if err != nil {
		log.Println(err.Error())
		return false
	}
	// 3.创建连接后打开一个channel用于消息的发布和接收等
	channel, err = conn.Channel()
	if err != nil {
		log.Println(err.Error())
		return false
	}
	return true
}

// Publish : 发布消息的方法
func Publish(exchange, routingKey string, msg []byte) bool {
	// 1.判断channel是否正常
	if !initChannel() {
		return false
	}
	// 2.调用channel的publish方法，将消息投递到rabbitMQ中去
	if nil == channel.Publish(
		exchange,
		routingKey,
		false, // false表示如果没有对应的queue, 就会丢弃这条消息
		false, 
		amqp.Publishing{
			ContentType: "text/plain",
			Body:        msg}) {
		return true
	}
	return false
}