package tui

import (
	"context"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecs"
	ecstypes "github.com/aws/aws-sdk-go-v2/service/ecs/types"
)

// ECSDeployment holds info about a single ECS service deployment.
type ECSDeployment struct {
	ID             string
	Status         string
	TaskDefinition string // short form: family:revision
	TaskDefARN     string // full ARN for matching tasks
	Desired        int
	Running        int
	Failed         int
	Pending        int
	RolloutState   string
	CreatedAt      string
	FailedTasks    []ECSFailedTask
}

// ECSFailedTask holds info about a failed/stopped ECS task.
type ECSFailedTask struct {
	TaskID     string
	StopCode   string
	StopReason string
	StoppedAt  string
}

// ECSServiceState holds deployments for an ECS service.
type ECSServiceState struct {
	Deployments []ECSDeployment
}

// ECSAPI abstracts the ECS API calls needed.
type ECSAPI interface {
	DescribeServices(ctx context.Context, params *ecs.DescribeServicesInput, optFns ...func(*ecs.Options)) (*ecs.DescribeServicesOutput, error)
	ListTasks(ctx context.Context, params *ecs.ListTasksInput, optFns ...func(*ecs.Options)) (*ecs.ListTasksOutput, error)
	DescribeTasks(ctx context.Context, params *ecs.DescribeTasksInput, optFns ...func(*ecs.Options)) (*ecs.DescribeTasksOutput, error)
}

// parseServiceARN extracts cluster and service name from an ECS service ARN or physical ID.
// Format: arn:aws:ecs:region:account:service/cluster/service-name
// Or just: cluster/service-name
func parseServiceARN(physicalID string) (cluster, service string, ok bool) {
	// Handle ARN format
	if strings.HasPrefix(physicalID, "arn:") {
		// Find the resource part after the last ":"
		parts := strings.SplitN(physicalID, ":", 6)
		if len(parts) < 6 {
			return "", "", false
		}
		physicalID = parts[5] // "service/cluster/service-name"
	}

	// Strip "service/" prefix if present
	physicalID = strings.TrimPrefix(physicalID, "service/")

	// Split cluster/service-name
	parts := strings.SplitN(physicalID, "/", 2)
	if len(parts) != 2 {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// shortenTaskDef extracts family:revision from a task definition ARN.
func shortenTaskDef(arn string) string {
	// arn:aws:ecs:region:account:task-definition/family:revision -> family:revision
	if idx := strings.LastIndex(arn, "/"); idx >= 0 {
		return arn[idx+1:]
	}
	return arn
}

// shortenTaskID extracts the task ID from a task ARN.
func shortenTaskID(arn string) string {
	if idx := strings.LastIndex(arn, "/"); idx >= 0 {
		return arn[idx+1:]
	}
	return arn
}

// FetchECSServiceState fetches deployment and failed task info for an ECS service.
func FetchECSServiceState(ctx context.Context, ecsClient ECSAPI, physicalID string) (*ECSServiceState, error) {
	cluster, serviceName, ok := parseServiceARN(physicalID)
	if !ok {
		return nil, nil
	}

	out, err := ecsClient.DescribeServices(ctx, &ecs.DescribeServicesInput{
		Cluster:  aws.String(cluster),
		Services: []string{serviceName},
	})
	if err != nil {
		return nil, err
	}
	if len(out.Services) == 0 {
		return nil, nil
	}

	svc := out.Services[0]
	state := &ECSServiceState{}

	// Build deployment index by task def ARN for attributing failed tasks
	deployIdx := make(map[string]int) // taskDefARN -> index in state.Deployments
	for _, d := range svc.Deployments {
		createdAt := ""
		if d.CreatedAt != nil {
			createdAt = d.CreatedAt.Format("2006-01-02T15:04:05Z")
		}
		taskDefARN := aws.ToString(d.TaskDefinition)
		idx := len(state.Deployments)
		deployIdx[taskDefARN] = idx
		state.Deployments = append(state.Deployments, ECSDeployment{
			ID:             aws.ToString(d.Id),
			Status:         aws.ToString(d.Status),
			TaskDefinition: shortenTaskDef(taskDefARN),
			TaskDefARN:     taskDefARN,
			Desired:        int(d.DesiredCount),
			Running:        int(d.RunningCount),
			Failed:         int(d.FailedTasks),
			Pending:        int(d.PendingCount),
			RolloutState:   string(d.RolloutState),
			CreatedAt:      createdAt,
		})
	}

	// Check for failed tasks if any deployment has failures
	hasFailed := false
	for _, d := range state.Deployments {
		if d.Failed > 0 {
			hasFailed = true
			break
		}
	}

	if hasFailed {
		// List recently stopped tasks
		listOut, err := ecsClient.ListTasks(ctx, &ecs.ListTasksInput{
			Cluster:       aws.String(cluster),
			ServiceName:   aws.String(serviceName),
			DesiredStatus: ecstypes.DesiredStatusStopped,
		})
		if err == nil && len(listOut.TaskArns) > 0 {
			arns := listOut.TaskArns
			if len(arns) > 10 {
				arns = arns[:10]
			}
			descOut, err := ecsClient.DescribeTasks(ctx, &ecs.DescribeTasksInput{
				Cluster: aws.String(cluster),
				Tasks:   arns,
			})
			if err == nil {
				for _, t := range descOut.Tasks {
					stoppedAt := ""
					if t.StoppedAt != nil {
						stoppedAt = t.StoppedAt.Format("2006-01-02T15:04:05Z")
					}
					ft := ECSFailedTask{
						TaskID:     shortenTaskID(aws.ToString(t.TaskArn)),
						StopCode:   string(t.StopCode),
						StopReason: aws.ToString(t.StoppedReason),
						StoppedAt:  stoppedAt,
					}
					// Attribute to the deployment with matching task definition
					taskDefARN := aws.ToString(t.TaskDefinitionArn)
					if idx, ok := deployIdx[taskDefARN]; ok {
						state.Deployments[idx].FailedTasks = append(state.Deployments[idx].FailedTasks, ft)
					}
				}
			}
		}
	}

	return state, nil
}
