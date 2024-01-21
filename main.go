/******************************************************************************\
* Copyright (C) 2024-2024 The Molibackup Authors. All rights reserved.         *
* Licensed under the Apache version 2.0 License                                *
* Homepage: https://github.com/fdupoux/molibackup                              *
\******************************************************************************/

package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sort"

	"github.com/gookit/slog"
)

const progversion = "0.1.1-dev"

const (
	ExitStatusSuccessfulExecution  = 0
	ExitStatusInvalidConfiguration = 1
	ExitStatusFailedToExecuteJobs  = 2
)

func main() {

	var jobnames []string
	errcount := 0
	jobcount := 0

	// Process options specified on the command line
	configfile := flag.String("c", "", "path to the yaml configuration file")
	showversion := flag.Bool("v", false, "show program version and exit")
	flag.Parse()

	// Show version number if requested
	if *showversion {
		fmt.Printf("molibackup version %s built with %s\n", progversion, runtime.Version())
		os.Exit(ExitStatusSuccessfulExecution)
	}

	// Initialise the logging library
	logfmtTemplateShort := "[{{datetime}}] [{{level}}] {{message}} {{data}} {{extra}}\n"
	logfmtTemplateDebug := "[{{datetime}}] [{{level}}] [{{caller}}] {{message}} {{data}} {{extra}}\n"
	logfmt := slog.NewTextFormatter()
	logfmt.SetTemplate(logfmtTemplateShort)
	logfmt.EnableColor = true
	slog.SetFormatter(logfmt)
	slog.SetLogLevel(slog.InfoLevel)

	// Print version number
	slog.Infof("molibackup version %s built with %s starting ...", progversion, runtime.Version())

	// Read the configuration file
	err := readConfiguration(*configfile)
	if err != nil {
		slog.Errorf("Failed to read configuration: %v", err)
		os.Exit(ExitStatusInvalidConfiguration)
	}

	// Configure the logging library
	loglevel := progconfig.Global["loglevel"]
	switch loglevel {
	case "error":
		slog.SetLogLevel(slog.ErrorLevel)
		logfmt.SetTemplate(logfmtTemplateShort)
	case "warn":
		slog.SetLogLevel(slog.WarnLevel)
		logfmt.SetTemplate(logfmtTemplateShort)
	case "info":
		slog.SetLogLevel(slog.InfoLevel)
		logfmt.SetTemplate(logfmtTemplateShort)
	case "debug":
		slog.SetLogLevel(slog.DebugLevel)
		logfmt.SetTemplate(logfmtTemplateDebug)
	default:
		slog.Errorf("Invalid loglevel in configuration: \"%s\"", loglevel)
		os.Exit(ExitStatusInvalidConfiguration)
	}

	// Create list of jobs sorted alphabetically
	for jobname, _ := range jobmetadefs {
		jobnames = append(jobnames, jobname)
	}
	sort.Strings(jobnames)

	// Execute all jobs defined in the configuration
	for _, jobname := range jobnames {
		jobconfig := jobmetadefs[jobname]
		if jobconfig.Enabled == true {
			slog.Infof("Running job \"%s\" ...", jobname)
			err = runJob(jobname)
			if err != nil {
				errcount++
				slog.Errorf("Failed to execute job \"%s\": %v", jobname, err)
			}
			jobcount++
		} else {
			slog.Infof("Skipping job \"%s\" as it is disabled in the configuration", jobname)
		}
	}

	if errcount > 0 {
		slog.Errorf("Have finished running jobs with %d failures out of %d jobs", errcount, jobcount)
		os.Exit(ExitStatusFailedToExecuteJobs)
	}

	slog.Infof("Have successfully executed %d jobs", jobcount)
	os.Exit(ExitStatusSuccessfulExecution)
}
