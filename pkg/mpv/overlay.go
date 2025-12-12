package mpv

import (
	"fmt"
	"sort"
	"strings"
)

// PeerSyncState 定义参与者的同步状态
type PeerSyncState struct {
	Name       string // 节点名称或ID
	IsReady    bool   // 是否已就绪
	Buffering  int    // 缓冲进度百分比 (0-100)
	StatusText string // 额外的状态文本 (如 "Buffering", "Seeked")
}

// DrawSyncOverlay 在屏幕上绘制同步状态面板
func (c *Controller) DrawSyncOverlay(states map[string]PeerSyncState) error {
	// 1. 构建 ASS 内容
	var sb strings.Builder

	// 标题样式: 居中(\an5), 字号48(\fs48), 加粗(\b1), 亮青色(\c&HFFFF00&) - 注意ASS颜色是BGR
	// 这里使用简单的白色或亮色作为标题
	sb.WriteString(`{\an5\fs48\b1\c&HFFFFFF&}Sync Status{\N}{\fs30\b0}`) // 标题后换行，并重置字号

	// 空行
	sb.WriteString(`{\N}`)

	// 2. 对 states 进行排序，保证显示顺序稳定
	// 将 map 转换为 slice 以便排序
	type item struct {
		Name  string
		State PeerSyncState
	}
	items := make([]item, 0, len(states))
	for name, state := range states {
		items = append(items, item{Name: name, State: state})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Name < items[j].Name
	})

	// 3. 生成列表项
	for _, it := range items {
		s := it.State
		// 名字
		sb.WriteString(fmt.Sprintf(`{\c&HFFFFFF&}%s: `, s.Name))

		if s.IsReady {
			// Ready: 绿色 (\c&H00FF00&)
			sb.WriteString(`{\c&H00FF00&}Ready`)
		} else {
			// Not Ready: 黄色 (\c&H00FFFF&)
			sb.WriteString(fmt.Sprintf(`{\c&H00FFFF&}%s (%d%%)`, s.StatusText, s.Buffering))
		}
		// 换行
		sb.WriteString(`{\N}`)
	}

	assContent := sb.String()

	// 4. 发送 IPC 命令
	// 命令格式: ["osd-overlay", <overlay_id>, "ass-events", <ass_content_string>]
	// overlay_id = 1
	return c.sendCommand("osd-overlay", 1, "ass-events", assContent)
}

// ClearSyncOverlay 清除同步状态面板
func (c *Controller) ClearSyncOverlay() error {
	// 发送空字符串以清除
	return c.sendCommand("osd-overlay", 1, "ass-events", "")
}
