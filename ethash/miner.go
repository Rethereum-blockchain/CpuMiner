package ethash

import (
	"bytes"
	"encoding/json"
	"ethashcpu/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"io"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"strconv"
	"time"
)

var rpcUrl string
var cpuHash *Ethash
var globalThreads int

type RpcReback struct {
	Jsonrpc string   `json:"jsonrpc"`
	Result  []string `json:"result"`
	Id      int      `json:"id"`
}

type WorkResult struct {
	Jsonrpc string `json:"jsonrpc"`
	Result  bool   `json:"result"`
	Id      int    `json:"id"`
}

type RpcInfo struct {
	Jsonrpc string   `json:"jsonrpc"`
	Method  string   `json:""`
	Params  []string `json:"params"`
	Id      int      `json:"id"`
}

type Work struct {
	Header *types.Header
	Hash   string
}

func InitConfig(currConfig *Config) {
	home := os.Getenv("HOME")

	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}

	if runtime.GOOS == "darwin" {
		currConfig.DatasetDir = filepath.Join(home, "Library", "Ethash-B3")
	} else if runtime.GOOS == "windows" {
		localappdata := os.Getenv("LOCALAPPDATA")
		if localappdata != "" {
			currConfig.DatasetDir = filepath.Join(localappdata, "Ethash-B3")
		} else {
			currConfig.DatasetDir = filepath.Join(home, "AppData", "Local", "Ethash-B3")
		}
	} else {
		currConfig.DatasetDir = filepath.Join(home, ".ethash-B3")
	}
}

func Start(url string, threads string) {
	globalThreads, _ = strconv.Atoi(threads)
	rpcUrl = url
	log.Println("Starting CPU Ethash-B3 mining. Connected RPC URL:", rpcUrl)
	getWork := make(chan Work)
	submitWork := make(chan *types.Block)

	StartMiner(getWork, submitWork)
}

func StartMiner(getWork chan Work, submitWork chan *types.Block) {
	newConfig := Config{
		CacheDir:         "ethash",
		CachesInMem:      2,
		CachesOnDisk:     3,
		CachesLockMmap:   false,
		DatasetsInMem:    1,
		DatasetsOnDisk:   2,
		DatasetsLockMmap: false,
	}
	InitConfig(&newConfig)
	cpuHash = New(newConfig, nil, false, globalThreads)
	defer func(cpuHash *Ethash) {
		err := cpuHash.Close()
		if err != nil {
			log.Println("Close cpuHash error", err)
		}
	}(cpuHash)

	stop := make(chan int)
	currentBlock := new(Work)
	getWorkTimer := time.NewTicker(5 * time.Second)
	first := true

	go func() {
		for {
			select {
			case work := <-getWork:
				currentBlock = &work

				log.Println("Mining block:", work.Header.Number, "| Difficulty:", work.Header.Difficulty)

				if !first {
					stop <- 1
				}

				first = false
				err := cpuHash.Seal(nil, types.NewBlockWithHeader(work.Header), submitWork, stop, common.HexToHash(work.Hash))

				if err != nil {
					log.Fatalf("failed to seal block: %v", err)
					return
				}
			case block := <-submitWork:
				currentBlock.Header.Nonce = types.EncodeNonce(block.Nonce())
				currentBlock.Header.MixDigest = block.MixDigest()
				nonce, _ := currentBlock.Header.Nonce.MarshalText()
				mix, _ := currentBlock.Header.MixDigest.MarshalText()
				SubmitWork(string(nonce), currentBlock.Hash, string(mix), *currentBlock.Header)

				first = true

				header, hash := GetWorkHead()

				if header == nil {
					return
				}

				go func() {
					getWork <- Work{Header: header, Hash: hash}
				}()

			case <-getWorkTimer.C:
				header, hash := GetWorkHead()
				if header == nil {
					return
				}

				if hash == currentBlock.Hash {
					continue
				}

				go func() {
					getWork <- Work{Header: header, Hash: hash}
				}()
			}
		}
	}()

	header, hash := GetWorkHead()
	if header == nil {
		return
	}

	getWork <- Work{Header: header, Hash: hash}
}

func SubmitWork(nonce string, blockHash string, mixHash string, currentBlock types.Header) {
	log.Println("Submitting work for block", currentBlock.Number)
	getWorkInfo := RpcInfo{Method: "eth_submitWork", Params: []string{nonce, blockHash, mixHash}, Id: 1, Jsonrpc: "2.0"}
	getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)

	req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)

	workResult := new(WorkResult)

	json.Unmarshal(body, workResult)

	if workResult.Result {
		log.Println("Work successfully submitted.")
	} else {
		log.Println("Invalid work block.")
	}
}

func GetWorkHead() (*types.Header, string) {
	getWorkInfo := RpcInfo{Method: "eth_getWork", Params: []string{}, Id: 1, Jsonrpc: "2.0"}
	getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)

	req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, ""
	}

	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	workReback := new(RpcReback)

	json.Unmarshal(body, workReback)

	if len(workReback.Result) != 4 {
		log.Println("Mining not enabled on Geth.")
		os.Exit(1)
	}

	newHeader := new(types.Header)
	newHeader.Number = util.HexToBig(workReback.Result[3])
	newHeader.Difficulty = util.TargetHexToDiff(workReback.Result[2])

	return newHeader, workReback.Result[0]
}
