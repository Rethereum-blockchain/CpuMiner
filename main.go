package main

import (
	"ethashcpu/ethash"
	sys "github.com/panglove/BaseServer/util/os"
	"os"
)

func main() {
	//if len(os.Args) < 3 {
	//	println("Usage: cpuminer [rpcUrl] [threads]")
	//	return
	//}
	// 获取命令行参数
	ethash.Start(os.Args[1])
	sys.WaitQuit()
}
