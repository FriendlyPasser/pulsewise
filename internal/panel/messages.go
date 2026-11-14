package panel

type panelMessages struct {
	Lang            string
	Update          string
	CPUUser         string
	CPUSys          string
	CPUIdle         string
	GPUModel        string
	GPUCores        string
	Memory          string
	MemApp          string
	MemWired        string
	MemComp         string
	MemSwap         string
	Disk            string
	DiskScope       string
	DiskAriaLabel   string
	DiskUnavailable string
	DiskUsed        string
	DiskAvailable   string
	Quit            string
}

var messagesZhCN = panelMessages{
	Lang:            "zh-CN",
	Update:          "更新",
	CPUUser:         "用户",
	CPUSys:          "系统",
	CPUIdle:         "空闲",
	GPUModel:        "型号",
	GPUCores:        "核心",
	Memory:          "内存",
	MemApp:          "App",
	MemWired:        "联动",
	MemComp:         "压缩",
	MemSwap:         "交换",
	Disk:            "硬盘",
	DiskScope:       "系统磁盘",
	DiskAriaLabel:   "系统磁盘已用容量",
	DiskUnavailable: "容量暂不可用",
	DiskUsed:        "已用",
	DiskAvailable:   "可用",
	Quit:            "退出",
}

var messagesEn = panelMessages{
	Lang:            "en",
	Update:          "Update",
	CPUUser:         "User",
	CPUSys:          "Sys",
	CPUIdle:         "Idle",
	GPUModel:        "Model",
	GPUCores:        "Cores",
	Memory:          "Memory",
	MemApp:          "App",
	MemWired:        "Wired",
	MemComp:         "Comp",
	MemSwap:         "Swap",
	Disk:            "Disk",
	DiskScope:       "System Disk",
	DiskAriaLabel:   "System disk usage",
	DiskUnavailable: "Capacity unavailable",
	DiskUsed:        "Used",
	DiskAvailable:   "Available",
	Quit:            "Quit",
}

func messagesForLocale(loc Locale) panelMessages {
	if loc.IsZh() {
		return messagesZhCN
	}
	return messagesEn
}
