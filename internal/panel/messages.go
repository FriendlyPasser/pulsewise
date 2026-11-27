package panel

type panelMessages struct {
	Lang                   string
	Update                 string
	CPUUser                string
	CPUSys                 string
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
	DiskOpenDetail         string
	DiskDetailTitle        string
	DiskScopeLabel         string
	DiskChooseFolder       string
	DiskStartScan          string
	DiskCancelScan         string
	DiskFilesHeading       string
	DiskNoteBasis          string
	DiskNoteAPFS           string
	DiskNotePermissions    string
	Network                string
	NetDown                string
	NetUp                  string
	NetPeak                string
	Quit                   string
}

var messagesZhCN = panelMessages{
	Lang:                   "zh-CN",
	Update:                 "更新",
	CPUUser:                "用户",
	CPUSys:                 "系统",
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
	DiskOpenDetail:         "查看磁盘大文件详情",
	DiskDetailTitle:        "磁盘大文件详情",
	DiskScopeLabel:         "扫描目录",
	DiskChooseFolder:       "选择目录…",
	DiskStartScan:          "开始扫描",
	DiskCancelScan:         "取消扫描",
	DiskFilesHeading:       "大文件列表",
	DiskNoteBasis:          "统计口径：按文件实际磁盘占用空间（st_blocks × 512，十进制单位）排序并展示前 100 项；跳过符号链接与其他挂载卷，硬链接文件在总计中不重复计数，未占用本地空间的云端文件不计入。",
	DiskNoteAPFS:           "APFS 容量说明：顶部系统磁盘“已用”来自 APFS 容器可用空间（statfs），包含系统/虚拟内存等共享卷、本地时间机器快照、可清除空间及元数据；APFS 写时复制克隆共享磁盘块，因此文件大小之和无法直接等于已用容量。",
	DiskNotePermissions:    "权限与完整性：扫描仅使用当前用户权限（无需 root 或完全磁盘访问权限）。被系统隐私或文件权限阻止的目录会跳过并在结果中标明。",
	Network:                "网络",
	NetDown:                "下行",
	NetUp:                  "上行",
	NetPeak:                "峰值",
	Quit:                   "退出",
}

var messagesEn = panelMessages{
	Lang:                   "en",
	Update:                 "Update",
	CPUUser:                "User",
	CPUSys:                 "Sys",
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
	DiskOpenDetail:         "View large disk files",
	DiskDetailTitle:        "Large Disk Files",
	DiskScopeLabel:         "Scan Folder",
	DiskChooseFolder:       "Choose Folder…",
	DiskStartScan:          "Scan",
	DiskCancelScan:         "Cancel",
	DiskFilesHeading:       "Largest Files",
	DiskNoteBasis:          "Metric: ranks the top 100 files by actual allocated disk space (st_blocks × 512, decimal units). Symbolic links and other mounted volumes are skipped, hard links are counted once in totals, and cloud-only dataless files are excluded.",
	DiskNoteAPFS:           "APFS capacity: System Disk Used comes from APFS container availability (statfs) and includes shared APFS volumes, local snapshots, purgeable space, and metadata. APFS clones share disk blocks, so individual file sizes will not sum to total used capacity.",
	DiskNotePermissions:    "Permissions: scans run with current user permissions (no root or Full Disk Access required). Inaccessible directories are skipped and reported in the scan summary.",
	Network:                "Network",
	NetDown:                "Down",
	NetUp:                  "Up",
	NetPeak:                "Peak",
	Quit:                   "Quit",
}

func messagesForLocale(loc Locale) panelMessages {
	if loc.IsZh() {
		return messagesZhCN
	}
	return messagesEn
}
