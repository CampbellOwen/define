// Copyright © 2018 Trevor N. Suarez (Rican7)

// Package config provides types and handling for configuration values used in
// the application.
package config

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/Rican7/define/registry"
	"github.com/fatih/structs"
	homedir "github.com/mitchellh/go-homedir"
	flag "github.com/ogier/pflag"

	"github.com/imdario/mergo"
)

// Configuration defines the application's configuration structure
type Configuration struct {
	IndentationSize uint
	PreferredSource string

	// Private fields that shouldn't be externally set or output
	providerConfigs    map[string]registry.Configuration
	configFileLocation string
}

// initializeCommandLineConfig initializes the command line configuration.
func initializeCommandLineConfig(flags *flag.FlagSet) *Configuration {
	var conf Configuration

	// Define our flags
	flags.StringVarP(&conf.configFileLocation, "config-file", "c", "", "The location of the config file to use")
	flags.UintVar(&conf.IndentationSize, "indent-size", 0, "The number of spaces to indent output by")
	flags.StringVar(&conf.PreferredSource, "preferred-source", "", "The preferred source to use, if available")

	return &conf
}

// initializeFileConfig initializes the file configuration by loading the
// configuration from a file at the given location.
func initializeFileConfig(fileLocation string) (Configuration, error) {
	var conf Configuration

	// If we can expand the location, do so
	if expanded, err := homedir.Expand(fileLocation); nil == err {
		fileLocation = expanded
	}

	fileContents, err := ioutil.ReadFile(fileLocation)

	if nil != err {
		return conf, err
	}

	if len(fileContents) > 0 {
		err = json.Unmarshal(fileContents, &conf)
	}

	return conf, err
}

// initializeEnvironmentConfig initializes the environment configuration from
// the application's environment.
func initializeEnvironmentConfig() Configuration {
	var conf Configuration

	if val, err := strconv.ParseUint(os.Getenv("DEFINE_APP_INDENT_SIZE"), 10, 0); nil == err {
		conf.IndentationSize = uint(val)
	}

	conf.PreferredSource = os.Getenv("DEFINE_APP_PREFERRED_SOURCE")

	return conf
}

// mergeConfigurations merges multiple configurations values together, from left
// to right argument position, by filling any of the left arguments zero-values
// with any non-zero-values from the right.
func mergeConfigurations(confs ...Configuration) (Configuration, error) {
	var merged Configuration

	for _, conf := range confs {
		if err := mergo.Merge(&merged, conf); nil != err {
			return merged, err
		}
	}

	return merged, nil
}

// NewFromRuntime builds a Configuration by merging values from multiple
// different sources. It accepts a Configuration containing default values to
// fill in any empty/blank configuration values found when merging from the
// different sources.
//
// The merging of values from different sources will take this priority:
// 1. Command line arguments
// 2. A loaded config file, if available
// 3. Environment variables
// 4. Passed in default values
func NewFromRuntime(
	flags *flag.FlagSet,
	providerConfigs map[string]registry.Configuration,
	defaultConfigFileLocation string,
	defaults Configuration,
) (Configuration, error) {

	var conf Configuration
	var err error

	var fileConfig Configuration

	// Set our config file location
	defaults.configFileLocation = defaultConfigFileLocation

	commandLineConfig := initializeCommandLineConfig(flags)

	// Parse our flag set, as we need the values from the commandLineConfig
	err = flags.Parse(os.Args[1:])

	if nil == err {
		configFileLocation := commandLineConfig.configFileLocation

		if "" == configFileLocation && "" != defaults.configFileLocation {
			// If we can expand the location, do so
			if expanded, err := homedir.Expand(defaults.configFileLocation); nil == err {
				defaults.configFileLocation = expanded
			}

			// If we haven't passed a config file flag, and our default exists
			if _, err := os.Stat(defaults.configFileLocation); !os.IsNotExist(err) {
				// Set our location to the default, since it exists
				// (if there are problems reading the file, we'll handle later)
				configFileLocation = defaults.configFileLocation
			}
		}

		// If we have a config file to load
		if "" != configFileLocation {
			fileConfig, err = initializeFileConfig(configFileLocation)

			if nil != err {
				err = errors.New("error reading config file")
			}
		}
	}

	if nil == err {
		conf, err = mergeConfigurations(
			*commandLineConfig,
			fileConfig,
			initializeEnvironmentConfig(),
			defaults,
		)
	}

	conf.providerConfigs = providerConfigs

	return conf, err
}

// ProviderConfigs returns the configurations of the source providers.
func (c Configuration) ProviderConfigs() []registry.Configuration {
	var list []registry.Configuration

	for _, providerConfig := range c.providerConfigs {
		list = append(list, providerConfig)
	}

	return list
}

// MarshalJSON defines how the configuration should be JSON marshalled.
func (c Configuration) MarshalJSON() ([]byte, error) {
	configMap := structs.Map(c)

	for _, providerConf := range c.providerConfigs {
		// Skip nil and zero-value configs
		if nil == providerConf || len(structs.Fields(providerConf)) < 1 {
			continue
		}

		configMap[providerConf.JSONKey()] = providerConf
	}

	return json.Marshal(configMap)
}

// UnmarshalJSON defines how the configuration should be JSON unmarshalled.
func (c *Configuration) UnmarshalJSON(data []byte) error {
	var err error

	// Alias our type so that we can unmarshal as usual
	type conf Configuration

	// Unmarshal our base configuration
	err = json.Unmarshal(data, (*conf)(c))

	if nil != err {
		return err
	}

	configMap := make(map[string]*json.RawMessage)

	err = json.Unmarshal(data, &configMap)

	if nil != err {
		return err
	}

	if nil == c.providerConfigs {
		c.providerConfigs = make(map[string]registry.Configuration)
	}

	for conf := range registry.Providers() {
		// If we have config data that matches a provider config
		if rawConf, exists := configMap[conf.JSONKey()]; exists {
			// Directly unmarshal the data into the provider config
			json.Unmarshal([]byte(*rawConf), conf)

			c.providerConfigs[conf.JSONKey()] = conf
		}
	}

	return err
}
