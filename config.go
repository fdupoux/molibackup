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

	"gopkg.in/yaml.v3"
)

type ConfigEntryValidation struct {
	entryname  string
	mandatory  bool
	defaultval string
	allowedval string
}

// Map which stores the global configuration section
var configGlobal map[string]string

// Map which stores the job configuration sections
var configJobs map[string]interface{}

func processConfigEntry(mapping map[string]string, key string, val interface{}) error {
	switch val.(type) { // Type assertion
	case string:
		mapping[key] = fmt.Sprintf("%s", val.(string))
	case int, int32, int64:
		mapping[key] = fmt.Sprintf("%d", val.(int))
	case float32, float64:
		mapping[key] = fmt.Sprintf("%f", val.(float64))
	case bool:
		mapping[key] = fmt.Sprintf("%t", val.(bool))
	case nil:
		return fmt.Errorf("unexpected empty value for key \"%s\"", key)
	default:
		return fmt.Errorf("unexpected value type %T for key \"%s\"", val, key)
	}

	return nil
}

func parseConfigGlobal(input interface{}) error {

	for key, val := range input.(map[string]interface{}) {
		err := processConfigEntry(configGlobal, key, val)
		if err != nil {
			return fmt.Errorf("error in the global section: %w", key)
		}
	}

	return nil
}

func parseConfigJobsconf(input interface{}) error {

	for jobname, jobconf := range input.(map[string]interface{}) {
		jobconfig := make(map[string]string)
		for key, val := range jobconf.(map[string]interface{}) {
			err := processConfigEntry(jobconfig, key, val)
			if err != nil {
				return fmt.Errorf("error in the section for job \"%s\": %w", jobname, err)
			}
		}
		if _, exists := configJobs[jobname]; exists {
			return fmt.Errorf("found duplicate definition for job \"%s\"", jobname)
		}
		configJobs[jobname] = jobconfig
	}

	return nil
}

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

	yamldata, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read configuration file \"%s\": %v", configPath, err)
	}

	yamlconfig := make(map[string]interface{})

	err = yaml.Unmarshal(yamldata, &yamlconfig)
	if err != nil {
		return fmt.Errorf("failed to parse yaml in configuration file \"%s\": %v", configPath, err)
	}

	// Populate maps for each configuration section
	for key, val := range yamlconfig {
		switch key {
		case "global":
			if val != nil {
				err = parseConfigGlobal(val)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
			}
		case "jobs":
			if val != nil {
				err = parseConfigJobsconf(val)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
			}
		default:
			return fmt.Errorf("ERROR: Unexpected key \"%s\" in the configuration file", key)
		}
	}

	// Validate each section of the configuration
	validateConfigGlobal := []ConfigEntryValidation{
		{
			entryname:  "loglevel",
			mandatory:  false,
			defaultval: "info",
			allowedval: "error,warn,info,debug",
		},
	}
	err = validateConfigMap(configGlobal, validateConfigGlobal)
	if err != nil {
		return fmt.Errorf("failed to validate the global config section: %w", err)
	}

	// Check all job configuration sections have a 'module' entry
	validmods := []string{"ebs-snapshot"}
	for jobname, jobdata := range configJobs {
		jobconfig := jobdata.(map[string]string)
		entryval, hasentry := jobconfig["module"]
		if hasentry == false {
			return fmt.Errorf("the configuration section for job \"%s\" has no 'module' entry", jobname)
		}
		if slices.Contains(validmods, entryval) == false {
			return fmt.Errorf("the configuration section for job \"%s\" has an invalid value \"%s\" for 'module'", jobname, entryval)
		}
	}

	if len(configJobs) == 0 {
		slog.Warnf("Have not found any job definition in the configuration, there is nothing to do")
	}

	return nil
}

func validateConfigMap(configmap map[string]string, validation []ConfigEntryValidation) error {

	var knownEntries []string

	// Make sure all validation rules are met for all entries in the map
	for _, entry := range validation {
		knownEntries = append(knownEntries, entry.entryname)
		// Get list of valid values for the current entry
		var allowedval []string
		if entry.allowedval != "" {
			allowedval = strings.Split(entry.allowedval, ",")
		}
		// Make sure the entry does not have invalid values and meets all conditions
		entryval, hasentry := configmap[entry.entryname]
		if hasentry == true {
			if (len(allowedval) > 0) && (slices.Contains(allowedval, entryval) == false) {
				return fmt.Errorf("value '%v' is invalid for configuration entry \"%s\" as it must be one of %v", entryval, entry.entryname, allowedval)
			}
		} else {
			if entry.mandatory == true {
				return fmt.Errorf("configuration entry \"%s\" must be specified", entry.entryname)
			} else {
				configmap[entry.entryname] = entry.defaultval
			}
		}
	}

	// Make sure all entries in the map are known entries
	for key, _ := range configmap {
		if slices.Contains(knownEntries, key) == false {
			return fmt.Errorf("entry '%v' is not a valid entry in this section of the configuration", key)
		}
	}

	return nil
}
