package tui

import (
	"strings"
	"time"

	"github.com/dustin/go-humanize"
)

// serviceAbbreviations maps verbose AWS service names to short tags.
var serviceAbbreviations = map[string]string{
	"ElasticLoadBalancingV2": "ELBv2",
	"ElasticLoadBalancing":   "ELB",
	"CloudFormation":         "CFN",
	"CloudFront":             "CF",
	"CloudWatch":             "CW",
	"CloudTrail":             "CT",
	"CertificateManager":     "ACM",
	"SecretsManager":         "SM",
	"ApiGateway":             "APIGW",
	"ApiGatewayV2":           "APIGWv2",
	"AutoScaling":            "ASG",
	"ElastiCache":            "ElastiCache",
	"DynamoDB":               "DDB",
	"KinesisFirehose":        "Firehose",
	"StepFunctions":          "SFN",
	"EventBridge":            "EB",
	"CodeBuild":              "CB",
	"CodePipeline":           "CP",
	"CodeDeploy":             "CD",
	"ApplicationAutoScaling": "AppASG",
	"ServiceDiscovery":       "SD",
	"ElasticBeanstalk":       "EBeanstalk",
	"ResourceGroups":         "RG",
	"GuardDuty":              "GD",
	"SecurityHub":            "SecHub",
	"OpenSearchService":      "OpenSearch",
	"Elasticsearch":          "ES",
	"Transfer":               "SFTP",
	"GlobalAccelerator":      "GA",
	"DirectoryService":       "DS",
	"AppStream":              "AS2",
	"Cognito":                "Cognito",
	"Route53":                "R53",
	"WAFv2":                  "WAFv2",
	"SSM":                    "SSM",
	"KMS":                    "KMS",
}

// ShortenType converts AWS::ServiceName::ResourceType to ServiceTag::ResourceType.
func ShortenType(t string) string {
	// Custom::Something -> Custom Something
	if strings.HasPrefix(t, "Custom::") {
		return "Custom " + t[8:]
	}
	if !strings.HasPrefix(t, "AWS::") {
		return strings.ReplaceAll(t, "::", " ")
	}
	rest := t[5:] // strip "AWS::"
	parts := strings.SplitN(rest, "::", 2)
	if len(parts) != 2 {
		return rest
	}
	service := parts[0]
	resource := parts[1]
	if abbr, ok := serviceAbbreviations[service]; ok {
		service = abbr
	}
	return service + " " + resource
}

// HumanizeTime converts an ISO timestamp to a relative time string like "2m ago".
func HumanizeTime(ts string) string {
	t, err := time.Parse("2006-01-02T15:04:05Z", ts)
	if err != nil {
		t, err = time.Parse(time.RFC3339, ts)
		if err != nil {
			return ts
		}
	}
	return humanize.Time(t)
}
