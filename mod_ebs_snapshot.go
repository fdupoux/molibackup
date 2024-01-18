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
	"strconv"
	"strings"
	"time"

	"github.com/gookit/slog"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
)

type backup_ebs_snapshot struct {
	config  map[string]string
	cfg     aws.Config
	client  *ec2.Client
	volumes []ProviderAwsEbsVolume
}

func (b *backup_ebs_snapshot) LoadConfiguration(config map[string]string) error {
	b.config = config

	slog.Debugf("Dump of the initial configuration:")
	for key, val := range b.config {
		slog.Debugf("- %s=\"%s\"", key, val)
	}

	// Validate specific fields in the job configuration section
	validateConfigJobdef := []ConfigEntryValidation{
		{
			entryname:  "module",
			mandatory:  true,
			allowedval: "ebs-snapshot",
		},
		{
			entryname:  "enabled",
			mandatory:  false,
			defaultval: "true",
			allowedval: "true,false",
		},
		{
			entryname:  "aws_region",
			mandatory:  true,
			allowedval: "",
		},
		{
			entryname:  "dryrun",
			mandatory:  false,
			defaultval: "false",
			allowedval: "true,false",
		},
		{
			entryname:  "retention",
			mandatory:  false,
			defaultval: "30",
			allowedval: "",
		},
		{
			entryname:  "accesskey_id",
			mandatory:  false,
			defaultval: "",
			allowedval: "",
		},
		{
			entryname:  "accesskey_secret",
			mandatory:  false,
			defaultval: "",
			allowedval: "",
		},
		{
			entryname:  "instance_id",
			mandatory:  false,
			defaultval: "",
			allowedval: "",
		},
		{
			entryname:  "instance_tag",
			mandatory:  false,
			defaultval: "",
			allowedval: "",
		},
		{
			entryname:  "volume_tag",
			mandatory:  false,
			defaultval: "",
			allowedval: "",
		},
	}

	slog.Debugf("Validating the job configuration ...")
	err := validateConfigMap(b.config, validateConfigJobdef)
	if err != nil {
		return fmt.Errorf("failed to validate job configuration: %w", err)
	}

	if b.config["instance_id"] != "" {
		matched, _ := regexp.MatchString("^(local|i-[a-z0-9]{17})$", b.config["instance_id"])
		if matched == false {
			return fmt.Errorf("Option \"instance_id\" must be either \"local\" or in the \"i-0123456789abcdef0\" format")
		}
	}

	if b.config["instance_tag"] != "" {
		matched, _ := regexp.MatchString("^[A-Za-z0-9_-]+=[A-Za-z0-9_-]+$", b.config["instance_tag"])
		if matched == false {
			return fmt.Errorf("Option \"instance_tag\" must be in the \"TagName=TagValue\" format")
		}
	}

	if b.config["volume_tag"] != "" {
		matched, _ := regexp.MatchString("^[A-Za-z0-9_-]+=[A-Za-z0-9_-]+$", b.config["volume_tag"])
		if matched == false {
			return fmt.Errorf("Option \"volume_tag\" must be in the \"TagName=TagValue\" format")
		}
	}

	retention, err := strconv.ParseInt(b.config["retention"], 10, 64)
	if err != nil || retention <= 0 {
		return fmt.Errorf("Option \"retention\" must be a valid number greater than 0")
	}

	slog.Debugf("Dump of the processed configuration:")
	for key, val := range b.config {
		slog.Debugf("- %s=\"%s\"", key, val)
	}

	return nil
}

func (b *backup_ebs_snapshot) InitialiseModule() error {

	var err error

	// Load the configuration using an access key pair if it has been provided in the configuration
	b.cfg, err = ProviderAwsLoadConfig(b.config["aws_region"], b.config["accesskey_id"], b.config["accesskey_secret"])
	if err != nil {
		return fmt.Errorf("%w", err)
	}

	// Dynamically determine the EC2 Instance ID if requested in the configuration
	if b.config["instance_id"] == "local" {
		slog.Debugf("Trying to detect the instance ID of the local instance ...")
		b.config["instance_id"], err = ProviderAwsGetCurrentInstance(b.cfg)
		if err != nil {
			return fmt.Errorf("failed to detect the instance ID of the local instance: %w", err)
		}
		slog.Debugf("Have detected the instance ID of the local instance as %s", b.config["instance_id"])
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
	if b.config["instance_tag"] != "" {
		insttag := strings.Split(b.config["instance_tag"], "=")
		if len(insttag) == 2 {
			insttagkey = insttag[0]
			insttagval = insttag[1]
		}
	}

	// Parse "volume_tag" option
	if b.config["volume_tag"] != "" {
		insttag := strings.Split(b.config["volume_tag"], "=")
		if len(insttag) == 2 {
			voltagkey = insttag[0]
			voltagval = insttag[1]
		}
	}

	// Get list of instances that match the conditions specified
	slog.Debugf("Listing instances based on instance_id=\"%s\" and instance_tag_key=\"%s\" and instance_tag_val=\"%s\" ...",
		b.config["instance_id"], insttagkey, insttagval)
	instances, err := ProviderAwsGetEc2Instances(b.client, b.config["instance_id"], insttagkey, insttagval)
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
		if b.config["dryrun"] == "false" {
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

	retention, _ := strconv.ParseInt(b.config["retention"], 10, 64)
	curtime := time.Now().Unix()

	for _, item := range bkpitems {
		snapshotAge := (curtime - item.timestamp) / 86400
		snapDelete := snapshotAge > retention
		slog.Debugf("Considering deletion of snapshot: id=\"%s\" desc=\"%s\" age=%v retention=%v ...",
			item.identifier, item.description, snapshotAge, retention)
		if snapDelete == true {
			if b.config["dryrun"] == "false" {
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
