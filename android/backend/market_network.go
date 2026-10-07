package main

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"

	"github.com/mayswind/ezbookkeeping/pkg/marketquotes"
)

// Read only private deployment configuration, never serve or back up its token.
func loadMarketNetwork(root string) error {
	file, err := os.Open(filepath.Join(root, "market-network.json"))
	if errors.Is(err, os.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 4096))
	decoder.DisallowUnknownFields()
	var config marketquotes.NetworkConfig
	if err = decoder.Decode(&config); err != nil {
		return errors.New("invalid market network configuration")
	}
	var trailing interface{}
	if decoder.Decode(&trailing) != io.EOF {
		return errors.New("invalid market network configuration")
	}
	return marketquotes.Default.ConfigureNetworkBeforeStart(config)
}
