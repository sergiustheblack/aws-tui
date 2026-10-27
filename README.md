# aws-tui

A k9s-inspired terminal UI for AWS services. Supports:

* ACM
* CloudFront
* CloudWatch
* DynamoDB
* EBS
* EC2
* EKS
* ELB
* ElastiCache
* IAM
* Key Management Service
* Lambda
* RDS
* Route 53
* S3
* SNS
* SQS
* Secrets Manager
* Service Quotas
* VPC

## Keys

* `Ctrl+G` / `Ctrl+P` – switch region / profile; type to narrow the list
* `S` – sort by a column (pick the same column again to reverse)
* `/` – filter rows; `Enter` keeps the filter, `Esc` clears it
* `Ctrl+R` – refresh, `Esc` – back

## Configuration

Credentials, profile and region are resolved like the AWS CLI does: `AWS_PROFILE`, `AWS_REGION`,
`AWS_DEFAULT_REGION`, `AWS_CONFIG_FILE`, `AWS_SHARED_CREDENTIALS_FILE` etc. and `~/.aws/config` are
honored. If no region is configured, `us-east-1` is used.
