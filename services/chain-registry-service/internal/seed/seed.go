package seed

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	shpg "github.com/quangdang46/NFT-Marketplace/shared/postgres"
)

type ContractSeed struct {
	ChainCAIP2  string
	Name        string
	Address     string
	Standard    string
	AbiFileName string
}

func RunStartupSeed(pg *shpg.Postgres) error {
	abiNameFolder := "abi"

	_, currFile, _, _ := runtime.Caller(0)
	abiPath := filepath.Join(filepath.Dir(currFile), abiNameFolder)
	abiDir, err := findAbiDirectory([]string{
		abiPath,
	})

	if err != nil {
		return fmt.Errorf("find abi directory: %w", err)
	}

	seeds := anvilSeeds()

	for _, seed := range seeds {
		if err := seedOne(pg, abiDir, seed); err != nil {
			return fmt.Errorf("seed %s: %w", seed.Name, err)
		}
	}
	return nil
}

func findAbiDirectory(candidates []string) (string, error) {
	for _, p := range candidates {

		st, err := os.Stat(p)

		if err == nil && st.IsDir() {
			return p, nil
		}
	}

	return "", errors.New("abi directory not found; checked paths: " + strings.Join(candidates, ", "))
}

func seedOne(pg *shpg.Postgres, abiDir string, seed ContractSeed) error {
	abiPath := filepath.Join(abiDir, seed.AbiFileName)

	abiBytes, sha, size, err := readAbi(abiPath)
	if err != nil {
		return fmt.Errorf("failed to read ABI: %w", err)
	}

	if isExistContract := IsContractExists(pg, seed.ChainCAIP2, strings.ToLower(seed.Address)); isExistContract {
		log.Printf("Contract %s already exists, skipping", seed.Name)
		return nil
	}

	if err := upsertAbiBlob(pg, sha, size, seed.Name, seed.Standard, abiBytes); err != nil {
		return fmt.Errorf("failed to upsert ABI blob: %w", err)
	}

	if err := upsertContract(pg, seed.ChainCAIP2, seed.Name, strings.ToLower(seed.Address), seed.Standard, sha); err != nil {
		return fmt.Errorf("failed to upsert contract: %w", err)
	}

	return nil
}

func IsContractExists(pg *shpg.Postgres, caip2, address string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	var exists bool
	err := pg.GetClient().QueryRowContext(ctx, `
		SELECT EXISTS(
			SELECT 1 FROM chain_contracts cc
			JOIN chains c ON cc.chain_id = c.id
			WHERE c.caip2 = $1 AND cc.address = $2
		)
	`, caip2, address).Scan(&exists)

	if err != nil {
		log.Printf("failed to check contract existence: %v", err)
		return false
	}

	return exists
}

func readAbi(path string) (raw []byte, sha string, size int, err error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, "", 0, err
	}
	var tmp map[string]interface{}
	if err := json.Unmarshal(b, &tmp); err != nil {
		return nil, "", 0, fmt.Errorf("invalid ABI JSON: %w", err)
	}
	h := sha256.Sum256(b)
	return b, hex.EncodeToString(h[:]), len(b), nil
}

func upsertAbiBlob(pg *shpg.Postgres, sha string, size int, name, standard string, abiBytes []byte) error {
	_, err := pg.GetClient().Exec(
		`INSERT INTO abi_blobs (sha256, size_bytes, source, compiler, contract_name, standard, abi_json, s3_key)
		 VALUES ($1, $2, 'internal', NULL, $3, $4, $5::jsonb, $6)
		 ON CONFLICT (sha256) DO UPDATE SET contract_name = EXCLUDED.contract_name, standard = EXCLUDED.standard, abi_json = EXCLUDED.abi_json`,
		sha, size, name, strings.ToLower(standard), string(abiBytes), fmt.Sprintf("local/%s", name),
	)
	if err != nil {
		return err
	}
	return nil
}

func upsertContract(pg *shpg.Postgres, caip2, name, address, standard, sha string) error {
	// First, let's check if the chain exists and get both id and chain_numeric
	var chainID int
	var chainNumeric int
	err := pg.GetClient().QueryRow("SELECT id, chain_numeric FROM chains WHERE caip2 = $1", caip2).Scan(&chainID, &chainNumeric)
	if err != nil {
		return fmt.Errorf("chain not found for caip2 %s: %w", caip2, err)
	}

	_, err = pg.GetClient().Exec(
		`INSERT INTO chain_contracts (chain_id, name, address, standard, abi_sha256)
		 VALUES ($1, $2, $3, $4, $5)
		 ON CONFLICT (chain_id, address)
		 DO UPDATE SET name = EXCLUDED.name, standard = EXCLUDED.standard, abi_sha256 = EXCLUDED.abi_sha256`,
		chainID, name, address, strings.ToUpper(standard), sha,
	)
	if err != nil {
		return err
	}
	return nil
}

func anvilSeeds() []ContractSeed {
	return []ContractSeed{
		{
			ChainCAIP2:  "eip155:31337",
			Name:        "ERC721CollectionFactory",
			Address:     "0xe7f1725E7734CE288F8367e1Bb143E90bb3F0512",
			Standard:    "CUSTOM",
			AbiFileName: "ERC721CollectionFactory.json",
		},
		{
			ChainCAIP2:  "eip155:31337",
			Name:        "ERC1155CollectionFactory",
			Address:     "0x9fE46736679d2D9a65F0992F2272dE9f3c7fa6e0",
			Standard:    "CUSTOM",
			AbiFileName: "ERC1155CollectionFactory.json",
		},
	}
}
