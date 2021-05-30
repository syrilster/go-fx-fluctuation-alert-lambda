package fxTrigger

import (
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"io/ioutil"
)

type Config struct {
	ToEmail string `yaml:"toEmail"`
}

func (c *Config) getConfig(path string) *Config {
	bytes, err := ioutil.ReadFile(path)
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
