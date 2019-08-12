//nolint:gochecknoglobals
package zabbix

var versionRequest = []byte{0x5a, 0x42, 0x58, 0x44,
	0x01, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x76, 0x65, 0x72, 0x73, 0x69, 0x6f, 0x6e}
var versionAnswer = []byte{0x5a, 0x42, 0x58, 0x44,
	0x01, 0x05, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x34, 0x2e, 0x32, 0x2e, 0x34}
var pingRequest = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0x0a, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x61, 0x67, 0x65, 0x6e, 0x74, 0x2e, 0x70, 0x69, 0x6e, 0x67}
var pingAnswer = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x31}
var discRequest = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0x10, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x6e, 0x65, 0x74, 0x2e, 0x69, 0x66, 0x2e, 0x64,
	0x69, 0x73, 0x63, 0x6f, 0x76, 0x65, 0x72, 0x79} // net.if.discovery
var discAnswer = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0xb3, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x5b, 0x7b, 0x22, 0x7b, 0x23, 0x49, 0x46, 0x4e,
	0x41, 0x4d, 0x45, 0x7d, 0x22, 0x3a, 0x22, 0x77,
	0x6c, 0x70, 0x32, 0x73, 0x30, 0x22, 0x7d, 0x2c,
	0x7b, 0x22, 0x7b, 0x23, 0x49, 0x46, 0x4e, 0x41,
	0x4d, 0x45, 0x7d, 0x22, 0x3a, 0x22, 0x76, 0x65,
	0x74, 0x68, 0x30, 0x61, 0x38, 0x30, 0x33, 0x61,
	0x64, 0x22, 0x7d, 0x2c, 0x7b, 0x22, 0x7b, 0x23,
	0x49, 0x46, 0x4e, 0x41, 0x4d, 0x45, 0x7d, 0x22,
	0x3a, 0x22, 0x64, 0x6f, 0x63, 0x6b, 0x65, 0x72,
	0x30, 0x22, 0x7d, 0x2c, 0x7b, 0x22, 0x7b, 0x23,
	0x49, 0x46, 0x4e, 0x41, 0x4d, 0x45, 0x7d, 0x22,
	0x3a, 0x22, 0x76, 0x65, 0x74, 0x68, 0x33, 0x63,
	0x64, 0x33, 0x61, 0x34, 0x32, 0x22, 0x7d, 0x2c,
	0x7b, 0x22, 0x7b, 0x23, 0x49, 0x46, 0x4e, 0x41,
	0x4d, 0x45, 0x7d, 0x22, 0x3a, 0x22, 0x6c, 0x6f,
	0x22, 0x7d, 0x2c, 0x7b, 0x22, 0x7b, 0x23, 0x49,
	0x46, 0x4e, 0x41, 0x4d, 0x45, 0x7d, 0x22, 0x3a,
	0x22, 0x76, 0x65, 0x74, 0x68, 0x32, 0x30, 0x61,
	0x64, 0x34, 0x65, 0x61, 0x22, 0x7d, 0x2c, 0x7b,
	0x22, 0x7b, 0x23, 0x49, 0x46, 0x4e, 0x41, 0x4d,
	0x45, 0x7d, 0x22, 0x3a, 0x22, 0x76, 0x65, 0x74,
	0x68, 0x62, 0x63, 0x30, 0x39, 0x38, 0x31, 0x31,
	0x22, 0x7d, 0x5d}
var discString = `[{"{#IFNAME}":"wlp2s0"},{"{#IFNAME}":"veth0a803ad"},{"{#IFNAME}":"docker0"},{"{#IFNAME}":"veth3cd3a42"},{"{#IFNAME}":"lo"},{"{#IFNAME}":"veth20ad4ea"},{"{#IFNAME}":"vethbc09811"}]`
var inloRequest = []byte{0x5a, 0x42, 0x58, 0x44,
	0x01, 0x0d, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x6e, 0x65, 0x74, 0x2e, 0x69, 0x66, 0x2e,
	0x69, 0x6e, 0x5b, 0x6c, 0x6f, 0x5d} //net.if.in[lo]
var inloAnswer = []byte{0x5a, 0x42, 0x58, 0x44,
	0x01, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x37, 0x39, 0x37, 0x38, 0x32, 0x36}
var cpuUtilRequest = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0x1e, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x73, 0x79, 0x73, 0x74, 0x65, 0x6d, 0x2e, 0x63,
	0x70, 0x75, 0x2e, 0x75, 0x74, 0x69, 0x6c, 0x5b,
	0x61, 0x6c, 0x6c, 0x2c, 0x75, 0x73, 0x65, 0x72,
	0x2c, 0x61, 0x76, 0x67, 0x31, 0x5d}
var cpuUtilAnswer = []byte{0x5a, 0x42, 0x58, 0x44, 0x01,
	0x08, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
	0x33, 0x2e, 0x30, 0x30, 0x38, 0x31, 0x32, 0x33}
