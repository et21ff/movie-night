package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	// ⚠️ 替换为你实际的项目包名
	"movie-night/pkg/mpv"
)

// 必须保持和主程序一致的 Socket 路径
var socketPath = filepath.Join(os.TempDir(), "mpv-socket")

func main() {
	// 1. 初始化控制器
	// 只要主程序运行中，这个 Socket 文件就存在，我们直接连上去
	ctrl := mpv.NewController(socketPath)

	fmt.Println("🎮 [Remote] 远程遥控器已启动")
	fmt.Printf("🔌 连接目标: %s\n", socketPath)
	fmt.Println("-------------------------------------------")
	fmt.Println("命令列表:")
	fmt.Println("  p          -> 暂停/播放")
	fmt.Println("  seek <秒>  -> 跳转 (如: seek 60)")
	fmt.Println("  text <话>  -> 发送弹幕 (如: text 大家好)")
	fmt.Println("  q          -> 退出遥控器")
	fmt.Println("-------------------------------------------")

	reader := bufio.NewReader(os.Stdin)

	for {
		fmt.Print("指令 > ")
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			continue
		}

		parts := strings.SplitN(input, " ", 2)
		cmd := parts[0]
		arg := ""
		if len(parts) > 1 {
			arg = parts[1]
		}

		var err error

		switch cmd {
		case "p":
			fmt.Println("🔄 切换暂停状态...")
			err = ctrl.CyclePause()

		case "seek":
			if arg == "" {
				fmt.Println("❌ 缺少参数，用法: seek 60")
				continue
			}
			sec, parseErr := strconv.ParseFloat(arg, 64)
			if parseErr != nil {
				fmt.Println("❌ 时间格式错误")
				continue
			}
			fmt.Printf("⏩ 跳转到 %.1f 秒\n", sec)
			err = ctrl.Seek(sec, "absolute")
			// 顺便显示一个 OSD 提示
			ctrl.ShowText(fmt.Sprintf("Seek: %.1f", sec), 1000)

		case "text":
			if arg == "" {
				arg = "Hello World"
			}
			fmt.Printf("💬 发送弹幕: %s\n", arg)
			err = ctrl.ShowText(arg, 3000)

		case "q", "exit":
			fmt.Println("👋 退出遥控器")
			return

		default:
			fmt.Println("❓ 未知指令")
		}

		if err != nil {
			fmt.Printf("❌ 执行失败: %v (请确认主程序已启动且 MPV 正在运行)\n", err)
		}
	}
}
