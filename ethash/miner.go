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
	Jsonrpc string `json:"jsonrpc"`
	Result []string `json:"result"`
	Id int `json:"id"`
}


type RpcInfo struct {
	Jsonrpc string `json:"jsonrpc"`
	Method string `json:""`
	Params []string `json:"params"`
	Id int `json:"id"`
}

func InitConfig(currConfig *Config){
	home := os.Getenv("HOME")
	if home == "" {
		if user, err := user.Current(); err == nil {
			home = user.HomeDir
		}
	}
	if runtime.GOOS == "darwin" {
		currConfig.DatasetDir = filepath.Join(home, "Library", "Ethash")
	} else if runtime.GOOS == "windows" {
		localappdata := os.Getenv("LOCALAPPDATA")
		if localappdata != "" {
			currConfig.DatasetDir = filepath.Join(localappdata, "Ethash")
		} else {
			currConfig.DatasetDir = filepath.Join(home, "AppData", "Local", "Ethash")
		}
	} else {
		currConfig.DatasetDir = filepath.Join(home, ".ethash")
	}
}
func Start(url string){
	rpcUrl = url

	log.Println("start cpu ethash,rpc url:",url)
	go func() {
		for{
			StartMiner()

			time.Sleep(time.Second)
		}
	}()
}

func StartMiner(){

	newConfig :=Config{
		CacheDir:         "ethash",
		CachesInMem:      2,
		CachesOnDisk:     3,
		CachesLockMmap:   false,
		DatasetsInMem:    1,
		DatasetsOnDisk:   2,
		DatasetsLockMmap: false,
	}
	InitConfig(&newConfig)
	cpuHash = New(newConfig,nil,false)
	header,hash:= GetWorkHead()

	if header==nil {
		return
	}

	defer cpuHash.Close()

	results := make(chan *types.Block)
	err := cpuHash.Seal(nil, types.NewBlockWithHeader(header), results, nil,common.HexToHash(hash))
	if err != nil {
		log.Fatalf("failed to seal block: %v", err)
	}
	select {
	case block := <-results:

		header.Nonce = types.EncodeNonce(block.Nonce())
		header.MixDigest = block.MixDigest()

		nonce,_ := header.Nonce.MarshalText()
		mix,_ := header.MixDigest.MarshalText()

		log.Println(string(nonce),string(mix))

		SubmitWork(string(nonce),hash,string(mix))
	}

}
func SubmitWork(nonce string,blockHash string,mixHash string) {

	url := rpcUrl

	getWorkInfo :=RpcInfo{Method:"eth_submitWork",Params:[]string{nonce,blockHash,mixHash},Id:1,Jsonrpc:"2.0"}

	log.Println("submit work:",getWorkInfo.Params)

	getWorkInfoBuffs ,_ :=json.Marshal(getWorkInfo)

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil{
		log.Println(err)
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	log.Println("Submit reback",string(body))

}
func GetWorkHead()( *types.Header,string) {

	url := rpcUrl

	getWorkInfo :=RpcInfo{Method:"eth_getWork",Params:[]string{},Id:1,Jsonrpc:"2.0"}

	getWorkInfoBuffs ,_ :=json.Marshal(getWorkInfo)




	req, err := http.NewRequest("POST", url, bytes.NewBuffer(getWorkInfoBuffs))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil{
		return nil,""
	}
	defer resp.Body.Close()
	body, _ := ioutil.ReadAll(resp.Body)

	workReback :=new(RpcReback)

	json.Unmarshal(body,workReback)

	newHeader :=new(types.Header)

	newHeader.Number =util.HexToBig(workReback.Result[3])

	newHeader.Difficulty = util.TargetHexToDiff(workReback.Result[2])

	log.Println("New block",newHeader.Number,newHeader.Difficulty,workReback.Result[0])

	return newHeader,workReback.Result[0]
}
