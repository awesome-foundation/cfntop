package tui

import (
	"time"

	"github.com/awesome-foundation/cfntop/internal/model"
)

func ts(now time.Time, ago time.Duration) string {
	return now.Add(-ago).UTC().Format("2006-01-02T15:04:05Z")
}

// DemoStacks returns a static set of stacks that showcase all UI states.
func DemoStacks() []StackState {
	now := time.Now().UTC()

	return []StackState{
		// 1. Active deploy — ECS service rolling out, one task failing
		{
			Summary: model.StackSummary{
				StackName: "prod-api",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-api/aaa",
				Status:    "UPDATE_IN_PROGRESS",
				UpdatedAt: ts(now, 45*time.Second),
			},
			Active:   true,
			Expanded: true,
			Resources: []ResourceState{
				{
					Resource: model.Resource{
						LogicalID:   "Service",
						PhysicalID:  "arn:aws:ecs:eu-west-1:123456789:service/main/prod-api",
						Type:        "AWS::ECS::Service",
						Status:      "UPDATE_IN_PROGRESS",
						LastUpdated: ts(now, 30*time.Second),
					},
					Touched: true,
					ECS: &ECSServiceState{
						Deployments: []ECSDeployment{
							{
								ID:             "ecs-svc/111",
								Status:         "PRIMARY",
								TaskDefinition: "prod-api:42",
								Desired:        3,
								Running:        1,
								Pending:        1,
								Failed:         1,
								RolloutState:   "IN_PROGRESS",
								CreatedAt:      ts(now, 35*time.Second),
								FailedTasks: []ECSFailedTask{
									{
										TaskID:     "abc123def456",
										StopCode:   "EssentialContainerExited",
										StopReason: "CannotPullContainerError: ref not found",
										StoppedAt:  ts(now, 15*time.Second),
									},
								},
							},
							{
								ID:             "ecs-svc/222",
								Status:         "ACTIVE",
								TaskDefinition: "prod-api:41",
								Desired:        3,
								Running:        3,
								RolloutState:   "COMPLETED",
								CreatedAt:      ts(now, 2*time.Hour),
							},
						},
					},
				},
				{
					Resource: model.Resource{
						LogicalID:   "TaskDefinition",
						Type:        "AWS::ECS::TaskDefinition",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 40*time.Second),
					},
					Touched: true,
				},
				{
					Resource: model.Resource{
						LogicalID:   "TaskRole",
						Type:        "AWS::IAM::Role",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 24*time.Hour),
					},
					Touched: false, // untouched in this deploy
				},
				{
					Resource: model.Resource{
						LogicalID:   "TargetGroup",
						Type:        "AWS::ElasticLoadBalancingV2::TargetGroup",
						Status:      "CREATE_COMPLETE",
						LastUpdated: ts(now, 14*24*time.Hour),
					},
					Touched: false,
				},
				{
					Resource: model.Resource{
						LogicalID:   "ListenerRule",
						Type:        "AWS::ElasticLoadBalancingV2::ListenerRule",
						Status:      "CREATE_COMPLETE",
						LastUpdated: ts(now, 14*24*time.Hour),
					},
					Touched: false,
				},
				{
					Resource: model.Resource{
						LogicalID:   "LogGroup",
						Type:        "AWS::Logs::LogGroup",
						Status:      "CREATE_COMPLETE",
						LastUpdated: ts(now, 14*24*time.Hour),
					},
					Touched: false,
				},
			},
		},

		// 2. Active deploy — simple, no ECS, resource creation failed
		{
			Summary: model.StackSummary{
				StackName: "prod-worker",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-worker/bbb",
				Status:    "UPDATE_ROLLBACK_IN_PROGRESS",
				UpdatedAt: ts(now, 2*time.Minute),
			},
			Active:   true,
			Expanded: true,
			Resources: []ResourceState{
				{
					Resource: model.Resource{
						LogicalID:   "Queue",
						Type:        "AWS::SQS::Queue",
						Status:      "CREATE_FAILED",
						LastUpdated: ts(now, 2*time.Minute),
					},
					Touched:  true,
					HasError: true,
					Events: []model.StackEvent{
						{
							Timestamp:    ts(now, 2*time.Minute),
							LogicalID:    "Queue",
							Status:       "CREATE_FAILED",
							StatusReason: "Resource handler returned message: \"Queue name already exists in this region\"",
						},
						{
							Timestamp: ts(now, 3*time.Minute),
							LogicalID: "Queue",
							Status:    "CREATE_IN_PROGRESS",
						},
					},
				},
				{
					Resource: model.Resource{
						LogicalID:   "Lambda",
						Type:        "AWS::Lambda::Function",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 3*time.Minute),
					},
					Touched: true,
				},
				{
					Resource: model.Resource{
						LogicalID:   "ExecutionRole",
						Type:        "AWS::IAM::Role",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 5*time.Hour),
					},
					Touched: false,
				},
			},
		},

		// 3. Recently completed deploy — all green
		{
			Summary: model.StackSummary{
				StackName: "prod-web",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-web/ccc",
				Status:    "UPDATE_COMPLETE",
				UpdatedAt: ts(now, 12*time.Minute),
			},
			Active:   false,
			Expanded: true,
			Resources: []ResourceState{
				{
					Resource: model.Resource{
						LogicalID:   "Service",
						PhysicalID:  "arn:aws:ecs:eu-west-1:123456789:service/main/prod-web",
						Type:        "AWS::ECS::Service",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 12*time.Minute),
					},
					Touched: true,
					ECS: &ECSServiceState{
						Deployments: []ECSDeployment{
							{
								ID:             "ecs-svc/333",
								Status:         "PRIMARY",
								TaskDefinition: "prod-web:18",
								Desired:        2,
								Running:        2,
								RolloutState:   "COMPLETED",
								CreatedAt:      ts(now, 15*time.Minute),
							},
						},
					},
				},
				{
					Resource: model.Resource{
						LogicalID:   "TaskDefinition",
						Type:        "AWS::ECS::TaskDefinition",
						Status:      "UPDATE_COMPLETE",
						LastUpdated: ts(now, 14*time.Minute),
					},
					Touched: true,
				},
				// Recently deleted resource — sorted by timestamp, not pushed to bottom
				{
					Resource: model.Resource{
						LogicalID:   "OldSidecar",
						Type:        "AWS::ECS::TaskDefinition",
						Status:      "DELETE_COMPLETE",
						LastUpdated: ts(now, 13*time.Minute),
					},
					Deleted: true,
					Touched: true,
				},
				{
					Resource: model.Resource{
						LogicalID:   "TargetGroup",
						Type:        "AWS::ElasticLoadBalancingV2::TargetGroup",
						Status:      "CREATE_COMPLETE",
						LastUpdated: ts(now, 30*24*time.Hour),
					},
					Touched: false,
				},
			},
		},

		// 4. Idle stack — collapsed
		{
			Summary: model.StackSummary{
				StackName: "prod-vpc",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-vpc/ddd",
				Status:    "UPDATE_COMPLETE",
				UpdatedAt: ts(now, 3*24*time.Hour),
			},
		},

		// 5. Idle stack — collapsed
		{
			Summary: model.StackSummary{
				StackName: "prod-bastion",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-bastion/eee",
				Status:    "CREATE_COMPLETE",
				CreatedAt: ts(now, 45*24*time.Hour),
			},
		},

		// 6. Idle stack — collapsed
		{
			Summary: model.StackSummary{
				StackName: "prod-dns",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/prod-dns/fff",
				Status:    "UPDATE_COMPLETE",
				UpdatedAt: ts(now, 7*24*time.Hour),
			},
		},

		// 7. Rollback complete — collapsed
		{
			Summary: model.StackSummary{
				StackName: "staging-api",
				StackID:   "arn:aws:cloudformation:eu-west-1:123456789:stack/staging-api/ggg",
				Status:    "UPDATE_ROLLBACK_COMPLETE",
				UpdatedAt: ts(now, 1*time.Hour),
			},
		},
	}
}
