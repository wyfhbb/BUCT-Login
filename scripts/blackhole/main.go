// blackhole 是防重叠锁测试用的辅助程序：它接受 TCP 连接但永远不回数据，
// 于是探测那一端会一直挂到超时为止。测试需要一个「跑得足够久」的进程来占住锁，
// 而 connection refused 或不可路由的地址都是瞬间返回的，制造不出重叠。
package main

import (
	"flag"
	"log"
	"net"
)

func main() {
	addr := flag.String("addr", "127.0.0.1:18080", "监听地址")
	flag.Parse()

	listener, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("监听 %s 失败: %v", *addr, err)
	}

	// 已接受的连接必须留着引用，否则被回收关闭后对端就不会挂起了
	var held []net.Conn
	for {
		conn, err := listener.Accept()
		if err != nil {
			return
		}
		held = append(held, conn)
	}
}
