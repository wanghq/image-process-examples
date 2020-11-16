package main

import (
	"fmt"
	"github.com/pulumi/pulumi-alicloud/sdk/v2/go/alicloud/fc"
	"github.com/pulumi/pulumi-alicloud/sdk/v2/go/alicloud/log"
	"github.com/pulumi/pulumi-alicloud/sdk/v2/go/alicloud/oss"
	"github.com/pulumi/pulumi-alicloud/sdk/v2/go/alicloud/ram"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi"
	"github.com/pulumi/pulumi/sdk/v2/go/pulumi/config"
)

func main() {
	pulumi.Run(func(ctx *pulumi.Context) error {

		// Create log resources
		logPrj, err := log.NewProject(ctx, "fc-logs", &log.ProjectArgs{
			Description: pulumi.StringPtr("Function logs"),
			Name: pulumi.StringPtr("fc-logs-30c26be"), // Cannot use auto-gen name due to log service behavior.
		})
		if err != nil {
			return err
		}
		store, err := log.NewStore(ctx, "function-logs", &log.StoreArgs{
			Name: pulumi.StringPtr("function-logs"),
			AppendMeta:         nil,
			AutoSplit:          nil,
			EnableWebTracking:  nil,
			MaxSplitShardCount: nil,
			Project:            logPrj.Name,
			RetentionPeriod:    nil,
			ShardCount:         nil,
		})
		if err != nil {
			return err
		}

		_, err = log.NewStoreIndex(ctx, "index", &log.StoreIndexArgs{
			FieldSearches: log.StoreIndexFieldSearchArray{
				log.StoreIndexFieldSearchArgs{
					Alias:           pulumi.String("functionName"),
					CaseSensitive:   pulumi.BoolPtr(false),
					EnableAnalytics: pulumi.BoolPtr(true),
					IncludeChinese:  pulumi.BoolPtr(false),
					JsonKeys:        nil,
					Name:            pulumi.String("functionName"),
					Token:           pulumi.StringPtr(`, '";=()[]{}?@&<>/:\n\t\r`),
					Type:            pulumi.StringPtr("text"),
				},
				log.StoreIndexFieldSearchArgs{
					Alias:           pulumi.String("serviceName"),
					CaseSensitive:   pulumi.BoolPtr(false),
					EnableAnalytics: pulumi.BoolPtr(true),
					IncludeChinese:  pulumi.BoolPtr(false),
					JsonKeys:        nil,
					Name:            pulumi.String("serviceName"),
					Token:           pulumi.StringPtr(`, '";=()[]{}?@&<>/:\n\t\r`),
					Type:            pulumi.StringPtr("text"),
				},
				log.StoreIndexFieldSearchArgs{
					Alias:           pulumi.String("qualifier"),
					CaseSensitive:   pulumi.BoolPtr(false),
					EnableAnalytics: pulumi.BoolPtr(true),
					IncludeChinese:  pulumi.BoolPtr(false),
					JsonKeys:        nil,
					Name:            pulumi.String("qualifier"),
					Token:           pulumi.StringPtr(`, '";=()[]{}?@&<>/:\n\t\r`),
					Type:            pulumi.StringPtr("text"),
				},
				log.StoreIndexFieldSearchArgs{
					Alias:           pulumi.String("versionId"),
					CaseSensitive:   pulumi.BoolPtr(false),
					EnableAnalytics: pulumi.BoolPtr(true),
					IncludeChinese:  pulumi.BoolPtr(false),
					JsonKeys:        nil,
					Name:            pulumi.String("versionId"),
					Type:            pulumi.StringPtr("double"),
				},
			},
			FullText:      log.StoreIndexFullTextArgs{
				CaseSensitive:  pulumi.BoolPtr(false),
				IncludeChinese: pulumi.BoolPtr(false),
				Token:          pulumi.StringPtr(`, '";=()[]{}?@&<>/:\n\t\r`),
			},
			Logstore:      store.Name,
			Project:       logPrj.Name,
		})
		if err != nil {
			return err
		}
		srcBucketName := "image-process-examples-media-assets"
		tgtBucketName := srcBucketName + "-target"
		srcBucket, err := oss.NewBucket(ctx, srcBucketName, &oss.BucketArgs{
			Acl:    pulumi.String("private"),
			Bucket: pulumi.String(srcBucketName),
		})
		if err != nil {
			return err
		}
		tgtBucket, err := oss.NewBucket(ctx, tgtBucketName, &oss.BucketArgs{
			Acl:    pulumi.String("private"),
			Bucket: pulumi.String(tgtBucketName),
		})
		if err != nil {
			return err
		}

		// Create service role
		srvRole, err := ram.NewRole(ctx, "service-role", &ram.RoleArgs{
			Document: pulumi.String(`
				{
					"Statement": [
						{
							"Action": "sts:AssumeRole",
							"Effect": "Allow",
							"Principal": {
								"Service": [
									"fc.aliyuncs.com"
								]
							}
						}
					],
					"Version": "1"
				}`),
			Description: pulumi.String("Allow fc to post logs and access oss"),
			Force:       pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		policyDoc := pulumi.All(logPrj.Name, store.Name, srcBucket.Bucket, tgtBucket.Bucket).ApplyString(
			func(args []interface{}) (string, error) {
				prjName := args[0].(string)
				storeName := args[1].(string)
				srcBucketName := args[2].(*string)
				tgtBucketName := args[3].(*string)
				return fmt.Sprintf(`
				{
					"Version": "1",
					"Statement": [
						{
							"Action": [
								"log:PostLogStoreLogs"
							],
							"Resource": "acs:log:*:*:project/%s/logstore/%s",
							"Effect": "Allow"
						},
						{
							"Action": [
								"oss:GetObject"
							],
							"Resource": "acs:oss:*:*:%s/*",
							"Effect": "Allow"
						},
						{
							"Action": [
								"oss:PutObject"
							],
							"Resource": "acs:oss:*:*:%s/*",
							"Effect": "Allow"
						}
					]
				}`, prjName, storeName, *srcBucketName, *tgtBucketName), nil
			},
		)
		policy, err := ram.NewPolicy(ctx, "service-role-policy", &ram.PolicyArgs{
			Description: pulumi.StringPtr("Post logs and access oss"),
			Document:    policyDoc,
			Force:       nil,
			Name:        nil,
			Statements:  nil,
			Version:     nil,
		})
		if err != nil {
			return err
		}
		_, err = ram.NewRolePolicyAttachment(ctx, "attach-service-role-policy", &ram.RolePolicyAttachmentArgs{
			PolicyName: policy.Name,
			PolicyType: pulumi.String("Custom"),
			RoleName:   srvRole.Name,
		})
		if err != nil {
			return err
		}


		srv, err := fc.NewService(ctx, "image-process-examples", &fc.ServiceArgs{
			Description:    pulumi.StringPtr("A collection of fc examples"),
			InternetAccess: pulumi.BoolPtr(true),
			LogConfig:      fc.ServiceLogConfigArgs{
				Logstore: store.Name,
				Project:  logPrj.Name,
			},
			Name:           pulumi.StringPtr("image-process-examples"),
			NamePrefix:     nil,
			NasConfig:      nil,
			Publish:        nil,
			Role:           srvRole.Arn,
			VpcConfig:      nil,
		})
		if err != nil {
			return err
		}

		f, err := fc.NewFunction(ctx, "compress-thumbnail", &fc.FunctionArgs{
			CaPort:                nil,
			CustomContainerConfig: nil,
			Description:           pulumi.StringPtr("simple hello world"),
			EnvironmentVariables:  pulumi.Map{
				"SOURCE_BUCKET": srcBucket.Bucket,
				"TARGET_BUCKET": tgtBucket.Bucket,
				"PYTHONUSERBASE": pulumi.String("/code/.fun/python"),
			},
			Filename:              pulumi.StringPtr(pulumi.NewFileArchive("./code.zip").Path()),
			Handler:               pulumi.String("index.handler"),
			InitializationTimeout: pulumi.IntPtr(10),
			Initializer:           pulumi.StringPtr("index.initializer"),
			InstanceConcurrency:   nil,
			InstanceType:          nil,
			MemorySize:            pulumi.IntPtr(128),
			Name:                  pulumi.StringPtr("compress-thumbnail"),
			NamePrefix:            nil,
			OssBucket:             nil,
			OssKey:                nil,
			Runtime:               pulumi.String("python3"),
			Service:               srv.Name,
			Timeout:               pulumi.IntPtr(20),
		})
		if err != nil {
			return err
		}

		invocationRole, err := ram.NewRole(ctx, "oss-invoke-fc-role", &ram.RoleArgs{
			Document: pulumi.String(`
				{
					"Statement": [
						{
							"Action": "sts:AssumeRole",
							"Effect": "Allow",
							"Principal": {
								"Service": [
									"oss.aliyuncs.com"
								]
							}
						}
					],
					"Version": "1"
				}`),
			Description: pulumi.String("Allow oss to invoke functions"),
			Force:       pulumi.Bool(true),
		})
		if err != nil {
			return err
		}
		_, err = ram.NewRolePolicyAttachment(ctx, "attach-invocation-role-policy", &ram.RolePolicyAttachmentArgs{
			PolicyName: pulumi.String("AliyunFCInvocationAccess"),
			PolicyType: pulumi.String("System"),
			RoleName:   invocationRole.Name,
		}, pulumi.Aliases([]pulumi.Alias{
			{
				Name: pulumi.String("attach"),
			},
		}))
		if err != nil {
			return err
		}

		region := config.Require(ctx, "alicloud:region")
		account := config.Require(ctx, "alicloud:account")
		srcArn := srcBucket.Bucket.ApplyString(func(name *string) string {
			return fmt.Sprintf("acs:oss:%s:%s:%s", region, account, *name)
		})
		t, err := fc.NewTrigger(ctx, "on-oss-object-created", &fc.TriggerArgs{
			Config: pulumi.StringPtr(`
			{
				"events": [
				  "oss:ObjectCreated:PutObject",
				  "oss:ObjectCreated:PostObject",
				  "oss:ObjectCreated:CompleteMultipartUpload",
				  "oss:ObjectCreated:PutSymlink"
				],
				"filter": {
				  "key": {
					"prefix": "src",
					"suffix": ".png"
				  }
				}
			  }`),
			ConfigMns:  nil,
			Function:   f.Name,
			Name:       pulumi.StringPtr("on-oss-object-created"),
			NamePrefix: nil,
			Role:       invocationRole.Arn,
			Service:    srv.Name,
			SourceArn:  srcArn,
			Type:       pulumi.String("oss"),
		})
		if err != nil {
			return err
		}
		ctx.Export("fc.service", srv.Name)
		ctx.Export("fc.function", f.Name)
		ctx.Export("fc.trigger", t.Name)
		ctx.Export("oss.srcBucket", srcBucket.Bucket)
		ctx.Export("oss.tgtBucket", tgtBucket.Bucket)
		return nil
	})
}
