/******************************************************************************\
* Copyright (C) 2024-2024 The Molibackup Authors. All rights reserved.         *
* Licensed under the Apache version 2.0 License                                *
* Homepage: https://github.com/fdupoux/molibackup                              *
\******************************************************************************/

package main

import (
	"fmt"
)

type BackupModule interface {
	LoadConfiguration(jobname string) error
	InitialiseModule() error
	CreateBackup() error
	ListBackups() ([]BackupItem, error)
	DeleteOldBackups([]BackupItem) error
}

type BackupItem struct {
	identifier  string
	description string
	timestamp   int64
}

func runJob(jobname string) error {

	var err error
	var module BackupModule

	jobconf, ok := jobmetadefs[jobname]
	if ok == false {
		return fmt.Errorf("configuration for job \"%s\" not found in the map", jobname)
	}

	switch jobconf.Module {
	case "ebs-snapshot":
		module = &backup_ebs_snapshot{}
	default:
		return fmt.Errorf("invalid type of backup module: \"%s\"", jobconf.Module)
	}

	// Load backup job configuration
	err = module.LoadConfiguration(jobname)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Initialise the backup job
	err = module.InitialiseModule()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Create a new backup
	err = module.CreateBackup()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// List existing backups
	bkpitems, err := module.ListBackups()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Delete backups older than retention period
	err = module.DeleteOldBackups(bkpitems)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}
