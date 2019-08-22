package facts

import (
	"agentgo/logger"
	"context"
	"fmt"
	"io/ioutil"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/errdefs"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/host"
	"github.com/shirou/gopsutil/load"
	"github.com/shirou/gopsutil/mem"
	"github.com/shirou/gopsutil/process"
)

var (
	//nolint:gochecknoglobals
	dockerCGroupRE = regexp.MustCompile(
		`(?m:^\d+:[^:]+:(/kubepods/.*pod[0-9a-fA-F-]+/|.*/docker[-/])([0-9a-fA-F]+)(\.scope)?$)`,
	)
)

// ProcessProvider provider information about processes.
type ProcessProvider struct {
	l sync.Mutex

	dp                    dockerProcess
	psutil                processLister
	containerIDFromCGroup func(int) string

	processes           map[int]Process
	topinfo             TopInfo
	lastCPUtimes        cpu.TimesStat
	lastProcessesUpdate time.Time
}

// Process describe one Process
type Process struct {
	PID             int       `json:"pid"`
	PPID            int       `json:"ppid"`
	CreateTime      time.Time `json:"-"`
	CreateTimestamp int64     `json:"create_time"`
	CmdLineList     []string  `json:"-"`
	CmdLine         string    `json:"cmdline"`
	Name            string    `json:"name"`
	MemoryRSS       uint64    `json:"memory_rss"`
	CPUPercent      float64   `json:"cpu_percent"`
	CPUTime         float64   `json:"cpu_times"`
	Status          string    `json:"status"`
	Username        string    `json:"username"`
	Executable      string    `json:"exe"`
	ContainerID     string    `json:"-"`
	ContainerName   string    `json:"instance"`
}

// TopInfo contains all information to show a top-like view
type TopInfo struct {
	Time      int64       `json:"time"`
	Uptime    int         `json:"uptime"`
	Loads     []float64   `json:"loads"`
	Users     int         `json:"users"`
	Processes []Process   `json:"processes"`
	CPU       CPUUsage    `json:"cpu"`
	Memory    MemoryUsage `json:"memory"`
	Swap      SwapUsage   `json:"swap"`
}

// CPUUsage contains usage of CPU
type CPUUsage struct {
	User      float64 `json:"user"`
	Nice      float64 `json:"nice"`
	System    float64 `json:"system"`
	Idle      float64 `json:"idle"`
	IOWait    float64 `json:"iowait"`
	Guest     float64 `json:"guest"`
	GuestNice float64 `json:"guest_nice"`
	IRQ       float64 `json:"irq"`
	SoftIRQ   float64 `json:"softirq"`
	Steal     float64 `json:"steal"`
}

// MemoryUsage contains usage of Memory
type MemoryUsage struct {
	Total   float64 `json:"total"`
	Used    float64 `json:"used"`
	Free    float64 `json:"free"`
	Buffers float64 `json:"buffers"`
	Cached  float64 `json:"cached"`
}

// SwapUsage contains usage of Swap
type SwapUsage struct {
	Total float64 `json:"total"`
	Used  float64 `json:"used"`
	Free  float64 `json:"free"`
}

// NewProcess creates a new Process provider
//
// Docker provider should be given to allow processes to be associated with a Docker container
func NewProcess(dockerProvider *DockerProvider) *ProcessProvider {
	return &ProcessProvider{
		psutil: psutilLister{},
		dp: &dockerProcessImpl{
			dockerProvider: dockerProvider,
		},
		containerIDFromCGroup: containerIDFromCGroup,
	}
}

// Processes returns the list of processes present on this system.
//
// It may use a cached value as old as maxAge.
func (pp *ProcessProvider) Processes(ctx context.Context, maxAge time.Duration) (processes map[int]Process, err error) {
	processes, _, err = pp.ProcessesWithTime(ctx, maxAge)
	return
}

// TopInfo returns a topinfo object
//
// It may use a cached value as old as maxAge.
func (pp *ProcessProvider) TopInfo(ctx context.Context, maxAge time.Duration) (topinfo TopInfo, err error) {
	pp.l.Lock()
	defer pp.l.Unlock()

	if time.Since(pp.lastProcessesUpdate) > maxAge {
		err = pp.updateProcesses(ctx)
		if err != nil {
			return
		}
	}
	return pp.topinfo, nil
}

// ProcessesWithTime returns the list of processes present on this system and the date of last update
//
// It the same as Processes but also return the date of last update
func (pp *ProcessProvider) ProcessesWithTime(ctx context.Context, maxAge time.Duration) (processes map[int]Process, updateAt time.Time, err error) {
	pp.l.Lock()
	defer pp.l.Unlock()

	if time.Since(pp.lastProcessesUpdate) > maxAge {
		err = pp.updateProcesses(ctx)
		if err != nil {
			return
		}
	}
	return pp.processes, pp.lastProcessesUpdate, nil
}

func containerIDFromCGroup(pid int) string {
	path := filepath.Join("/proc", fmt.Sprintf("%d", pid), "cgroup")
	data, err := ioutil.ReadFile(path)
	if err != nil {
		return ""
	}
	return containerIDFromCGroupData(string(data))
}

func containerIDFromCGroupData(data string) string {
	containerID := ""

	for _, submatches := range dockerCGroupRE.FindAllStringSubmatch(data, -1) {
		if containerID == "" {
			containerID = submatches[2]
		} else if containerID != submatches[2] {
			// different value for the same PID. Abort detection of container ID from cgroup
			return ""
		}
	}
	return containerID
}

func decodeDocker(top container.ContainerTopOKBody, containerID string, containerName string) []Process {
	userIndex := -1
	pidIndex := -1
	pcpuIndex := -1
	rssIndex := -1
	timeIndex := -1
	cmdlineIndex := -1
	statIndex := -1
	ppidIndex := -1
	for i, v := range top.Titles {
		switch v {
		case "PID":
			pidIndex = i
		case "CMD", "COMMAND":
			cmdlineIndex = i
		case "UID", "USER":
			userIndex = i
		case "%CPU":
			pcpuIndex = i
		case "RSS":
			rssIndex = i
		case "TIME":
			timeIndex = i
		case "STAT":
			statIndex = i
		case "PPID":
			ppidIndex = i
		}
	}

	if pidIndex == -1 || cmdlineIndex == -1 {
		return nil
	}

	processes := make([]Process, 0)
	for _, row := range top.Processes {
		pid, err := strconv.Atoi(row[pidIndex])
		if err != nil {
			continue
		}
		cmdLineList := strings.Split(row[cmdlineIndex], " ")
		process := Process{
			PID:           pid,
			CmdLine:       row[cmdlineIndex],
			CmdLineList:   cmdLineList,
			Name:          filepath.Base(cmdLineList[0]),
			ContainerID:   containerID,
			ContainerName: containerName,
		}
		if userIndex != -1 {
			process.Username = row[userIndex]
		}
		if pcpuIndex != -1 {
			v, err := strconv.ParseFloat(row[pcpuIndex], 64)
			if err == nil {
				process.CPUPercent = v
			}
		}
		if rssIndex != -1 {
			v, err := strconv.ParseInt(row[rssIndex], 10, 0)
			if err == nil {
				process.MemoryRSS = uint64(v)
			}
		}
		if timeIndex != -1 {
			v, err := psTime2Second(row[timeIndex])
			if err == nil {
				process.CPUTime = float64(v)
			}
		}
		if statIndex != -1 {
			process.Status = psStat2Status(row[statIndex])
		}
		if ppidIndex != -1 {
			v, err := strconv.ParseInt(row[ppidIndex], 10, 0)
			if err == nil {
				process.PPID = int(v)
			}
		}
		processes = append(processes, process)
	}
	return processes
}

func psTime2Second(psTime string) (int, error) {
	if strings.Count(psTime, ":") == 1 {
		// format is MM:SS
		l := strings.Split(psTime, ":")
		minute, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		second, err := strconv.ParseInt(l[1], 10, 0)
		if err != nil {
			return 0, err
		}
		return int(minute)*60 + int(second), nil
	}
	if strings.Count(psTime, ":") == 2 && strings.Contains(psTime, "-") {
		// format is DD-HH:MM:SS
		l := strings.Split(psTime, "-")
		day, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		l = strings.Split(l[1], ":")
		hour, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		minute, err := strconv.ParseInt(l[1], 10, 0)
		if err != nil {
			return 0, err
		}
		second, err := strconv.ParseInt(l[2], 10, 0)
		if err != nil {
			return 0, err
		}
		result := int(day)*86400 + int(hour)*3600 + int(minute)*60 + int(second)
		return result, nil
	}
	if strings.Count(psTime, ":") == 2 {
		// format is HH:MM:SS
		l := strings.Split(psTime, ":")
		hour, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		minute, err := strconv.ParseInt(l[1], 10, 0)
		if err != nil {
			return 0, err
		}
		second, err := strconv.ParseInt(l[2], 10, 0)
		if err != nil {
			return 0, err
		}
		result := int(hour)*3600 + int(minute)*60 + int(second)
		return result, nil
	}
	if strings.Contains(psTime, "h") {
		// format is HHhMM
		l := strings.Split(psTime, "h")
		hour, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		minute, err := strconv.ParseInt(l[1], 10, 0)
		if err != nil {
			return 0, err
		}
		return int(hour)*3600 + int(minute)*60, nil
	}
	if strings.Contains(psTime, "d") {
		// format is DDdHH
		l := strings.Split(psTime, "d")
		day, err := strconv.ParseInt(l[0], 10, 0)
		if err != nil {
			return 0, err
		}
		hour, err := strconv.ParseInt(l[1], 10, 0)
		if err != nil {
			return 0, err
		}
		return int(day)*86400 + int(hour)*3600, nil
	}
	return 0, fmt.Errorf("unknown pstime format %#v", psTime)
}

func psStat2Status(psStat string) string {
	if psStat == "" {
		return "?"
	}
	switch psStat[0] {
	case 'D':
		return "disk-sleep"
	case 'R':
		return "running"
	case 'S':
		return "sleeping"
	case 'T':
		return "stopped"
	case 't':
		return "tracing-stop"
	case 'X':
		return "dead"
	case 'Z':
		return "zombie"
	case 'I':
		return "idle"
	default:
		return "?"
	}
}

func (pp *ProcessProvider) updateProcesses(ctx context.Context) error {
	t0 := time.Now()
	// Process creation time is accurate up to 1/SC_CLK_TCK seconds,
	// usually 1/100th of seconds.
	// Process must be started at least 1/100th before t0.
	// Keep some additional margin by doubling this value.
	onlyStartedBefore := t0.Add(-20 * time.Millisecond)

	newProcessesMap := make(map[int]Process)
	if pp.dp != nil {
		dockerProcesses, err := pp.dp.processes(ctx, 0)
		if err != nil {
			return err
		}
		for _, p := range dockerProcesses {
			newProcessesMap[p.PID] = p
		}
	}
	psProcesses, err := pp.psutil.processes(ctx, 0)
	if err != nil {
		return err
	}
	for _, p := range psProcesses {
		if p.CreateTime.After(onlyStartedBefore) {
			continue
		}
		if pOld, ok := newProcessesMap[p.PID]; ok {
			p.ContainerID = pOld.ContainerID
			p.ContainerName = pOld.ContainerName
			pOld.update(p)
			newProcessesMap[p.PID] = pOld
		} else {
			newProcessesMap[p.PID] = p
		}
	}
	if pp.dp != nil {
		if id2name, err := pp.dp.containerID2Name(ctx, 10*time.Second); err == nil {
			for pid, p := range newProcessesMap {
				if p.ContainerID == "" && pp.containerIDFromCGroup != nil {
					// Using cgroup, make sure process is not running in a container
					candidateID := pp.containerIDFromCGroup(pid)
					if n, ok := id2name[candidateID]; ok {
						logger.V(1).Printf("Based on cgroup, process %d (%s) belong to container %s", p.PID, p.Name, n)
						p.ContainerID = candidateID
						p.ContainerName = n
						newProcessesMap[pid] = p
					} else if candidateID != "" && time.Since(p.CreateTime) < 3*time.Second {
						logger.V(1).Printf("Skipping process %d (%s) created recently and seems to belong to a container", p.PID, p.Name)
						delete(newProcessesMap, pid)
					}
				}
			}
		}
	}

	// Update CPU percent
	for pid, p := range newProcessesMap {
		if oldP, ok := pp.processes[pid]; ok && oldP.CreateTime.Equal(p.CreateTime) {
			deltaT := time.Since(pp.lastProcessesUpdate)
			deltaCPU := p.CPUTime - oldP.CPUTime
			if deltaCPU > 0 && deltaT > 0 {
				p.CPUPercent = deltaCPU / deltaT.Seconds() * 100
				newProcessesMap[pid] = p
			}
		} else if !p.CreateTime.IsZero() {
			deltaT := time.Since(p.CreateTime)
			deltaCPU := p.CPUTime
			if deltaCPU > 0 && deltaT > 0 {
				p.CPUPercent = deltaCPU / deltaT.Seconds() * 100
				newProcessesMap[pid] = p
			}
		}
	}

	topinfo, err := pp.baseTopinfo()
	if err != nil {
		return err
	}
	topinfo.Time = time.Now().Unix()
	topinfo.Processes = make([]Process, 0, len(newProcessesMap))
	for _, p := range newProcessesMap {
		topinfo.Processes = append(topinfo.Processes, p)
	}

	pp.topinfo = topinfo
	pp.processes = newProcessesMap
	pp.lastProcessesUpdate = time.Now()
	logger.V(2).Printf("Completed processes update in %v", time.Since(t0))
	return nil
}

func (pp *ProcessProvider) baseTopinfo() (result TopInfo, err error) {
	uptime, err := host.Uptime()
	if err != nil {
		return result, err
	}
	result.Uptime = int(uptime)

	loads, err := load.Avg()
	if err != nil {
		return result, err
	}
	result.Loads = []float64{loads.Load1, loads.Load5, loads.Load15}

	users, err := host.Users()
	if err != nil {
		return result, err
	}
	result.Users = len(users)

	memUsage, err := mem.VirtualMemory()
	if err != nil {
		return result, err
	}
	result.Memory.Total = float64(memUsage.Total) / 1024.
	result.Memory.Used = float64(memUsage.Used) / 1024.
	result.Memory.Free = float64(memUsage.Free) / 1024.
	result.Memory.Buffers = float64(memUsage.Buffers) / 1024.
	result.Memory.Cached = float64(memUsage.Cached) / 1024.

	swapUsage, err := mem.SwapMemory()
	if err != nil {
		return result, err
	}
	result.Swap.Total = float64(swapUsage.Total) / 1024.
	result.Swap.Used = float64(swapUsage.Used) / 1024.
	result.Swap.Free = float64(swapUsage.Free) / 1024.

	cpusTimes, err := cpu.Times(false)
	if err != nil {
		return result, err
	}
	cpuTimes := cpusTimes[0]

	total1 := pp.lastCPUtimes.Total()
	total2 := cpuTimes.Total()
	delta := total2 - total1
	if delta >= 0 {
		between0and100 := func(input float64) float64 {
			if input < 0 {
				return 0
			}
			if input > 100 {
				return 100
			}
			return input
		}
		result.CPU.User = between0and100((cpuTimes.User - pp.lastCPUtimes.User) / delta * 100)
		result.CPU.Nice = between0and100((cpuTimes.Nice - pp.lastCPUtimes.Nice) / delta * 100)
		result.CPU.System = between0and100((cpuTimes.System - pp.lastCPUtimes.System) / delta * 100)
		result.CPU.Idle = between0and100((cpuTimes.Idle - pp.lastCPUtimes.Idle) / delta * 100)
		result.CPU.IOWait = between0and100((cpuTimes.Iowait - pp.lastCPUtimes.Iowait) / delta * 100)
		result.CPU.Guest = between0and100((cpuTimes.Guest - pp.lastCPUtimes.Guest) / delta * 100)
		result.CPU.GuestNice = between0and100((cpuTimes.GuestNice - pp.lastCPUtimes.GuestNice) / delta * 100)
		result.CPU.IRQ = between0and100((cpuTimes.Irq - pp.lastCPUtimes.Irq) / delta * 100)
		result.CPU.SoftIRQ = between0and100((cpuTimes.Softirq - pp.lastCPUtimes.Softirq) / delta * 100)
		result.CPU.Steal = between0and100((cpuTimes.Steal - pp.lastCPUtimes.Steal) / delta * 100)
	}

	pp.lastCPUtimes = cpuTimes
	return result, nil
}

func (p *Process) update(other Process) {
	if other.PPID != 0 {
		p.PPID = other.PPID
	}
	if !other.CreateTime.IsZero() {
		p.CreateTime = other.CreateTime
		p.CreateTimestamp = other.CreateTimestamp
	}
	if len(other.CmdLineList) > 0 {
		p.CmdLineList = other.CmdLineList
		p.CmdLine = other.CmdLine
	}
	if other.Name != "" {
		p.Name = other.Name
	}
	if other.MemoryRSS != 0 {
		p.MemoryRSS = other.MemoryRSS
	}
	if other.CPUPercent != 0 {
		p.CPUPercent = other.CPUPercent
	}
	if other.CPUTime != 0 {
		p.CPUTime = other.CPUTime
	}
	if other.Status != "" {
		p.Status = other.Status
	}
	if other.Username != "" {
		p.Username = other.Username
	}
	if other.Executable != "" {
		p.Executable = other.Executable
	}
	if other.ContainerID != "" {
		p.ContainerID = other.ContainerID
	}
	if other.ContainerName != "" {
		p.ContainerName = other.ContainerName
	}
}

type processLister interface {
	processes(ctx context.Context, maxAge time.Duration) (processes []Process, err error)
}

type dockerProcess interface {
	processLister
	containerID2Name(ctx context.Context, maxAge time.Duration) (containerID2Name map[string]string, err error)
}

type psutilLister struct{}

func (z psutilLister) processes(ctx context.Context, maxAge time.Duration) (processes []Process, err error) {
	psutilProcesses, err := process.Processes()
	if err != nil {
		return nil, err
	}
	processes = make([]Process, 0)
	for _, p := range psutilProcesses {
		if p.Pid == 0 {
			// PID 0 on Windows use it for "System Idle Process".
			// PID 0 is not used on Linux
			// Other system are currently not supported.
			continue
		}
		ts, err := p.CreateTimeWithContext(ctx)
		if err != nil {
			continue
		}
		createTime := time.Unix(ts/1000, (ts%1000)*1000000)
		ppid, err := p.PpidWithContext(ctx)
		if err != nil {
			continue
		}
		userName, err := p.UsernameWithContext(ctx)
		if err != nil {
			if runtime.GOOS != "windows" {
				uids, err := p.UidsWithContext(ctx)
				if err != nil {
					userName = fmt.Sprintf("%d", uids[0])
				}
			}
		}
		name, err := p.NameWithContext(ctx)
		if err != nil {
			continue
		}
		cmdLine, err := p.CmdlineSliceWithContext(ctx)
		if err != nil || len(cmdLine) == 0 || len(cmdLine[0]) == 0 {
			cmdLine = []string{name}
		} else {
			// Remove empty argument. This is usually generated by
			// p rocesses which alter their name and result in
			// npm '' '' '' '' '' '' '' '' '' '' '' ''
			cmdLine2 := make([]string, 0)
			for _, v := range cmdLine {
				if len(v) > 0 {
					cmdLine2 = append(cmdLine2, v)
				}
			}
			cmdLine = cmdLine2
		}
		executable, err := p.ExeWithContext(ctx)
		if err != nil {
			executable = ""
		}

		memoryInfo, err := p.MemoryInfoWithContext(ctx)
		if err != nil {
			continue
		}
		cpuTimes, err := p.TimesWithContext(ctx)
		if err != nil {
			continue
		}
		status, err := p.StatusWithContext(ctx)
		if err != nil {
			continue
		}

		p := Process{
			PID:             int(p.Pid),
			PPID:            int(ppid),
			CreateTime:      createTime,
			CreateTimestamp: createTime.Unix(),
			CmdLineList:     cmdLine,
			CmdLine:         strings.Join(cmdLine, " "),
			Name:            name,
			MemoryRSS:       memoryInfo.RSS / 1024,
			CPUTime:         cpuTimes.Total(),
			Status:          psStat2Status(status),
			Username:        userName,
			Executable:      executable,
		}
		processes = append(processes, p)
	}
	return processes, nil
}

type dockerProcessImpl struct {
	dockerProvider *DockerProvider
}

func (d *dockerProcessImpl) containerID2Name(ctx context.Context, maxAge time.Duration) (containerID2Name map[string]string, err error) {
	if d.dockerProvider == nil {
		return
	}
	containers, err := d.dockerProvider.Containers(ctx, maxAge, true)
	if err != nil {
		return
	}
	containerID2Name = make(map[string]string)
	for _, c := range containers {
		containerID2Name[c.ID()] = c.Name()
	}
	return
}

func (d *dockerProcessImpl) processes(ctx context.Context, maxAge time.Duration) (processes []Process, err error) {
	if d.dockerProvider == nil {
		return
	}
	processesMap := make(map[int]Process)
	containers, err := d.dockerProvider.Containers(ctx, maxAge, true)
	if err != nil {
		if !d.dockerProvider.HasConnection(ctx) {
			err = nil
		}
		return
	}
	for _, c := range containers {
		var top, topWaux container.ContainerTopOKBody
		if !c.IsRunning() {
			continue
		}
		top, topWaux, err = d.dockerProvider.top(ctx, c.ID())
		switch {
		case err != nil && errdefs.IsNotFound(err):
			continue
		case err != nil && strings.Contains(fmt.Sprintf("%v", err), "is not running"):
			continue
		case err != nil:
			logger.Printf("%#v", err)
			return
		}
		processes1 := decodeDocker(top, c.ID(), c.Name())
		processes2 := decodeDocker(topWaux, c.ID(), c.Name())
		for _, p := range processes1 {
			processesMap[p.PID] = p
		}
		for _, p := range processes2 {
			if pOld, ok := processesMap[p.PID]; ok {
				pOld.update(p)
				processesMap[p.PID] = pOld
			} else {
				processesMap[p.PID] = p
			}
		}
	}
	processes = make([]Process, 0, len(processesMap))
	for _, p := range processesMap {
		processes = append(processes, p)
	}
	return processes, nil
}
