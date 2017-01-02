# aws-autoscale-zabbix

aws-autoscale-zabbix automatically manages Zabbix monitored hosts using AWS SNS.

Auto(Up-)Scaled hosts can be added to Zabbix OOTB, using
[AutoRegistration](https://www.zabbix.com/documentation/3.2/manual/discovery/auto_registration).
However, when scaling down, Zabbix should be notified to avoid
triggering alerts. This is where aws-autoscale-zabbix (AAZ) comes in.
AAZ can receive AWS SNS messages via HTTP(S) and will remove (delete or disable) hosts
from Zabbix monitoring when scale down occurs.

AAZ is implemented in pure Go; it has no runtime dependencies beyond
its configuration file (see below). To make AAZ work, you just have to ensure
that hosts in Zabbix use the AWS InstanceId as their name in Zabbix.


## Installation
To download and compile the go source, run:

```bash
go get github.com/schnoddelbotz/aws-autoscale-zabbix
```

The above command will automatically download AAZ's non-standard-library
dependencies, namely [go-aws-auth](https://github.com/smartystreets/go-aws-auth)
and [HCL](https://github.com/hashicorp/hcl), too.

<!-- todo...:
An example systemd unit for starting up AAZ on boot is included [here](aaz-systemd.service).
-->
AAZ will not daemonize or log to a file; this is considered systemd's task.

On startup, AAZ will retrieve the current state of the autoscaling group to
bring Zabbix in sync (via AWS API) -- given that required AWS credentials
are provided in the AAZ configuration file. Afterwards, AAZ will listen for SNS notifications.


## Configuration
AAZ requires Zabbix JSON-RPC/API access rights to remove hosts.
Upon scale-down, hosts can be either deleted or disabled only.

AAZ configuration uses HCL for its configuration file.
An example AAZ configuration:

```HCL
ListenerConfig {
  # IP and TCP port to listen for SNS notifications
  Address = ":8080"
  # Hosts allowed to send SNS notifications and request status:
  # Use a regexp; two backslashes in HCL will result in a single regexp backslash
  HostsAllow = "^192\\.168\\.*"
  # Optionally provide TLS certificate to serve using HTTPS
  # TLS_CertPath = "/etc/aaz-tls.cert"
  # TLS_CertKey = "/etc/aaz-tls.key"
}

AutoScale {
  # Provide AutoScale group name to monitor
  GroupName = "my-asg-0"
  # Region of the ASG
  Region = "eu-west-1"
  # Provide credentials for AWS API access for initial AWS<->Zabbix sync:
  AccessKey = "your-access-key-here"
  SecretKey = "your-secret-key-here"
}

ZabbixConfig {
  # Zabbix JSONRPC connection URL
  URL = "http://192.168.100.123/zabbix/api_jsonrpc.php"
  User = "Admin"
  Password = "zabbix"
  # What to do on scale-down: DELETE or DISABLE host
  ScaleDownAction = "DELETE"
  # Restrict hosts to manage through AAZ by Zabbix groupId ...
  RestrictToGroupId = 2
  # ... and/or templateId
  #RestrictToTemplateId = 10001
}
```

Before starting AAZ, you should create a SNS topic and add the AAZ `http(s)://host:port` as subscriber.
AAZ will log SNS subscription requests to make you aware that this has to be done, too...
Finally, enable notifications in your AutoScaling group, pointing to the corresponding SNS topic.


## Status
_BIG FAT WARNING_: aws-autoscale-zabbix is in *EXPERIMENTAL* / *PoC* state.
It should work as advertised by now... but:
Use at your own risk and fun. Feedback highly appreciated.

To monitor status of a running AAZ process, query `/status` via HTTP(S).


## Links

### Activating SNS notifications
- https://aws.amazon.com/blogs/aws/auto-scaling-notifications-recurrence-and-more-control/
- http://docs.aws.amazon.com/autoscaling/latest/userguide/ASGettingNotifications.html
- http://docs.aws.amazon.com/autoscaling/latest/userguide/lifecycle-hooks.html

### Retrieving information about auto-scaled instances
- https://docs.aws.amazon.com/cli/latest/reference/autoscaling/describe-auto-scaling-groups.html
- https://docs.aws.amazon.com/AutoScaling/latest/APIReference/API_DescribeAutoScalingGroups.html


## Alternatives
Instead of listening for AWS SNS notifications, one may alternatively rely on an AWS SQS queue
(see first link above). Another alternative is to put required Zabbix credentials
on every monitored node and let the node delete itself from Zabbix upon instance termination/shutdown using
[this](https://github.com/moshe0076/zabbix/tree/master/remove-host-from-zabbix) Python script
[introduced here](https://devopstrailer.wordpress.com/2015/06/11/zabbix-aws-and-auto-registration/).
The Python script relies on [pyZabbix](https://github.com/lukecyca/pyzabbix) to communicate with Zabbix.


## License
MIT
