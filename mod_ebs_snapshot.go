/******************************************************************************\
* Copyright (C) 2024-2024 The Molibackup Authors. All rights reserved.         *
* Licensed under the Apache version 2.0 License                                *
* Homepage: https://github.com/fdupoux/molibackup                              *
\******************************************************************************/

package main

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/gookit/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

// Structure of the job configuration for this specific module
type JobConfigEbsSnapshot struct {
	Module          string `koanf:"module"`
	Enabled         bool   `koanf:"enabled"`
	DryRun          bool   `koanf:"dryrun"`
	Retention       int64  `koanf:"retention"`
	AwsRegion       string `koanf:"aws_region"`
	AccessKeyId     string `koanf:"accesskey_id"`
	AccessKeySecret string `koanf:"accesskey_secret"`
	InstanceId      string `koanf:"instance_id"`
	InstanceTag     string `koanf:"instance_tag"`
	VolumeTag       string `koanf:"volume_tag"`
}

type backup_ebs_snapshot struct {
	config  JobConfigEbsSnapshot
	cfg     aws.Config
	client  *ec2.Client
	volumes []ProviderAwsEbsVolume
}

var validateConfigJobdef = []ConfigEntryValidation{
	{
		entryname:  "module",
		entrytype:  "string",
		mandatory:  true,
		allowedval: []string{"ebs-snapshot"},
	},
	{
		entryname:  "enabled",
		entrytype:  "bool",
		mandatory:  false,
		defaultval: "true",
		allowedval: []string{"true", "false"},
	},
	{
		entryname:  "dryrun",
		entrytype:  "bool",
		mandatory:  false,
		defaultval: "false",
		allowedval: []string{"true", "false"},
	},
	{
		entryname:  "retention",
		entrytype:  "int",
		mandatory:  false,
		defaultval: "30",
		allowedval: nil,
	},
	{
		entryname:  "aws_region",
		entrytype:  "string",
		mandatory:  true,
		allowedval: nil,
	},
	{
		entryname:  "accesskey_id",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "",
		allowedval: nil,
	},
	{
		entryname:  "accesskey_secret",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "",
		allowedval: nil,
	},
	{
		entryname:  "instance_id",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "",
		allowedval: nil,
	},
	{
		entryname:  "instance_tag",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "",
		allowedval: nil,
	},
	{
		entryname:  "volume_tag",
		entrytype:  "string",
		mandatory:  false,
		defaultval: "",
		allowedval: nil,
	},
}

func (b *backup_ebs_snapshot) LoadConfiguration(jobname string) error {

	// Original job config before validation and defaults
	var origconf JobConfigEbsSnapshot

	// Path of the job config section relative to the root of the config file
	jobpath := fmt.Sprintf("jobs.%s", jobname)

	slog.Debugf("Getting original job configuration (before validation and defaults) ...")

	if err := kconfig.Unmarshal(jobpath, &origconf); err != nil {
		return fmt.Errorf("failed to unmarshal path %s: %v", jobpath, err)
	}

	slog.Debugf("Dump of the initial configuration:")
	slog.Debugf("- Module=\"%v\"", origconf.Module)
	slog.Debugf("- Enabled=%v", origconf.Enabled)
	slog.Debugf("- DryRun=%v", origconf.DryRun)
	slog.Debugf("- Retention=%v", origconf.Retention)
	slog.Debugf("- AwsRegion=\"%v\"", origconf.AwsRegion)
	slog.Debugf("- AccessKeyId=\"%v\"", origconf.AccessKeyId)
	slog.Debugf("- AccessKeySecret=\"%v\"", origconf.AccessKeySecret)
	slog.Debugf("- InstanceId=\"%v\"", origconf.InstanceId)
	slog.Debugf("- InstanceTag=\"%v\"", origconf.InstanceTag)
	slog.Debugf("- VolumeTag=\"%v\"", origconf.VolumeTag)

	slog.Debugf("Validating the job configuration and setting default values ...")

	if err := configValidateAndSetDefaults(jobpath, validateConfigJobdef); err != nil {
		return fmt.Errorf("failed to validate job configuration: %w", err)
	}

	slog.Debugf("Getting processed job configuration (after validation and defaults) ...")

	if err := kconfig.Unmarshal(jobpath, &b.config); err != nil {
		return fmt.Errorf("failed to unmarshal path %s: %v", jobpath, err)
	}

	slog.Debugf("Advanced validation of the job configuration ...")

	if b.config.InstanceId != "" {
		matched, _ := regexp.MatchString("^(local|i-[a-z0-9]{17})$", b.config.InstanceId)
		if matched == false {
			return fmt.Errorf("Option \"instance_id\" must be either \"local\" or in the \"i-0123456789abcdef0\" format")
		}
	}

	if b.config.InstanceTag != "" {
		matched, _ := regexp.MatchString("^[A-Za-z0-9_-]+=[A-Za-z0-9_-]+$", b.config.InstanceTag)
		if matched == false {
			return fmt.Errorf("Option \"instance_tag\" must be in the \"TagName=TagValue\" format")
		}
	}

	if b.config.VolumeTag != "" {
		matched, _ := regexp.MatchString("^[A-Za-z0-9_-]+=[A-Za-z0-9_-]+$", b.config.VolumeTag)
		if matched == false {
			return fmt.Errorf("Option \"volume_tag\" must be in the \"TagName=TagValue\" format")
		}
	}

	if b.config.Retention <= 0 {
		return fmt.Errorf("Option \"retention\" must be a valid number greater than 0")
	}

	slog.Debugf("Dump of the processed configuration:")
	slog.Debugf("- Module=\"%v\"", b.config.Module)
	slog.Debugf("- Enabled=%v", b.config.Enabled)
	slog.Debugf("- DryRun=%v", b.config.DryRun)
	slog.Debugf("- Retention=%v", b.config.Retention)
	slog.Debugf("- AwsRegion=\"%v\"", b.config.AwsRegion)
	slog.Debugf("- AccessKeyId=\"%v\"", b.config.AccessKeyId)
	slog.Debugf("- AccessKeySecret=\"%v\"", b.config.AccessKeySecret)
	slog.Debugf("- InstanceId=\"%v\"", b.config.InstanceId)
	slog.Debugf("- InstanceTag=\"%v\"", b.config.InstanceTag)
	slog.Debugf("- VolumeTag=\"%v\"", b.config.VolumeTag)

	return nil
}

func (b *backup_ebs_snapshot) InitialiseModule() error {

	var err error

	// Load the configuration using an access key pair if it has been provided in the configuration
	b.cfg, err = ProviderAwsLoadConfig(b.config.AwsRegion, b.config.AccessKeyId, b.config.AccessKeySecret)
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Dynamically determine the EC2 Instance ID if requested in the configuration
	if b.config.InstanceId == "local" {
		slog.Debugf("Trying to detect the instance ID of the local instance ...")
		b.config.InstanceId, err = ProviderAwsGetCurrentInstance(b.cfg)
		if err != nil {
			return fmt.Errorf("failed to detect the instance ID of the local instance: %w", err)
		}
		slog.Debugf("Have detected the instance ID of the local instance as %s", b.config.InstanceId)
	}

	// Create a client
	b.client = ProviderAwsNewEc2Client(b.cfg)

	// Find list of all EBS volumes that match the conditions specific in the configuration
	err = b.findRelevantVolumes()
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	return nil
}

func (b *backup_ebs_snapshot) findRelevantVolumes() error {
	var results []ProviderAwsEbsVolume
	var insttagkey, insttagval string
	var voltagkey, voltagval string

	// Parse "instance_tag" option
	if b.config.InstanceTag != "" {
		insttag := strings.Split(b.config.InstanceTag, "=")
		if len(insttag) == 2 {
			insttagkey = insttag[0]
			insttagval = insttag[1]
		}
	}

	// Parse "volume_tag" option
	if b.config.VolumeTag != "" {
		insttag := strings.Split(b.config.VolumeTag, "=")
		if len(insttag) == 2 {
			voltagkey = insttag[0]
			voltagval = insttag[1]
		}
	}

	// Get list of instances that match the conditions specified
	slog.Debugf("Listing instances based on instance_id=\"%s\" and instance_tag_key=\"%s\" and instance_tag_val=\"%s\" ...",
		b.config.InstanceId, insttagkey, insttagval)
	instances, err := ProviderAwsGetEc2Instances(b.client, b.config.InstanceId, insttagkey, insttagval)
	if err != nil {
		return fmt.Errorf("%w", err)
	}
	if len(instances) == 0 {
		slog.Warnf("Have not found any instance matching the conditions")
	}

	// Go through each instance
	for _, instance := range instances {
		slog.Debugf("Found instance: instanceId=\"%s\" instanceName=\"%s\" ownerId=\"%s\"",
			instance.instanceId, instance.instanceName, instance.instanceOwner)
		volumes, err := ProviderAwsGetEbsVolumes(b.client, instance.instanceId, voltagkey, voltagval)
		if err != nil {
			return fmt.Errorf("%w", err)
		}

		// Go through each volume
		for _, curvol := range volumes {
			slog.Debugf("Found volume: volumeId=\"%s\" volumeName=\"%s\" instanceId=\"%s\"",
				curvol.volumeId, curvol.volumeName, instance.instanceId)
			results = append(results, curvol)
		}
	}
	if len(results) == 0 {
		slog.Warnf("Have not found any volume matching the conditions")
	}

	b.volumes = results
	return nil
}

func (b *backup_ebs_snapshot) CreateBackup() error {
	var basename string

	for _, curvol := range b.volumes {
		slog.Debugf("Considering backup for volume: volumeId=\"%s\" volumeName=\"%s\" ...", curvol.volumeId, curvol.volumeName)
		if curvol.volumeName != "" {
			basename = curvol.volumeName
		} else {
			basename = curvol.volumeId
		}
		curtime := time.Now()
		snapname := fmt.Sprintf("%s-%s", basename, curtime.Format(time.RFC3339))
		snapdate := fmt.Sprintf("%04d%02d%02d", curtime.Year(), curtime.Month(), curtime.Day())
		snaptime := fmt.Sprintf("%v", curtime.Unix())
		if b.config.DryRun == false {
			snapshotId, err := ProviderAwsCreateEbsSnapshot(b.client, curvol.volumeId, snapname, snapdate, snaptime)
			if err != nil {
				return fmt.Errorf("%w", err)
			}
			slog.Infof("Successfully created snapshot \"%s\" of volume \"%s\"", snapshotId, curvol.volumeId)
		} else {
			slog.Infof("Dryrun: Not creating snapshot of volume \"%s\"", curvol.volumeId)
		}
	}

	return nil
}

func (b *backup_ebs_snapshot) ListBackups() ([]BackupItem, error) {
	var resultsOrdered []BackupItem
	var snapshotNames []string
	resultsUnordered := make(map[string]BackupItem)

	// Enumerate volumes and their snapshots to get a list of relevant snapshots
	for _, curvol := range b.volumes {
		slog.Debugf("Listing snapshots from volume: volumeId=\"%s\" ...", curvol.volumeId)

		snapshots, err := ProviderAwsGetEbsSnapshots(b.client, curvol.volumeId)
		if err != nil {
			return nil, err
		}

		for _, snapshot := range snapshots {
			item := BackupItem{}
			item.identifier = snapshot.snapshotId
			item.description = snapshot.snapshotDesc
			item.timestamp = snapshot.snapshotTime
			snapshotNames = append(snapshotNames, item.description)
			resultsUnordered[item.description] = item
			snaptime := time.Unix(snapshot.snapshotTime, 0)
			slog.Debugf("Found snapshot: id=\"%s\" desc=\"%s\" created=\"%v\" vol=\"%s\"",
				snapshot.snapshotId, snapshot.snapshotDesc, snaptime.Format(time.RFC3339), snapshot.volumeId)
		}
	}

	// Reorder the snapshots names alphabetically
	sort.Strings(snapshotNames)

	// Create the final list of items in the alphabetical order
	for _, snapname := range snapshotNames {
		resultsOrdered = append(resultsOrdered, resultsUnordered[snapname])
	}

	return resultsOrdered, nil
}

func (b *backup_ebs_snapshot) DeleteOldBackups(bkpitems []BackupItem) error {

	retention := b.config.Retention
	curtime := time.Now().Unix()

	for _, item := range bkpitems {
		snapshotAge := (curtime - item.timestamp) / 86400
		snapDelete := snapshotAge > retention
		slog.Debugf("Considering deletion of snapshot: id=\"%s\" desc=\"%s\" age=%v retention=%v ...",
			item.identifier, item.description, snapshotAge, retention)
		if snapDelete == true {
			if b.config.DryRun == false {
				err := ProviderAwsDeleteEbsSnapshot(b.client, item.identifier)
				if err != nil {
					return fmt.Errorf("%w", err)
				}
				slog.Infof("Deleted snapshot: id=\"%s\" desc=\"%s\" age=%v retention=%v", item.identifier, item.description, snapshotAge, retention)
			} else {
				slog.Infof("Dryrun: Not deleting snapshot: id=\"%s\" desc=\"%s\" age=%d retention=%v", item.identifier, item.description, snapshotAge, retention)
			}
		} else {
			slog.Infof("Keeping snapshot: id=\"%s\" desc=\"%s\" age=%d retention=%d", item.identifier, item.description, snapshotAge, retention)
		}
	}

	return nil
}
