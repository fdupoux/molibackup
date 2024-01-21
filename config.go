/******************************************************************************\
* Copyright (C) 2024-2024 The Molibackup Authors. All rights reserved.         *
* Licensed under the Apache version 2.0 License                                *
* Homepage: https://github.com/fdupoux/molibackup                              *
\******************************************************************************/

package main

import (
	"fmt"
	"os"
	"path"
	"runtime"
	"strings"

	"golang.org/x/exp/slices"

	"github.com/gookit/slog"

	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Structure of the whole configuration file
type ProgramConfig struct {
	Global  map[string]interface{} `koanf:"global"`
	Jobsdef map[string]interface{} `koanf:"jobs"`
}

// Job attributes which are common to all job configs
type JobMetaConfig struct {
	Module    string `koanf:"module"`
	Enabled   bool   `koanf:"enabled"`
	DryRun    bool   `koanf:"dryrun"`
	Retention int    `koanf:"retention"`
}

// Structures for rules to validate config entries
type ConfigEntryValidation struct {
	entryname  string
	entrytype  string
	mandatory  bool
	defaultval string
	allowedval []string
}

// Rules to validate the global section of the config file
var validateConfigGlobal = []ConfigEntryValidation{
	{
		entryname:  "loglevel",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "info",
		allowedval: []string{"error", "warn", "info", "debug"},
	},
}

var kconfig = koanf.New(".")
var kparser = yaml.Parser()
var progconfig ProgramConfig
var jobmetadefs map[string]JobMetaConfig

func readConfiguration(configfile string) error {

	var configPaths []string
	var err error

	// Use the configuration file specified on the command line if requested
	if configfile != "" {
		configPaths = append(configPaths, configfile)
	} else { // Search for the configuration file in the default locations
		homedir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get the user home directory: %w", err)
		}
		dircwd, err := os.Getwd()
		if err != nil {
			return fmt.Errorf("failed to get current working directory: %w", err)
		}
		pathexe, err := os.Executable()
		if err != nil {
			return fmt.Errorf("failed to find path to executable: %w", err)
		}
		direxe := path.Dir(pathexe)

		// List of paths where to try to find the configuration file
		dirname := "molibackup"
		filename := "molibackup.yaml"
		configPaths = append(configPaths, path.Join(homedir, dirname, filename))
		configPaths = append(configPaths, path.Join(dircwd, dirname, filename))
		configPaths = append(configPaths, path.Join(direxe, dirname, filename))
		if runtime.GOOS == "linux" || runtime.GOOS == "darwin" {
			configPaths = append(configPaths, path.Join("/etc", dirname, filename))
		}
	}

	// Try to find a configuration file in the paths from the list
	configPath := ""
	for _, fullpath := range configPaths {
		slog.Debugf("Attempting to find configuration in \"%s\"", fullpath)
		_, err := os.Stat(fullpath)
		if err == nil {
			configPath = fullpath
			break
		}
	}
	if configPath == "" {
		return fmt.Errorf("could not find the configuration file in any of the following locations: %s", strings.Join(configPaths, ","))
	}
	slog.Infof("Found configuration file in %s", configPath)

	// Load configuration file
	if err := kconfig.Load(file.Provider(configPath), kparser); err != nil {
		return fmt.Errorf("failed to read configuration file %s: %v", configPath, err)
	}

	err = configValidateAndSetDefaults("global", validateConfigGlobal)
	if err != nil {
		return fmt.Errorf("failed to validate the global config section: %w", err)
	}

	// Parse the whole configuration file
	if err := kconfig.Unmarshal("", &progconfig); err != nil {
		return fmt.Errorf("failed to unmarshal the configuration file: %v", err)
	}

	jobmetadefs = make(map[string]JobMetaConfig)
	for jobname, _ := range progconfig.Jobsdef {
		var jobconf JobMetaConfig
		cfgpath := fmt.Sprintf("jobs.%s", jobname)
		if err = kconfig.Unmarshal(cfgpath, &jobconf); err != nil {
			return fmt.Errorf("failed to unmarshal path %s: %v", cfgpath, err)
		}
		jobmetadefs[jobname] = jobconf
	}

	// Make sure all job configuration sections have a "module" entry
	validmods := []string{"ebs-snapshot"}
	for jobname, jobconf := range jobmetadefs {
		if jobconf.Module == "" {
			return fmt.Errorf("the configuration section for job \"%s\" has no \"module\" entry", jobname)
		}
		if slices.Contains(validmods, jobconf.Module) == false {
			return fmt.Errorf("the configuration section for job \"%s\" has an invalid value \"%s\" for \"module\"", jobname, jobconf.Module)
		}
	}

	if len(jobmetadefs) == 0 {
		slog.Warnf("Have not found any job definition in the configuration, there is nothing to do")
	}

	return nil
}

// Process validation rules on a section of the configuration and use default values on entries having no value
func configValidateAndSetDefaults(configpath string, validation []ConfigEntryValidation) error {

	var knownEntries []string
	var configmap = make(map[string]interface{})

	err := kconfig.Unmarshal(configpath, &configmap)
	if err != nil {
		return fmt.Errorf("failed to unmarshal path %s: %v", configpath, err)
	}

	// Make sure all validation rules are met for all entries in the map
	for _, entry := range validation {
		knownEntries = append(knownEntries, entry.entryname)
		// Make sure the entry does not have invalid values and meets all conditions
		entryval, hasentry := configmap[entry.entryname]
		if hasentry == true {
			// Make sure the entry has the right type
			typefound := fmt.Sprintf("%T", entryval)
			if entry.entrytype != "" && typefound != entry.entrytype {
				return fmt.Errorf("entry \"%s\" in path \"%s\" has the wrong type: found=%s expected=%s",
					entry.entryname, configpath, typefound, entry.entrytype)
			}
			// Make sure the entry is set to a value which is allowed if there are restrictions
			entryvalstr := fmt.Sprintf("%v", entryval)
			if (entry.allowedval != nil) && (slices.Contains(entry.allowedval, entryvalstr) == false) {
				return fmt.Errorf("value \"%v\" is invalid for configuration entry \"%s\" as it must be one of %v", entryval, entry.entryname, entry.allowedval)
			}
		} else {
			if entry.mandatory == true {
				return fmt.Errorf("configuration entry \"%s\" must be specified", entry.entryname)
			} else {
				if entry.defaultval != "" {
					keypath := fmt.Sprintf("%s.%s", configpath, entry.entryname)
					slog.Debugf("Setting default value for config key with path=\"%s\" value=\"%v\"", keypath, entry.defaultval)
					err := kconfig.Set(keypath, entry.defaultval)
					if err != nil {
						return fmt.Errorf("failed to set default value for key at path %s: %v", keypath, err)
					}
				}
			}
		}
	}

	// Make sure all entries in the map are known entries
	for key, _ := range configmap {
		if slices.Contains(knownEntries, key) == false {
			return fmt.Errorf("entry \"%v\" is not a valid entry in this section of the configuration", key)
		}
	}

	return nil
}
