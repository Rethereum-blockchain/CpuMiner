package main

import (
	"ethashcpu/ethash"
	"os"
	sys "github.com/panglove/BaseServer/util/os"
)
func main(){
	// 获取命令行参数
	ethash.Start(os.Args[1])
	sys.WaitQuit()
}
