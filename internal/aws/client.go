package aws

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	cftypes "github.com/aws/aws-sdk-go-v2/service/cloudformation/types"

	"github.com/awesome-foundation/cfntop/internal/model"
)

// CloudFormationAPI abstracts the CloudFormation API calls for testability.
type CloudFormationAPI interface {
	DescribeStackEvents(ctx context.Context, params *cloudformation.DescribeStackEventsInput, optFns ...func(*cloudformation.Options)) (*cloudformation.DescribeStackEventsOutput, error)
	ListStacks(ctx context.Context, params *cloudformation.ListStacksInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStacksOutput, error)
	ListStackResources(ctx context.Context, params *cloudformation.ListStackResourcesInput, optFns ...func(*cloudformation.Options)) (*cloudformation.ListStackResourcesOutput, error)
}

// Client wraps an AWS CloudFormation API client.
type Client struct {
	api CloudFormationAPI
}

// NewClient creates a Client configured with the given region and profile.
func NewClient(ctx context.Context, region, profile string) (*Client, error) {
	var opts []func(*config.LoadOptions) error
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}

	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return nil, fmt.Errorf("loading AWS config: %w", err)
	}

	return &Client{api: cloudformation.NewFromConfig(cfg)}, nil
}

// NewClientFromAPI creates a Client from an existing API implementation (for testing).
func NewClientFromAPI(api CloudFormationAPI) *Client {
	return &Client{api: api}
}

// activeStatuses are the stack statuses we show by default (excludes DELETE_COMPLETE).
var activeStatuses = []cftypes.StackStatus{
	cftypes.StackStatusCreateInProgress,
	cftypes.StackStatusCreateFailed,
	cftypes.StackStatusCreateComplete,
	cftypes.StackStatusRollbackInProgress,
	cftypes.StackStatusRollbackFailed,
	cftypes.StackStatusRollbackComplete,
	cftypes.StackStatusDeleteInProgress,
	cftypes.StackStatusDeleteFailed,
	cftypes.StackStatusUpdateInProgress,
	cftypes.StackStatusUpdateCompleteCleanupInProgress,
	cftypes.StackStatusUpdateComplete,
	cftypes.StackStatusUpdateFailed,
	cftypes.StackStatusUpdateRollbackInProgress,
	cftypes.StackStatusUpdateRollbackFailed,
	cftypes.StackStatusUpdateRollbackCompleteCleanupInProgress,
	cftypes.StackStatusUpdateRollbackComplete,
	cftypes.StackStatusReviewInProgress,
	cftypes.StackStatusImportInProgress,
	cftypes.StackStatusImportComplete,
	cftypes.StackStatusImportRollbackInProgress,
	cftypes.StackStatusImportRollbackFailed,
	cftypes.StackStatusImportRollbackComplete,
}

// FetchStacks returns a list of all active stacks in the region.
func (c *Client) FetchStacks(ctx context.Context) (*model.StackList, error) {
	var stacks []model.StackSummary
	var nextToken *string

	for {
		out, err := c.api.ListStacks(ctx, &cloudformation.ListStacksInput{
			StackStatusFilter: activeStatuses,
			NextToken:         nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing stacks: %w", err)
		}

		for _, s := range out.StackSummaries {
			createdAt := ""
			if s.CreationTime != nil {
				createdAt = s.CreationTime.Format("2006-01-02T15:04:05Z")
			}
			updatedAt := ""
			if s.LastUpdatedTime != nil {
				updatedAt = s.LastUpdatedTime.Format("2006-01-02T15:04:05Z")
			}
			stacks = append(stacks, model.StackSummary{
				StackName:   aws.ToString(s.StackName),
				StackID:     aws.ToString(s.StackId),
				Status:      string(s.StackStatus),
				CreatedAt:   createdAt,
				UpdatedAt:   updatedAt,
				Description: aws.ToString(s.TemplateDescription),
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return &model.StackList{Stacks: stacks}, nil
}

// FetchResources returns all resources for a stack, handling pagination.
func (c *Client) FetchResources(ctx context.Context, stackName string) ([]model.Resource, error) {
	var resources []model.Resource
	var nextToken *string

	for {
		out, err := c.api.ListStackResources(ctx, &cloudformation.ListStackResourcesInput{
			StackName: aws.String(stackName),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("listing stack resources: %w", err)
		}

		for _, r := range out.StackResourceSummaries {
			lastUpdated := ""
			if r.LastUpdatedTimestamp != nil {
				lastUpdated = r.LastUpdatedTimestamp.Format("2006-01-02T15:04:05Z")
			}
			resources = append(resources, model.Resource{
				LogicalID:   aws.ToString(r.LogicalResourceId),
				PhysicalID:  aws.ToString(r.PhysicalResourceId),
				Type:        aws.ToString(r.ResourceType),
				Status:      string(r.ResourceStatus),
				LastUpdated: lastUpdated,
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	return resources, nil
}

// FetchEvents returns stack events sorted ascending by timestamp (oldest first).
// AWS returns events newest-first, so we reverse after collecting all pages.
// If limit > 0, only the last N events (by time) are returned.
func (c *Client) FetchEvents(ctx context.Context, stackName string, limit int) (*model.StackEvents, error) {
	var events []model.StackEvent
	var nextToken *string

	for {
		out, err := c.api.DescribeStackEvents(ctx, &cloudformation.DescribeStackEventsInput{
			StackName: aws.String(stackName),
			NextToken: nextToken,
		})
		if err != nil {
			return nil, fmt.Errorf("describing stack events: %w", err)
		}

		for _, e := range out.StackEvents {
			ts := ""
			if e.Timestamp != nil {
				ts = e.Timestamp.Format("2006-01-02T15:04:05Z")
			}
			events = append(events, model.StackEvent{
				Timestamp:    ts,
				LogicalID:    aws.ToString(e.LogicalResourceId),
				Status:       string(e.ResourceStatus),
				StatusReason: aws.ToString(e.ResourceStatusReason),
				ResourceType: aws.ToString(e.ResourceType),
				PhysicalID:   aws.ToString(e.PhysicalResourceId),
			})
		}

		if out.NextToken == nil {
			break
		}
		nextToken = out.NextToken
	}

	// AWS returns newest-first; reverse to get oldest-first.
	for i, j := 0, len(events)-1; i < j; i, j = i+1, j-1 {
		events[i], events[j] = events[j], events[i]
	}

	// Apply limit: keep the last N events (already sorted ascending, so tail).
	if limit > 0 && len(events) > limit {
		events = events[len(events)-limit:]
	}

	return &model.StackEvents{
		StackName: stackName,
		Events:    events,
	}, nil
}

// FormatError returns a user-friendly error message for common AWS errors.
func FormatError(err error) string {
	msg := err.Error()

	if strings.Contains(msg, "does not exist") {
		return "Stack not found. Check the stack name/ARN and region."
	}
	if strings.Contains(msg, "ExpiredToken") || strings.Contains(msg, "ExpiredTokenException") {
		return "AWS credentials have expired. Refresh your credentials and try again."
	}
	if strings.Contains(msg, "NoCredentialProviders") || strings.Contains(msg, "no EC2 IMDS") {
		return "No AWS credentials found. Set AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY, use --profile, or configure AWS SSO."
	}
	if strings.Contains(msg, "AccessDenied") || strings.Contains(msg, "is not authorized") {
		return "Access denied. Check your IAM permissions for CloudFormation read access."
	}
	if strings.Contains(msg, "could not find region") || strings.Contains(msg, "MissingRegion") {
		return "No AWS region configured. Use --region, set AWS_REGION, or configure a default region."
	}

	return fmt.Sprintf("AWS error: %s", msg)
}
