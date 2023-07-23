package ethash

import (
	"bytes"
	"encoding/json"
	"ethashcpu/util"
	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core/types"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"runtime"
	"time"
)

var rpcUrl string
var cpuHash *Ethash

type RpcReback struct {
	Jsonrpc string   `json:"jsonrpc"`
	Result  []string `json:"result"`
	Id      int      `json:"id"`
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

func Start(url string) {
	rpcUrl = url
	log.Println("Starting cpu Ethash-B3 mining, Connected RPC url:", rpcUrl)
	getWork := make(chan Work)
	submitWork := make(chan *types.Block)

	go func() {
		StartMiner(getWork, submitWork)
	}()
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
	cpuHash = New(newConfig, nil, false)
	defer func(cpuHash *Ethash) {
		err := cpuHash.Close()
		if err != nil {
			log.Println("Close cpuHash error", err)
		}
	}(cpuHash)

	stop := make(chan int)
	currentBlock := new(Work)
	getWorkTimer := time.NewTicker(5 * time.Second)
	first := false

	go func() {
		for {
			select {
			case work := <-getWork:
				currentBlock = &work
				log.Println("Mining block", work.Header.Number, work.Header.Difficulty, work.Hash)

				go func() {
					if first {
						go func() {
							stop <- 1
						}()
					}
					first = true
					err := cpuHash.Seal(nil, types.NewBlockWithHeader(work.Header), submitWork, stop, common.HexToHash(work.Hash))
					if err != nil {
						log.Fatalf("failed to seal block: %v", err)
						return
					}
				}()
			case block := <-submitWork:
				currentBlock.Header.Nonce = types.EncodeNonce(block.Nonce())
				currentBlock.Header.MixDigest = block.MixDigest()
				nonce, _ := currentBlock.Header.Nonce.MarshalText()
				mix, _ := currentBlock.Header.MixDigest.MarshalText()
				log.Println(string(nonce), string(mix))
				SubmitWork(string(nonce), currentBlock.Hash, string(mix))

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

				//stop <- 1
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

func SubmitWork(nonce string, blockHash string, mixHash string) {
	getWorkInfo := RpcInfo{Method: "eth_submitWork", Params: []string{nonce, blockHash, mixHash}, Id: 1, Jsonrpc: "2.0"}
	log.Println("Submit work:", getWorkInfo.Params)
	getWorkInfoBuffs, _ := json.Marshal(getWorkInfo)

	req, err := http.NewRequest("POST", rpcUrl, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	log.Println("Submit reback", string(body))
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
	body, _ := ioutil.ReadAll(resp.Body)
	workReback := new(RpcReback)

	json.Unmarshal(body, workReback)

	newHeader := new(types.Header)
	newHeader.Number = util.HexToBig(workReback.Result[3])
	newHeader.Difficulty = util.TargetHexToDiff(workReback.Result[2])

	return newHeader, workReback.Result[0]
}
