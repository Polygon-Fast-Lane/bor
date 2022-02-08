package chains

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"os"

	"github.com/ethereum/go-ethereum/common"
	"github.com/ethereum/go-ethereum/core"
)

type Chain struct {
	Hash      common.Hash
	Genesis   *core.Genesis
	Bootnodes []string
	NetworkId uint64
	DNS       []string
}

var chains = map[string]*Chain{
	"mainnet": mainnetBor,
	"mumbai":  mumbaiTestnet,
}

func GetChain(name string) (*Chain, error) {
	var chain *Chain
	var err error
	if _, fileErr := os.Stat(name); fileErr == nil {
		if chain, err = ImportFromFile(name); err != nil {
			return nil, fmt.Errorf("error importing chain from file: %v", err)
		}
		return chain, nil
	} else if errors.Is(fileErr, os.ErrNotExist) {
		var ok bool
		if chain, ok = chains[name]; !ok {
			return nil, fmt.Errorf("chain %s not found", name)
		}
		return chain, nil
	} else {
		return nil, fileErr
	}
}

func ImportFromFile(filename string) (*Chain, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	return importChain(data)
}

func importChain(content []byte) (*Chain, error) {
	var chain *Chain

	if err := json.Unmarshal(content, &chain); err != nil {
		return nil, err
	}

	return chain, nil
}