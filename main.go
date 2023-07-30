package main

import (
	"ethashcpu/ethash"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	if len(os.Args) < 3 {
		println("Usage: cpuminer [rpcUrl] [threads]")
		return
	}

	thirdArg := ""

	if len(os.Args) == 4 {
		thirdArg = os.Args[3]
	}

	ethash.Start(os.Args[1], os.Args[2], thirdArg)

	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc
}
