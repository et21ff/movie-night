package mpv

import (
	"fmt"
	"os/exec"
	"runtime"
)

// CheckMPV 检查 MPV 是否可用
func CheckMPV() error {
	cmd := exec.Command("mpv", "--version")
	output, err := cmd.Output()

	if err != nil {
		if runtime.GOOS == "windows" {
			return fmt.Errorf(`❌ MPV 未安装或不在 PATH 中

Windows 安装方法：
1. 下载: https://mpv.io/installation/
2. 下载 Windows 版本并解压
3. 将 mpv.exe 所在目录添加到 PATH
   或将 mpv.exe 复制到项目目录

当前错误: %v`, err)
		}
		return fmt.Errorf("MPV 未安装: %v", err)
	}

	// 简单的输出检查，确保不是报错信息
	if len(output) > 0 {
		fmt.Printf("✅ MPV 已安装\n")
	}

	return nil
}
