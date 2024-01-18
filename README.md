# molibackup

## Overview
This is "molibackup" also known as "modular libre backup". It is a golang based
backup utility, hence it should be able to run on all major paltforms and architectures.

In the current version, this program provides features for creating and managing
snapshots of EBS volumes in AWS (Amazon Web Services). It is able to find all
volumes attached to either one specific instance or all instances which have a
particular tag, create a snapshot of each volume, and then delete snapshots of
these volumes after a retention period.

## Installation
This program is stateless, it does not require any database or state file to track
the state of its backups. You just need the static binary, a yaml configuration
file, and access to the required service APIs.

Installing this program involves the following steps:
* Downloading and copying the static binary to your system
* Creating a yaml configuration file on your system
* Creating a cron job (or similar) to run it on a regular basis
* Providing access to the service APIs with sufficient credentials

### Downloading and copying the binary
As this is a golang program, the binaries are static with no runtime dependencies.
Hence the application can be installed by just copying the binary to a location such
as `/usr/local/sbin`.

### Creating a yaml configuration file
This application is using a yaml configuration file. By default it will try to find
the configuration file in the following locations:

  * /etc/molibackup/molibackup.yaml
  * ${HOME}/molibackup/molibackup.yaml
  * directory where the binary is located

You can also specify the path to the configuration file on the command line:
```
$ /usr/local/sbin/molibackup -c /etc/molibackup/molibackup.yaml
```

The configuration file is made of a `global` section as well as a `jobs` section
which defines one or multiple backup jobs. Here is an example:
```
# cat /etc/molibackup/molibackup.yaml
---
global:
  loglevel: info

jobs:
    myjob01:
      module: ebs-snapshot
      enabled: true
      dryrun: false
      retention: 90
      aws_region: "us-west-2"
      accesskey_id: "MyAccessKeyId"
      accesskey_secret: "MyAccessKeySecret"
      instance_id: "i-01233456789abcdef"
    myjob02:
      module: ebs-snapshot
      enabled: true
      dryrun: false
      retention: 90
      aws_region: "us-west-2"
      accesskey_id: "MyAccessKeyId"
      accesskey_secret: "MyAccessKeySecret"
      instance_id: "local"
```

Each job comes with several options which are either mandatory or optional. The
`module` option is mandatory and it tells the program what type of backup to
create. At this stage, this program comes with only one module: `ebs-snapshot`.
The `enabled` and `dryrun` ones are optional, and their respective default values
are `true` and `false`. They allow you to disable a backup job, and to do a dry run
to see what the program would do without actually do anything. The other options in
the job confguration are specific to each type of backup job, and these are documented
in the sections corresponding to each type of backup.

### Scheduling the execution
You need to create a cron job (or use any alternative scheduler) to execute the program
automatically. Here is an example of a cronjob which runs the program daily at 4am:
```
# cat /etc/cron.d/molibackup
0 4 * * * root  /usr/local/sbin/molibackup -c /etc/molibackup/molibackup.yaml >> /var/log/molibackup.log 2>&1
```

The example above configures the cron job to run as the `root` user as this should
exist on all linux systems. But the program does not need `root` privileges to run.
Hence it is recommended to run it as an ordinary user. Just make sure this ordinary
user has sufficient permissions to read the configuration file and write to the log
file.

### Providing the required credentials
If the backup module you are using requires access to some service APIs, make sure these
conditions are satisfied. Please refer to the module specific documentation below for
more details.

## Exit status
This program returns the following exit status depending on the success or failure:

  * 0 => The program has executed all backup jobs without any error
  * 1 => Failed to read the configuration file (file not found or invalid configuration)
  * 2 => The program has attempted to run the backup jobs but there were some errors

## Building from sources
Static binaries for this program are provided on the official project page for the most
popular platforms and architectures so you do not have to build it yourself. But you
can compile this program from sources if there is no official binary for your platform
or architecture, or if you want to build it for any other reason.

You need golang version 1.19 or more recent, as well as `make` and `sed` in order to
build this program from sources. These dependencies must either be present on your build
system, or you can use a docker image which comes with all these dependencies.

You can build a binary for your local platform by running `make build` if you have all
the build dependencies installed on your system. Or you can use `make docker-build` if
you want to build the program using the recommended docker image.

It is possible to create reproducible builds. It means the compiled binaries should always
be strictly identical if you compile this program from the same sources multiple times.
This allows you to check that the binary you are using was built from the official sources
without any malicious modification. For this to work you have to use the same version of
golang with the same sources and with the correct compilation options. To rebuild an
official release in such a way, you have to get the correct sources by using either the
sources from an official release source file, or by getting the sources from the tag
corresponding to the release in the git repository. You then have to follow the compilation
instructions included in the sources for the version you want to rebuild.

Here is how to create reproducible builds for the current version. You should run
`make docker-release` to rebuild the binaries with the same build command and the same
docker image that was used to produce the official release. You can then use the
`sha256sum` command to make sure the binaries you produced match the checksums of the
official binary files.

## Creating and rotating snapshots of EBS volumes

### Overview
This program comes with a module named `ebs-snapshot` which is able to create and rotate
snapshots of EBS volumes in AWS. It finds one or multiple EC2 instances based on the
criteria specified in the configuration, then it finds all EBS volumes attached to these
EC2 instances and it creates a snapshots of some or all of these EBS volumes depending on
the configuration. The snapshots are named after the name of the corresponding volumes
with the date of the backup at the end. Finally it deletes snapshots which are older than
the retention period.

### Configuration
Here is an example of a configuration file for running multiple jobs that create EBS snapshots:
```
# cat /etc/molibackup/molibackup.yaml
---
global:
  loglevel: info

jobs:
    myjob01:
      module: ebs-snapshot
      enabled: true
      dryrun: false
      retention: 180
      aws_region: "us-west-2"
      accesskey_id: "MyAccessKeyId"
      accesskey_secret: "MyAccessKeySecret"
      instance_id: "i-01233456789abcdef"
    myjob02:
      module: ebs-snapshot
      enabled: true
      dryrun: false
      retention: 60
      aws_region: "us-west-2"
      accesskey_id: "MyAccessKeyId"
      accesskey_secret: "MyAccessKeySecret"
      instance_id: "local"
    myjob03:
      module: ebs-snapshot
      enabled: true
      dryrun: false
      retention: 90
      aws_region: "us-west-2"
      accesskey_id: "MyAccessKeyId"
      accesskey_secret: "MyAccessKeySecret"
      instance_tag: "Molibackup_enabled=true"
      volume_tag: "Molibackup_enabled=true"
```

The `aws_region` attribute is mandatory. The AWS Access Key pair details are required
unless you run the program on an EC2 instance which is attached to an IAM role which
has sufficient privileges to perform all the actions.

The `instance_id`, `instance_tag` and `volume_tag` attributes are optional. They are used
to restrict the scope of the job. For example you can use `instance_tag: "Molibackup_enabled=true"`
so the program resctrits the backup to instances which have a tag named `Molibackup_enabled`
and a value set to `true`. The tags are case sensitive so please make sure the values in
the configuration matches the case of the actual tags you have created on your instances
and volumes.

The `retention` option speficies the retention period expressed in days. For example if
you set `retention: 90` it will delete snapshots which were created more than 90 days ago.
If you do not specify the `retention` attribute, it will use 30 days as the default value.

### Strategies
You can either install this program to run on each EC2 instances that needs to be backed up,
or you can install it on an server that will be responsible for creating the backups for
multiple EC2 instances in a particular region of your AWS account.

If you want this program to create snapshots of the volumes of a single EC2 instance, you
should use the `instance_id` attribute in the job configuration to target this single
instance. You should either set the `instance_id` attribute to a specific instance ID
or you can use the special keyword `local` so the program uses the instance meta-data to
automatically detects the ID of the instance where it is running.

Alternatively you can configure this program to create backups of multiple EC2 instances.
In that case you should use the `instance_tag` attribute instead of `instance_id`. You will
have to create a tag on all EC2 instance that needs to be backed up so the program can
determine which instances must be included in the scope of the backup job. By default the
program will create snapshots of all EBS volumes which are attached to these EC2 instances.
If you want to control which EBS volumes must be included in the backup job, you should
create a tag on all EBS volumes that you want to be backed up and use the `volume_tag`
attribute in the job configuration to tell the program which volumes must be included.
Please refer to the configuration section above for specific examples.

### How it works
The program creates and rotates snapshots of EBS volumes which meet the conditions
specified in the jobs configurations.

First it finds all EC2 instances that meet the conditions configured using `instance_id`
and/or `instance_tag`. Then it finds all EBS volumes which are attached to these EC2
instances and which meet the volume tags specified with `volume_tag` if this option is
present.

After it has found a list of all volumes that are included in the scope of the backup job,
it creates one new snapshot of each volume. Then it finds all snapshots that have already
been created for these volumes, and it deletes snapshots which are older than the retention
period.

### Credentials
The `ebs-snapshot` module uses the AWS APIs to create an manage snapshots of EBS Volumes.
Hence it requires an IAM Role with sufficient AWS credentials to perform these actions.
These credentials can be provided either explicitly through an AWS Access Key pair, or
implicitly by running the program on an EC2 instance which has the IAM Role attached
to it via an instance profile. The IAM Role you are using requires the following permissions
so the program is able to run successfully:
```
ec2:CreateSnapshot
ec2:CreateTags
ec2:DeleteSnapshot
ec2:DescribeInstances
ec2:DescribeSnapshotAttribute
ec2:DescribeSnapshots
ec2:DescribeSnapshotsAttribute
ec2:DescribeVolumeAttribute
ec2:DescribeVolumeStatus
ec2:DescribeVolumes
ec2:DescribeTags
ec2:ResetSnapshotAttribute
```
