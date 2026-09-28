//go:build ignore

// redis_exec_gate is a single-connection, test-only Redis wire gate. It lets
// AUTH/SELECT and queued revoke commands reach the real Redis server, but
// holds EXEC so the CLI is interrupted after its MariaDB revocation stage.
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"net"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {
	if len(os.Args) != 4 {
		os.Exit(2)
	}
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		os.Exit(1)
	}
	defer listener.Close()
	port := listener.Addr().(*net.TCPAddr).Port
	if err := os.WriteFile(os.Args[2], []byte(strconv.Itoa(port)+"\n"), 0600); err != nil {
		os.Exit(1)
	}
	client, err := listener.Accept()
	if err != nil {
		os.Exit(1)
	}
	defer client.Close()
	upstream, err := net.DialTimeout("tcp", os.Args[1], 3*time.Second)
	if err != nil {
		os.Exit(1)
	}
	defer upstream.Close()
	go io.Copy(client, upstream)
	reader := bufio.NewReader(client)
	for {
		packet, command, err := readCommand(reader)
		if err != nil {
			return
		}
		if command == "EXEC" {
			if err := os.WriteFile(os.Args[3], []byte("held\n"), 0600); err != nil {
				os.Exit(1)
			}
			// The caller has a shorter bounded timeout and terminates this helper.
			// Never forward EXEC: the Redis ACL revocation must remain incomplete.
			time.Sleep(20 * time.Second)
			return
		}
		if _, err := io.Copy(upstream, bytes.NewReader(packet)); err != nil {
			return
		}
	}
}

func readCommand(reader *bufio.Reader) ([]byte, string, error) {
	var packet bytes.Buffer
	header, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, "", err
	}
	packet.Write(header)
	if len(header) < 4 || header[0] != '*' || !bytes.HasSuffix(header, []byte("\r\n")) {
		return nil, "", fmt.Errorf("invalid RESP array")
	}
	count, err := strconv.Atoi(string(header[1 : len(header)-2]))
	if err != nil || count < 1 || count > 32 {
		return nil, "", fmt.Errorf("invalid RESP array length")
	}
	var command string
	for index := 0; index < count; index++ {
		bulkHeader, err := reader.ReadBytes('\n')
		if err != nil {
			return nil, "", err
		}
		packet.Write(bulkHeader)
		if len(bulkHeader) < 4 || bulkHeader[0] != '$' || !bytes.HasSuffix(bulkHeader, []byte("\r\n")) {
			return nil, "", fmt.Errorf("invalid RESP bulk string")
		}
		length, err := strconv.Atoi(string(bulkHeader[1 : len(bulkHeader)-2]))
		if err != nil || length < 0 || length > 1<<20 {
			return nil, "", fmt.Errorf("invalid RESP bulk length")
		}
		bulk := make([]byte, length+2)
		if _, err := io.ReadFull(reader, bulk); err != nil {
			return nil, "", err
		}
		if !bytes.HasSuffix(bulk, []byte("\r\n")) {
			return nil, "", fmt.Errorf("invalid RESP bulk terminator")
		}
		packet.Write(bulk)
		if index == 0 {
			command = strings.ToUpper(string(bulk[:length]))
		}
	}
	return packet.Bytes(), command, nil
}
