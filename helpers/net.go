package helpers

import (
	"net"
	"strings"
)

func CompareAddrs(a1, a2 net.Addr) bool {
	if !(a1 != nil && a2 != nil) {
		return false
	}
	return a1.String() == a2.String()
}

func CompareIps(a1, a2 net.Addr) bool {
	if !(a1 != nil && a2 != nil) {
		return false
	}
	return strings.Split(a1.String(), ":")[0] == strings.Split(a2.String(), ":")[0]

}
