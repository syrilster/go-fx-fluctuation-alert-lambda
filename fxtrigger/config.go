package fxtrigger

import (
	"gopkg.in/yaml.v3"
	"io/ioutil"
	"log/slog"
	"os"
	"path/filepath"
)

type Config struct {
	ToEmail          string  //from Env
	FromEmail        string  `yaml:"fromEmail"`
	AWSRegion        string  `yaml:"awsRegion"`
	ExchangeEndpoint string  `yaml:"exchangeEndpoint"`
	ThresholdPercent float64 `yaml:"thresholdPercent"`
	FXTableName      string  `yaml:"fxTableName"`
	LowerBound       float64 //From env
	UpperBound       float64 //From env
	AppID            string  //From env
}

func (c *Config) getConfig(path string) *Config {
	log := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	bytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Error("Invalid Config Path", slog.Any("error", err))
		return nil
	}

	err = yaml.Unmarshal(bytes, c)
	if err != nil {
		log.Error("Unable to marshall successfully", slog.Any("error", err))
		return nil
	}
	return c
}
