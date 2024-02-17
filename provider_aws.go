/******************************************************************************\
* Copyright (C) 2024-2024 The Molibackup Authors. All rights reserved.         *
* Licensed under the Apache version 2.0 License                                *
* Homepage: https://github.com/fdupoux/molibackup                              *
\******************************************************************************/

package main

import (
	"context"
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/slices"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/feature/ec2/imds"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

type ProviderAwsEc2Instance struct {
	instanceId    string
	instanceName  string
	instanceOwner string
}

type ProviderAwsEbsVolume struct {
	volumeId   string
	volumeName string
}

type ProviderAwsEbsSnapshot struct {
	volumeId     string
	snapshotId   string
	snapshotDesc string
	snapshotTime int64
}

func ProviderAwsLoadConfig(region string, accesskey_id string, accesskey_secret string) (aws.Config, error) {

	var cfg aws.Config
	var err error

	os.Setenv("AWS_REGION", region)

	// Load the configuration using an access key pair if it has been provided in the configuration
	if accesskey_id != "" && accesskey_secret != "" {
		staticProvider := credentials.NewStaticCredentialsProvider(accesskey_id, accesskey_secret, "")
		cfg, err = config.LoadDefaultConfig(context.TODO(), config.WithCredentialsProvider(staticProvider))
		if err != nil {
			return cfg, fmt.Errorf("failed to load the aws configuration with explicit access key pair: %v", err)
		}
	} else {
		cfg, err = config.LoadDefaultConfig(context.TODO())
		if err != nil {
			return cfg, fmt.Errorf("failed to load the aws configuration without an explicit access key pair: %v", err)
		}
	}

	return cfg, nil
}

func ProviderAwsNewEc2Client(cfg aws.Config) *ec2.Client {

	return ec2.NewFromConfig(cfg)

}

// Get the InstanceId of the EC2 instance currently running this program
func ProviderAwsGetCurrentInstance(cfg aws.Config) (string, error) {

	var instanceId string

	clientImds := imds.NewFromConfig(cfg)
	res1, err := clientImds.GetMetadata(context.TODO(), &imds.GetMetadataInput{
		Path: "instance-id",
	})
	if err != nil {
		return instanceId, fmt.Errorf("unable to determine the EC2 instance ID: %v", err)
	}

	defer res1.Content.Close()
	res2, err := io.ReadAll(res1.Content)
	if err != nil {
		return instanceId, fmt.Errorf("unable to retrieve the EC2 instance ID: %v", err)
	}
	instanceId = string(res2)

	return instanceId, nil
}

// Return basic information about all instances that match conditions specified in the arguments
func ProviderAwsGetEc2Instances(client *ec2.Client, instanceId string, instanceTags map[string]string) ([]ProviderAwsEc2Instance, error) {

	var results []ProviderAwsEc2Instance
	var params *ec2.DescribeInstancesInput
	var filters []types.Filter
	var filtcnt int
	var count int

	if instanceId != "" {
		curfilter := types.Filter{
			Name:   aws.String("instance-id"),
			Values: []string{instanceId},
		}
		filters = append(filters, curfilter)
		filtcnt++
	}

	for tagkey := range instanceTags {
		curfilter := types.Filter{
			Name:   aws.String("tag-key"),
			Values: []string{tagkey},
		}
		filters = append(filters, curfilter)
		filtcnt++
	}

	params = &ec2.DescribeInstancesInput{Filters: filters}
	res, err := client.DescribeInstances(context.TODO(), params)
	if err != nil {
		return nil, fmt.Errorf("DescribeInstances() has failed: %v", err)
	}

	for _, reservation := range res.Reservations {
		for _, instance := range reservation.Instances {
			// Collect all tags in a map
			tagsdict := make(map[string]string)
			for _, curtag := range instance.Tags {
				tagsdict[*curtag.Key] = *curtag.Value
			}
			// Check if all tags specified in instance_tags match
			tagsmatch := true
			for tagkey, tagval := range instanceTags {
				val, ok := tagsdict[tagkey]
				if (ok == false) || (val != tagval) {
					tagsmatch = false
				}
			}
			// Add instance to the results if all the tags required match
			if tagsmatch == true {
				instdata := ProviderAwsEc2Instance{}
				instdata.instanceId = string(*instance.InstanceId)
				instdata.instanceName = string(tagsdict["Name"])
				instdata.instanceOwner = string(*reservation.OwnerId)
				results = append(results, instdata)
				count++
			}
		}
	}

	return results, nil
}

// Return basic information about all volumes that match conditions specified in the arguments
func ProviderAwsGetEbsVolumes(client *ec2.Client, instanceId string, volumeTags map[string]string) ([]ProviderAwsEbsVolume, error) {

	var results []ProviderAwsEbsVolume

	params := &ec2.DescribeVolumesInput{
		Filters: []types.Filter{
			{
				Name:   aws.String("attachment.instance-id"),
				Values: []string{instanceId},
			},
			{
				Name:   aws.String("attachment.status"),
				Values: []string{"attached"},
			},
		},
	}

	resvols, err := client.DescribeVolumes(context.TODO(), params)
	if err != nil {
		return nil, fmt.Errorf("DescribeVolumes() has failed: %v", err)
	}

	for _, volume := range resvols.Volumes {
		// Collect all tags in a map
		tagsdict := make(map[string]string)
		for _, curtag := range volume.Tags {
			tagsdict[*curtag.Key] = *curtag.Value
		}
		// Check if all tags specified in volume_tags match
		tagsmatch := true
		for tagkey, tagval := range volumeTags {
			val, ok := tagsdict[tagkey]
			if (ok == false) || (val != tagval) {
				tagsmatch = false
			}
		}
		// Add volume to the results if all the tags required match
		if tagsmatch == true {
			voldata := ProviderAwsEbsVolume{}
			voldata.volumeId = string(*volume.VolumeId)
			voldata.volumeName = string(tagsdict["Name"])
			results = append(results, voldata)
		}
	}

	return results, nil
}

// Get basic information about snapshots which are related to a particular volume
func ProviderAwsGetEbsSnapshots(client *ec2.Client, volumeId string) ([]ProviderAwsEbsSnapshot, error) {

	var results []ProviderAwsEbsSnapshot

	params := &ec2.DescribeSnapshotsInput{
		Filters: []types.Filter{
			{
				Name: aws.String("tag:CreatedBy"),
				Values: []string{
					"molibackup",
				},
			},
			{
				Name: aws.String("volume-id"),
				Values: []string{
					volumeId,
				},
			},
		},
	}

	ressnaps, err := client.DescribeSnapshots(context.TODO(), params)
	if err != nil {
		return nil, fmt.Errorf("DescribeSnapshots() has failed: %v", err)
	}

	for _, snapshot := range ressnaps.Snapshots {
		snapdata := ProviderAwsEbsSnapshot{}
		snapdata.volumeId = string(*snapshot.VolumeId)
		snapdata.snapshotId = string(*snapshot.SnapshotId)
		snapdata.snapshotDesc = string(*snapshot.Description)
		snapdata.snapshotTime = (*snapshot.StartTime).Unix()
		results = append(results, snapdata)
	}

	return results, nil
}

func ProviderAwsCreateEbsSnapshot(client *ec2.Client, volumeId string, snapname string, snapdate string, snaptime string, lockmode string, lockduration int32) (string, error) {

	params1 := &ec2.CreateSnapshotInput{
		VolumeId:    &volumeId,
		Description: &snapname,
		TagSpecifications: []types.TagSpecification{
			{
				ResourceType: types.ResourceTypeSnapshot,
				Tags: []types.Tag{
					{
						Key:   aws.String("Name"),
						Value: aws.String(snapname),
					},
					{
						Key:   aws.String("CreatedBy"),
						Value: aws.String("molibackup"),
					},
					{
						Key:   aws.String("CreateDate"),
						Value: aws.String(snapdate),
					},
					{
						Key:   aws.String("Timestamp"),
						Value: aws.String(snaptime),
					},
				},
			},
		},
	}

	// Create snapshot of the volume
	result, err := client.CreateSnapshot(context.TODO(), params1)
	if err != nil {
		return "", fmt.Errorf("CreateSnapshot() has failed for volume %s: %v", volumeId, err)
	}
	snapid := *result.SnapshotId

	// Lock the new snapshot is this has been requested
	curmode := types.LockMode(lockmode)
	lockmodes := curmode.Values()
	if slices.Contains(lockmodes, curmode) {
		params2 := &ec2.LockSnapshotInput{
			SnapshotId:   &snapid,
			LockMode:     curmode,
			LockDuration: &lockduration,
		}

		if _, err := client.LockSnapshot(context.TODO(), params2); err != nil {
			return snapid, fmt.Errorf("LockSnapshot() has failed for snapshot %s: %v", snapid, err)
		}
	}

	return snapid, nil
}

func ProviderAwsDeleteEbsSnapshot(client *ec2.Client, snapshotId string) error {

	params := &ec2.DeleteSnapshotInput{
		SnapshotId: &snapshotId,
	}
	_, err := client.DeleteSnapshot(context.TODO(), params)
	if err != nil {
		return fmt.Errorf("DeleteSnapshot() has failed for snapshot %s: %v", snapshotId, err)
	}

	return nil
}
