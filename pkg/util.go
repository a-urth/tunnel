package pkg

import (
	"fmt"
	"github.com/google/uuid"
	"hash/fnv"
	"math"
	"net"
	"os"
)

func GetFreePort() (int, error) {
	addr, err := net.ResolveTCPAddr("tcp", "localhost:0")
	if err != nil {
		return 0, fmt.Errorf("resolve local address: %w", err)
	}

	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return 0, fmt.Errorf("listen tcp: %w", err)
	}

	defer func() { _ = l.Close() }()

	return l.Addr().(*net.TCPAddr).Port, nil //nolint:forcetypeassert
}

func getPortFromStr(s string) (int, error) {
	hash := fnv.New32()
	if _, err := hash.Write([]byte(s)); err != nil {
		return -1, fmt.Errorf("hash input strings: %w", err)
	}

	return int(hash.Sum32() % math.MaxUint16), nil
}

func getHostID(hostID string) (string, error) {
	b, err := os.ReadFile(hostID)
	if err == nil {
		return string(b), nil
	}

	if _, err := uuid.Parse(hostID); err == nil {
		return hostID, nil
	}

	return "", fmt.Errorf("host id should be either uuid or path to file with it")
}
