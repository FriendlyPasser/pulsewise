package panel

type panelMessages struct {
	Lang                   string
	Update                 string
	CPUUser                string
	CPUSys                 string
	CPUIdle                string
	GPUModel               string
	GPUCores               string
	Memory                 string
	MemApp                 string
	MemWired               string
	MemComp                string
	MemSwap                string
	MemOpenDetail          string
	MemDetailTitle         string
	MemAppsHeading         string
	MemUnattributedHeading string
	MemNoteBasis           string
	MemNoteAttribution     string
	MemNoteUnattributed    string
	Disk                   string
	DiskScope              string
	DiskAriaLabel          string
	DiskUnavailable        string
	DiskUsed               string
	DiskAvailable          string
	Quit                   string
}

var messagesZhCN = panelMessages{
	Lang:                   "zh-CN",
	Update:                 "更新",
	CPUUser:                "用户",
	CPUSys:                 "系统",
	CPUIdle:                "空闲",
	GPUModel:               "型号",
	GPUCores:               "核心",
	Memory:                 "内存",
	MemApp:                 "App",
	MemWired:               "联动",
	MemComp:                "压缩",
	MemSwap:                "交换",
	MemOpenDetail:          "查看应用内存详情",
	MemDetailTitle:         "应用内存详情",
	MemAppsHeading:         "应用占用",
	MemUnattributedHeading: "未归属应用的进程",
	MemNoteBasis:           "统计口径：按 macOS“活动监视器”口径统计当前用户各进程的物理内存（phys_footprint）。",
	MemNoteAttribution:     "归属方式：按可执行文件最外层 .app 包、macOS 责任进程及父进程链，将渲染器、插件与 XPC 辅助进程合并到所属应用。",
	MemNoteUnattributed:    "无法归属的进程：不属于任何 .app 包的后台服务与命令行进程列于“未归属应用的进程”；其他系统账户及内核联动内存因无需 root 权限而不纳入单进程统计。",
	Disk:                   "硬盘",
	DiskScope:              "系统磁盘",
	DiskAriaLabel:          "系统磁盘已用容量",
	DiskUnavailable:        "容量暂不可用",
	DiskUsed:               "已用",
	DiskAvailable:          "可用",
	Quit:                   "退出",
}

var messagesEn = panelMessages{
	Lang:                   "en",
	Update:                 "Update",
	CPUUser:                "User",
	CPUSys:                 "Sys",
	CPUIdle:                "Idle",
	GPUModel:               "Model",
	GPUCores:               "Cores",
	Memory:                 "Memory",
	MemApp:                 "App",
	MemWired:               "Wired",
	MemComp:                "Comp",
	MemSwap:                "Swap",
	MemOpenDetail:          "View app memory details",
	MemDetailTitle:         "App Memory Details",
	MemAppsHeading:         "Applications",
	MemUnattributedHeading: "Unattributed Processes",
	MemNoteBasis:           "Metric: per-process physical memory footprint (phys_footprint) for the current user, matching Activity Monitor.",
	MemNoteAttribution:     "Attribution: renderer, plugin, and XPC helper processes are merged into their owning app via outermost .app bundle path, responsible PID, and parent PID chain.",
	MemNoteUnattributed:    "Unattributed processes: background services and CLI tools outside .app bundles are listed under Unattributed Processes; other system accounts and kernel wired memory are excluded without root.",
	Disk:                   "Disk",
	DiskScope:              "System Disk",
	DiskAriaLabel:          "System disk usage",
	DiskUnavailable:        "Capacity unavailable",
	DiskUsed:               "Used",
	DiskAvailable:          "Available",
	Quit:                   "Quit",
}

func messagesForLocale(loc Locale) panelMessages {
	if loc.IsZh() {
		return messagesZhCN
	}
	return messagesEn
}
