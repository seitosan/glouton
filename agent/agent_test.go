package agent

import "testing"

func TestParseIPOutput(t *testing.T) {
	cases := []struct {
		description string
		in          string
		want        string
	}{
		{
			description: `Output from "ip route get 8.8.8.8;ip address show"`,
			in: `8.8.8.8 via 10.0.2.2 dev eth0  src 10.0.2.15 
    cache
1: lo: <LOOPBACK,UP,LOWER_UP> mtu 65536 qdisc noqueue state UNKNOWN group default 
    link/loopback 00:00:00:00:00:00 brd 00:00:00:00:00:00
    inet 127.0.0.1/8 scope host lo
       valid_lft forever preferred_lft forever
    inet6 ::1/128 scope host 
       valid_lft forever preferred_lft forever
2: eth0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc pfifo_fast state UP group default qlen 1000
    link/ether 08:00:27:72:ca:f1 brd ff:ff:ff:ff:ff:ff
    inet 10.0.2.15/24 brd 10.0.2.255 scope global eth0
       valid_lft forever preferred_lft forever
    inet6 fe80::a00:27ff:fe72:caf1/64 scope link 
       valid_lft forever preferred_lft forever
3: docker0: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue state UP group default 
    link/ether 02:42:8d:e2:74:26 brd ff:ff:ff:ff:ff:ff
    inet 172.17.0.1/16 brd 172.17.255.255 scope global docker0
       valid_lft forever preferred_lft forever
    inet6 fe80::42:8dff:fee2:7426/64 scope link 
       valid_lft forever preferred_lft forever
5: vethde299e2: <BROADCAST,MULTICAST,UP,LOWER_UP> mtu 1500 qdisc noqueue master docker0 state UP group default 
    link/ether fe:b8:9f:9e:36:aa brd ff:ff:ff:ff:ff:ff
    inet6 fe80::fcb8:9fff:fe9e:36aa/64 scope link 
       valid_lft forever preferred_lft forever
`,
			want: "08:00:27:72:ca:f1",
		},
		{
			description: "don't crash on invalid input",
			in:          `8.8.8.8 via 10.0.2.2 dev eth0  src`,
			want:        "",
		},
	}
	for _, c := range cases {
		got := parseIPOutput([]byte(c.in))
		if got != c.want {
			t.Errorf("parseIPOutput([case %s]) == %#v, want %#v", c.description, got, c.want)
		}
	}
}
