package facts

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"os"
	"regexp"
	"strconv"
	"strings"
	"syscall"

	psutilNet "github.com/shirou/gopsutil/net"
)

// NetstatProvider provide netstat information from both a file (output of netstat command) and using gopsutil
//
// The file is useful since gopsutil will be run with current privilege which are unlikely to be root.
// The file should be the output of netstat run as root.
type NetstatProvider struct {
	filePath string
}

// Netstat return a mapping from PID to listening addresses
//
// Supported addresses network is currently "tcp", "udp" or "unix".
func (np NetstatProvider) Netstat(ctx context.Context) (netstat map[int][]net.Addr, err error) {
	netstatData, err := ioutil.ReadFile(np.filePath)
	if err != nil && !os.IsNotExist(err) {
		return
	}
	netstat = decodeNetstatFile(string(netstatData))
	dynamicNetstat, err := psutilNet.Connections("inet")
	if err == nil {
		for _, c := range dynamicNetstat {
			if c.Pid == 0 {
				continue
			}
			if c.Status != "LISTEN" {
				continue
			}
			address := c.Laddr.IP
			protocol := ""
			switch {
			case c.Type == syscall.SOCK_STREAM:
				protocol = "tcp"
			case c.Type == syscall.SOCK_DGRAM:
				protocol = "udp"
			default:
				continue
			}
			if c.Family == syscall.AF_INET6 {
				protocol += "6"
			}

			netstat[int(c.Pid)] = addAddress(netstat[int(c.Pid)], listenAddress{
				network: protocol,
				address: address,
				port:    int(c.Laddr.Port),
			})
		}
	}
	return netstat, nil
}

//nolint:gochecknoglobals
var (
	netstatRE = regexp.MustCompile(
		`(?P<protocol>udp6?|tcp6?)\s+\d+\s+\d+\s+(?P<address>[0-9a-f.:]+):(?P<port>\d+)\s+[0-9a-f.:*]+\s+(LISTEN)?\s+(?P<pid>\d+)/(?P<program>.*)$`,
	)
	netstatUnixRE = regexp.MustCompile(
		`^(?P<protocol>unix)\s+\d+\s+\[\s+(ACC |W |N )+\s*\]\s+(DGRAM|STREAM)\s+LISTENING\s+(\d+\s+)?(?P<pid>\d+)/(?P<program>.*)\s+(?P<address>.+)$`,
	)
)

type listenAddress struct {
	network string
	address string
	port    int
}

func (l listenAddress) Network() string {
	return l.network
}
func (l listenAddress) String() string {
	if l.network == "unix" {
		return l.address
	}
	return fmt.Sprintf("%s:%d", l.address, l.port)
}

func decodeNetstatFile(data string) map[int][]net.Addr {
	result := make(map[int][]net.Addr)
	lines := strings.Split(data, "\n")
	for _, line := range lines {
		var protocol, address string
		var pid, port int64
		var err error
		r := netstatRE.FindStringSubmatch(line)
		if r != nil {
			protocol = r[1]
			address = r[2]
			port, err = strconv.ParseInt(r[3], 10, 0)
			if err != nil {
				continue
			}
			pid, err = strconv.ParseInt(r[5], 10, 0)
			if err != nil {
				continue
			}
		} else {
			r = netstatUnixRE.FindStringSubmatch(line)
			if r == nil {
				continue
			}
			protocol = r[1]
			address = r[7]
			pid, err = strconv.ParseInt(r[5], 10, 0)
			if err != nil {
				continue
			}
			port = 0
		}

		addresses := result[int(pid)]
		if addresses == nil {
			addresses = make([]net.Addr, 0)
		}
		result[int(pid)] = addAddress(addresses, listenAddress{
			network: protocol,
			address: address,
			port:    int(port),
		})
	}
	return result
}

func addAddress(addresses []net.Addr, newAddr listenAddress) []net.Addr {
	duplicate := false
	if newAddr.network != "unix" {
		if newAddr.network == "tcp6" || newAddr.network == "udp6" {
			if newAddr.address == "::" {
				newAddr.address = "0.0.0.0"
			}
			if newAddr.address == "::1" {
				newAddr.address = "127.0.0.1"
			}
			if strings.Contains(newAddr.address, ":") {
				// It's still an IPv6 address, we don't know how to convert it to IPv4
				return addresses
			}
			newAddr.network = newAddr.network[:3]
		}

		for i, v := range addresses {
			if v.Network() != newAddr.Network() {
				continue
			}
			_, otherPortStr, err := net.SplitHostPort(v.String())
			if err != nil {
				log.Printf("DBG: unable to split host/port for %#v: %v", v.String(), err)
				return addresses
			}
			otherPort, err := strconv.ParseInt(otherPortStr, 10, 0)
			if err != nil {
				log.Printf("DBG: unable to parse port %#v: %v", otherPortStr, err)
				return addresses
			}
			if int(otherPort) == newAddr.port {
				duplicate = true
				// We prefere 127.* address
				if strings.HasPrefix(newAddr.address, "127.") {
					addresses[i] = newAddr
				}
				break
			}
		}
	}
	if !duplicate {
		addresses = append(addresses, newAddr)
	}
	return addresses
}