package fxtrigger

import (
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io/ioutil"
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
	bytes, err := ioutil.ReadFile(filepath.Clean(path))
	if err != nil {
		log.Error().Err(err).Msg("Invalid Config Path")
		return nil
	}

	err = yaml.Unmarshal(bytes, c)
	if err != nil {
		log.Error().Err(err).Msg("Unable to marshall successfully")
		return nil
	}

	return c
}
