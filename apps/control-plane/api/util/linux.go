package util

import (
	"errors"
	"fmt"
	"github.com/go-ping/ping"
	"github.com/shirou/gopsutil/cpu"
	"github.com/shirou/gopsutil/disk"
	"github.com/shirou/gopsutil/mem"
	"github.com/sirupsen/logrus"
	"net"
	"strconv"
	"time"
	"trojan-panel/model/constant"
)

func Ping(ip string) (int, error) {
	pingEr, err := ping.NewPinger(ip)
	if err != nil {
		return -1, errors.New(constant.SysError)
	}
	pingEr.Count = 1
	pingEr.Timeout = 2 * time.Second
	pingEr.SetPrivileged(true)
	err = pingEr.Run()
	if err != nil {
		return -1, errors.New(constant.SysError)
	}
	milliseconds := pingEr.Statistics().AvgRtt.Milliseconds()
	return int(milliseconds), nil
}

// IsPortAvailable 判断端口是否可用
func IsPortAvailable(port uint, network string) bool {
	if network == "tcp" {
		listener, err := net.ListenTCP(network, &net.TCPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: int(port),
		})
		defer listener.Close()
		if err != nil {
			logrus.Warnf("port %d is taken err: %s", port, err)
			return false
		}
	}
	if network == "udp" {
		listener, err := net.ListenUDP("udp", &net.UDPAddr{
			IP:   net.IPv4(0, 0, 0, 0),
			Port: int(port),
		})
		defer listener.Close()
		if err != nil {
			logrus.Warnf("port %d is taken err: %s", port, err)
			return false
		}
	}
	return true
}

// GetCpuPercent 获取CPU使用率
func GetCpuPercent() (float64, error) {
	var err error
	percent, err := cpu.Percent(time.Second, false)
	value, err := strconv.ParseFloat(fmt.Sprintf("%.1f", percent[0]), 64)
	return value, err
}

// GetMemPercent 获取内存使用率
func GetMemPercent() (float64, error) {
	var err error
	memInfo, err := mem.VirtualMemory()
	value, err := strconv.ParseFloat(fmt.Sprintf("%.1f", memInfo.UsedPercent), 64)
	return value, err
}

// GetDiskPercent 获取硬盘使用率
func GetDiskPercent() (float64, error) {
	var err error
	parts, err := disk.Partitions(true)
	diskInfo, err := disk.Usage(parts[0].Mountpoint)
	value, err := strconv.ParseFloat(fmt.Sprintf("%.1f", diskInfo.UsedPercent), 64)
	return value, err
}
